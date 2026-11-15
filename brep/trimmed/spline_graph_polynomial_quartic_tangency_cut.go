package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// PolynomialQuarticTangencyCut retains separately embedded positive
// layers and their one original order-four point contact.
type PolynomialQuarticTangencyCut struct {
	sources             [3]*geom.BSplineSurface
	material            polynomialNodeCutMaterial
	fourthDerivativeGap *big.Rat
	axis                int
	tolerance           float64
}

// Components returns the native solids in original arrangement region order.
func (r *PolynomialQuarticTangencyCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return append([]Solid(nil), r.material.components...)
}

// Regions returns the indices of the retained original arrangement regions.
func (r *PolynomialQuarticTangencyCut) Regions() []int {
	if r == nil {
		return nil
	}
	return append([]int(nil), r.material.regions...)
}

// Contact returns the exact order-four point and component-local vertices.
// Rational fields are caller-owned copies.
func (r *PolynomialQuarticTangencyCut) Contact() PolynomialMixedCutContact {
	if r == nil || r.sources[0] == nil {
		return PolynomialMixedCutContact{}
	}
	return PolynomialMixedCutContact{
		TwoPolynomialNodeComponentContact: r.material.contacts[0].copy(),
		Order:                             4, DerivativeGap: new(big.Rat).Set(r.fourthDerivativeGap),
	}
}

// CutSplineGraphPolynomialQuarticTangencyComponents cuts A by B over
// their positive shared-base identity XY charts. It retains either two
// exterior regions or two between-branch regions. The positive shells
// share only the exact tangent point. Each binds both original caps.
func CutSplineGraphPolynomialQuarticTangencyComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, tolerance float64,
	o PolynomialNodeCutOptions) (*PolynomialQuarticTangencyCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		!finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	arrangement, e := geom.DiscoverSplineGraphPolynomialQuarticTangency(ctx, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	if !arrangement.Proven || arrangement.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || arrangement.Graph.ProjectionOrientation != 1 {
		return nil, geom.ErrConvergence
	}
	uv, xyz, e := arrangement.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	material, e := assemblePolynomialCutSourceComponents(ctx, a, b, tolerance, o.Proof, arrangement.Regions[:], arrangement.Edges[:],
		[]geom.SplineGraphPolynomialJunction{arrangement.Node}, arrangement.Vertices[:], uv[:], xyz[:])
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return &PolynomialQuarticTangencyCut{
		sources: [3]*geom.BSplineSurface{base, a, b}, material: material, axis: axis, tolerance: tolerance,
		fourthDerivativeGap: arrangement.FourthDerivativeGap,
	}, nil
}
