package trimmed

import (
	"context"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// FivePolynomialNodeEnvelopeOptions bounds preparation and exact
// five-crossing source proofs for a shared-base Common or Fuse.
type FivePolynomialNodeEnvelopeOptions = PolynomialTangencyEnvelopeOptions

// FivePolynomialNodeEnvelopeHistory maps each original restriction, vertex
// and signed region to native topology. Edges gives the native subdivisions
// in increasing free order, with -1 in the second slot for unsplit edges.
// Midpoints indexes the lenses in free order and their sides by branch.
type FivePolynomialNodeEnvelopeHistory struct {
	Edges      [12][2]int
	Vertices   [9]int
	Midpoints  [4][2]int
	RoofFaces  [8]int
	RoofOwners [8]int // 0 for A, 1 for B
	Base       int
}

// FivePolynomialNodeEnvelopeContact records an exact transverse vertex, its
// original slope gap, and its native rays and roof sectors. Order is one.
type FivePolynomialNodeEnvelopeContact = PolynomialMixedEnvelopeContact

// FivePolynomialNodeEnvelope retains a source-backed native Common or Fuse
// with five transverse junctions and four bounded lenses.
type FivePolynomialNodeEnvelope struct {
	solid     Solid
	history   FivePolynomialNodeEnvelopeHistory
	contacts  [5]FivePolynomialNodeEnvelopeContact
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

// Boundary returns the native manifold boundary.
func (r *FivePolynomialNodeEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the original-to-native topology ledger.
func (r *FivePolynomialNodeEnvelope) History() FivePolynomialNodeEnvelopeHistory {
	if r == nil {
		return FivePolynomialNodeEnvelopeHistory{}
	}
	return r.history
}

// Contacts returns the five exact junctions in increasing free order.
// The rational values are caller-owned copies.
func (r *FivePolynomialNodeEnvelope) Contacts() [5]FivePolynomialNodeEnvelopeContact {
	if r == nil || r.sources[0] == nil {
		return [5]FivePolynomialNodeEnvelopeContact{}
	}
	out := r.contacts
	for i := range out {
		out[i] = out[i].copy()
	}
	return out
}

// CombineSplineGraphFivePolynomialNodes builds a native Common or Fuse
// from the complete original shared-base five-crossing arrangement. It
// retains both source supports, exact coedges, and eight signed roof
// sectors. Native UV and XYZ vertices must be losslessly representable.
func CombineSplineGraphFivePolynomialNodes(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp, tolerance float64,
	o FivePolynomialNodeEnvelopeOptions) (*FivePolynomialNodeEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		(op != parametric.Intersection && op != parametric.Union) || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverSplineGraphPolynomialArrangement(ctx, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	if !isNativeFivePolynomialNodeArrangement(proof) {
		return nil, geom.ErrConvergence
	}
	uv, xyz, e := proof.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	pieces, e := polynomialNodePieces(proof.Edges, uv)
	if e != nil {
		return nil, e
	}
	loops, e := polynomialNodeRegionLoops(proof.Regions, uv, pieces)
	if e != nil {
		return nil, e
	}
	solid, envelope, e := assemblePolynomialSourceEnvelope(ctx, base, a, b, op, tolerance, proof.Regions, loops, o.Proof)
	if e != nil {
		return nil, e
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	binding, e := bindPolynomialEnvelopeSources(solid, sources, op, proof.Edges, proof.Regions, xyz, pieces, loops, envelope)
	if e != nil {
		return nil, e
	}
	r := &FivePolynomialNodeEnvelope{solid: solid, sources: sources, axis: axis, op: op, tolerance: tolerance}
	r.history = FivePolynomialNodeEnvelopeHistory{
		Base: envelope.Base, Vertices: [9]int(binding.vertices), Edges: [12][2]int(binding.chains),
		RoofFaces: [8]int(binding.roofFaces), RoofOwners: [8]int(binding.roofOwners),
	}
	for i := range r.history.Midpoints {
		r.history.Midpoints[i] = [2]int{binding.midpoints[i+1], binding.midpoints[i+7]}
	}
	for i, node := range proof.Nodes {
		c, e := bindPolynomialEnvelopeJunction(solid, binding, proof.Edges, node, proof.Vertices[node.Vertex], xyz[node.Vertex])
		if e != nil {
			return nil, e
		}
		c.Order, c.DerivativeGap = proof.ContactOrder[i], proof.DerivativeGap[i]
		r.contacts[i] = c
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return r, nil
}

func isNativeFivePolynomialNodeArrangement(proof geom.SplineGraphPolynomialArrangement) bool {
	return proof.Proven && proof.Domain == (geom.ParameterRectangle{UMax: 1, VMax: 1}) &&
		proof.Graph.ProjectionOrientation == 1 && slices.Equal(proof.ContactOrder, []int{1, 1, 1, 1, 1})
}
