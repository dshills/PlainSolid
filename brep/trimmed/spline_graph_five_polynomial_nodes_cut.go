package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// FivePolynomialNodeCut retains separately embedded positive
// layers and their five original transverse point contacts.
type FivePolynomialNodeCut struct {
	sources     [3]*geom.BSplineSurface
	material    polynomialNodeCutMaterial
	derivatives [5]*big.Rat
	axis        int
	tolerance   float64
}

// Components returns the native solids in original arrangement region order.
func (r *FivePolynomialNodeCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return append([]Solid(nil), r.material.components...)
}

// Regions returns the indices of the retained original arrangement regions.
func (r *FivePolynomialNodeCut) Regions() []int {
	if r == nil {
		return nil
	}
	return append([]int(nil), r.material.regions...)
}

// Contacts returns all five exact contacts in increasing free order.
// Rational fields are caller-owned copies.
func (r *FivePolynomialNodeCut) Contacts() [5]PolynomialMixedCutContact {
	var out [5]PolynomialMixedCutContact
	if r == nil {
		return out
	}
	for i, c := range r.material.contacts {
		out[i] = PolynomialMixedCutContact{
			TwoPolynomialNodeComponentContact: c.copy(),
			Order:                             1, DerivativeGap: new(big.Rat).Set(r.derivatives[i]),
		}
	}
	return out
}

// CutSplineGraphFivePolynomialNodeComponents cuts A by B over
// their positive shared-base identity XY charts. It retains either two
// exterior components sharing all five points or six between-branch
// components meeting successively. Each component binds both original caps.
func CutSplineGraphFivePolynomialNodeComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, tolerance float64,
	o PolynomialNodeCutOptions) (*FivePolynomialNodeCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		!finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	arrangement, e := geom.DiscoverSplineGraphPolynomialArrangement(ctx, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	if !isNativeFivePolynomialNodeArrangement(arrangement) {
		return nil, geom.ErrConvergence
	}
	uv, xyz, e := arrangement.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	material, e := assemblePolynomialCutSourceComponents(ctx, a, b, tolerance, o.Proof, arrangement.Regions, arrangement.Edges,
		arrangement.Nodes, arrangement.Vertices, uv, xyz)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return &FivePolynomialNodeCut{
		sources: [3]*geom.BSplineSurface{base, a, b}, material: material, axis: axis, tolerance: tolerance,
		derivatives: [5]*big.Rat(arrangement.DerivativeGap),
	}, nil
}
