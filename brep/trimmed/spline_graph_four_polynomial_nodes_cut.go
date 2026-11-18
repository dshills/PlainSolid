//go:build research

package trimmed

import (
	"context"

	"plainsolid/geom"
)

// FourPolynomialNodeComponentContact records the exact original junction and
// its vertex identities local to the two incident components.
type FourPolynomialNodeComponentContact = TwoPolynomialNodeComponentContact

// FourPolynomialNodeCut retains the original roofs, positive native layers,
// and the four exact junctions shared between layers.
type FourPolynomialNodeCut struct {
	sources   [3]*geom.BSplineSurface
	material  polynomialNodeCutMaterial
	axis      int
	tolerance float64
}

// Components returns the native solids in original arrangement region order.
func (r *FourPolynomialNodeCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return append([]Solid(nil), r.material.components...)
}

// Regions returns the indices of the retained original arrangement regions.
func (r *FourPolynomialNodeCut) Regions() []int {
	if r == nil {
		return nil
	}
	return append([]int(nil), r.material.regions...)
}

// Contacts returns the four junctions in increasing free order.
// The exact rational coordinates are caller-owned copies.
func (r *FourPolynomialNodeCut) Contacts() [4]FourPolynomialNodeComponentContact {
	var out [4]FourPolynomialNodeComponentContact
	if r == nil {
		return out
	}
	for i, c := range r.material.contacts {
		out[i] = c.copy()
	}
	return out
}

// CutSplineGraphFourPolynomialNodeComponents cuts A by B over their positive
// shared-base identity XY charts. Two exterior components share all four
// junctions, or five between-branch components meet successively. Each
// component binds both original roof caps and keeps its own vertex namespace.
func CutSplineGraphFourPolynomialNodeComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, tolerance float64, o PolynomialNodeCutOptions) (*FourPolynomialNodeCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		!finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	arrangement, e := geom.DiscoverSplineGraphFourPolynomialNodes(ctx, patches[0], patches[1], axis, o.Proof)
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
	material, e := assemblePolynomialNodeCutComponents(ctx, a, b, tolerance, o.Proof, arrangement.Regions[:], arrangement.Nodes[:], arrangement.Vertices[:], xyz[:], loops, bindLayer)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return &FourPolynomialNodeCut{sources: [3]*geom.BSplineSurface{base, a, b}, axis: axis, tolerance: tolerance, material: material}, nil
}
