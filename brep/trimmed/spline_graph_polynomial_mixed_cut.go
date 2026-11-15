package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// PolynomialMixedCutContact binds the original crossing or tangency to the
// vertex IDs local to its incident components. Order is one at the crossing
// and two at the tangency. DerivativeGap records the original branch
// difference derivative of that order at the contact.
type PolynomialMixedCutContact struct {
	TwoPolynomialNodeComponentContact
	Order         int
	DerivativeGap *big.Rat
}

// PolynomialMixedCut retains the positive native layers and both original
// exact contacts. The components have separate topology and vertex namespaces.
type PolynomialMixedCut struct {
	sources     [3]*geom.BSplineSurface
	material    polynomialNodeCutMaterial
	orders      [2]int
	derivatives [2]*big.Rat
	axis        int
	tolerance   float64
}

// Components returns the native solids in original arrangement region order.
func (r *PolynomialMixedCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return append([]Solid(nil), r.material.components...)
}

// Regions returns the indices of the retained original arrangement regions.
func (r *PolynomialMixedCut) Regions() []int {
	if r == nil {
		return nil
	}
	return append([]int(nil), r.material.regions...)
}

// Contacts returns the original crossing and tangency in increasing free
// order. Rational coordinates and derivative gaps are caller-owned copies.
func (r *PolynomialMixedCut) Contacts() [2]PolynomialMixedCutContact {
	var out [2]PolynomialMixedCutContact
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

// CutSplineGraphPolynomialMixedComponents cuts A by B across one transverse
// and one tangent junction on their original shared-base identity XY charts.
// It proves either two exterior components sharing both points, or three
// between-branch components meeting successively. Every published component
// binds its roof caps to the exact signed original sector.
func CutSplineGraphPolynomialMixedComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, tolerance float64, o PolynomialNodeCutOptions) (*PolynomialMixedCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		!finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	arrangement, e := geom.DiscoverSplineGraphPolynomialMixedContact(ctx, patches[0], patches[1], axis, o.Proof)
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
	pieces, e := polynomialNodePieces(arrangement.Edges[:], uv[:])
	if e != nil {
		return nil, e
	}
	loops, e := polynomialNodeRegionLoops(arrangement.Regions[:], uv[:], pieces)
	if e != nil {
		return nil, e
	}
	bindLayer := func(part Solid, history OpenSectionHistory, loop []openSectionPiece) error {
		return bindPolynomialCutSourceLayer(ctx, part, history, [2]*geom.BSplineSurface{a, b}, loop, o.Proof)
	}
	material, e := assemblePolynomialNodeCutComponents(ctx, a, b, tolerance, o.Proof,
		arrangement.Regions[:], arrangement.Nodes[:], arrangement.Vertices[:], xyz[:], loops, bindLayer)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return &PolynomialMixedCut{
		sources: [3]*geom.BSplineSurface{base, a, b}, material: material,
		orders: arrangement.ContactOrder, derivatives: arrangement.DerivativeGap, axis: axis, tolerance: tolerance,
	}, nil
}
