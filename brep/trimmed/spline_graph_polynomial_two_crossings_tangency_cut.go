//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// PolynomialTwoCrossingsTangencyCut retains separately embedded positive
// layers and their original transverse and tangent point contacts.
type PolynomialTwoCrossingsTangencyCut struct {
	sources     [3]*geom.BSplineSurface
	material    polynomialNodeCutMaterial
	orders      [3]int
	derivatives [3]*big.Rat
	axis        int
	tolerance   float64
}

// Components returns the native solids in original arrangement region order.
func (r *PolynomialTwoCrossingsTangencyCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return append([]Solid(nil), r.material.components...)
}

// Regions returns the indices of the retained original arrangement regions.
func (r *PolynomialTwoCrossingsTangencyCut) Regions() []int {
	if r == nil {
		return nil
	}
	return append([]int(nil), r.material.regions...)
}

// Contacts returns all three typed contacts in increasing free order.
// Rational fields are caller-owned copies.
func (r *PolynomialTwoCrossingsTangencyCut) Contacts() [3]PolynomialMixedCutContact {
	var out [3]PolynomialMixedCutContact
	if r == nil {
		return out
	}
	for i, c := range r.material.contacts {
		out[i] = PolynomialMixedCutContact{
			TwoPolynomialNodeComponentContact: c.copy(),
			Order:                             r.orders[i], DerivativeGap: new(big.Rat).Set(r.derivatives[i]),
		}
	}
	return out
}

// CutSplineGraphPolynomialTwoCrossingsTangencyComponents cuts A by B over
// their positive shared-base identity XY charts. It retains either two
// exterior components sharing all three points or four between-branch
// components meeting successively. Each component binds both original caps.
func CutSplineGraphPolynomialTwoCrossingsTangencyComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, tolerance float64,
	o PolynomialNodeCutOptions) (*PolynomialTwoCrossingsTangencyCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		!finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	arrangement, e := geom.DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, patches[0], patches[1], axis, o.Proof)
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
		arrangement.Nodes[:], arrangement.Vertices[:], uv[:], xyz[:])
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return &PolynomialTwoCrossingsTangencyCut{
		sources: [3]*geom.BSplineSurface{base, a, b}, material: material, axis: axis, tolerance: tolerance,
		orders: arrangement.ContactOrder, derivatives: arrangement.DerivativeGap,
	}, nil
}
