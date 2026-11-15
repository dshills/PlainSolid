package trimmed

import (
	"context"

	"plainsolid/geom"
)

// ThreePolynomialNodeComponentContact binds an original junction to the
// local vertex IDs of the two incident Cut components. The components may
// share more than one junction.
type ThreePolynomialNodeComponentContact = TwoPolynomialNodeComponentContact

// ThreePolynomialNodeCut retains the original source recipe and separately
// embedded, positive native shells with a complete exact contact ledger.
type ThreePolynomialNodeCut struct {
	sources    [3]*geom.BSplineSurface
	components []Solid
	regions    []int
	contacts   [3]ThreePolynomialNodeComponentContact
	axis       int
	tolerance  float64
}

// Components returns the native solids in original arrangement region order.
func (r *ThreePolynomialNodeCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return append([]Solid(nil), r.components...)
}

// Regions returns the indices of the retained original arrangement regions.
func (r *ThreePolynomialNodeCut) Regions() []int {
	if r == nil {
		return nil
	}
	return append([]int(nil), r.regions...)
}

// Contacts returns all three exact junctions in increasing free-coordinate
// order. The returned rational coordinates are caller-owned copies.
func (r *ThreePolynomialNodeCut) Contacts() [3]ThreePolynomialNodeComponentContact {
	if r == nil || r.sources[0] == nil {
		return [3]ThreePolynomialNodeComponentContact{}
	}
	out := r.contacts
	for i, c := range out {
		out[i] = c.copy()
	}
	return out
}

// CutSplineGraphThreePolynomialNodeComponents cuts A by B across a complete
// three-node arrangement on their original shared-base identity XY charts.
// It proves two exterior components sharing all three nodes, or four
// between-branch components with three successive point contacts. Each
// component keeps its own topology and exact original roof supports.
func CutSplineGraphThreePolynomialNodeComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, tolerance float64, o PolynomialNodeCutOptions) (*ThreePolynomialNodeCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		!finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	arrangement, e := geom.DiscoverSplineGraphThreePolynomialNodes(ctx, patches[0], patches[1], axis, o.Proof)
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
	material, e := assemblePolynomialNodeCutComponents(ctx, a, b, tolerance, o.Proof, arrangement.Regions[:], arrangement.Nodes[:], arrangement.Vertices[:], xyz[:], loops, nil)
	if e != nil {
		return nil, e
	}
	return &ThreePolynomialNodeCut{
		sources: [3]*geom.BSplineSurface{base, a, b}, axis: axis, tolerance: tolerance,
		components: material.components, regions: material.regions, contacts: [3]ThreePolynomialNodeComponentContact(material.contacts),
	}, nil
}
