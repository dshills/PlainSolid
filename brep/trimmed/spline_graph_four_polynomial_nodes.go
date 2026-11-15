package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// FourPolynomialNodeEnvelopeOptions bounds preparation and exact
// four-crossing source proofs for a shared-base Common or Fuse.
type FourPolynomialNodeEnvelopeOptions = PolynomialTangencyEnvelopeOptions

// FourPolynomialNodeEnvelopeHistory maps each original restriction, vertex
// and signed region to native topology. Edges gives the native subdivisions
// in increasing free order, with -1 in the second slot for unsplit edges.
// Midpoints indexes the lenses in free order and their sides by branch.
type FourPolynomialNodeEnvelopeHistory struct {
	Edges      [10][2]int
	Vertices   [8]int
	Midpoints  [3][2]int
	RoofFaces  [7]int
	RoofOwners [7]int // 0 for A, 1 for B
	Base       int
}

// FourPolynomialNodeEnvelopeContact records an exact transverse vertex, its
// original slope gap, and its native rays and roof sectors. Order is one.
type FourPolynomialNodeEnvelopeContact = PolynomialMixedEnvelopeContact

// FourPolynomialNodeEnvelope retains a source-backed native Common or Fuse
// with four transverse junctions and three bounded lenses.
type FourPolynomialNodeEnvelope struct {
	solid     Solid
	history   FourPolynomialNodeEnvelopeHistory
	contacts  [4]FourPolynomialNodeEnvelopeContact
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

// Boundary returns the native manifold boundary.
func (r *FourPolynomialNodeEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the original-to-native topology ledger.
func (r *FourPolynomialNodeEnvelope) History() FourPolynomialNodeEnvelopeHistory {
	if r == nil {
		return FourPolynomialNodeEnvelopeHistory{}
	}
	return r.history
}

// Contacts returns the four exact junctions in increasing free order.
// The rational values are caller-owned copies.
func (r *FourPolynomialNodeEnvelope) Contacts() [4]FourPolynomialNodeEnvelopeContact {
	if r == nil || r.sources[0] == nil {
		return [4]FourPolynomialNodeEnvelopeContact{}
	}
	out := r.contacts
	for i := range out {
		out[i] = out[i].copy()
	}
	return out
}

// CombineSplineGraphFourPolynomialNodes builds a native Common or Fuse
// from the complete original shared-base four-crossing arrangement. It
// retains both source supports, exact coedges, and seven signed roof
// sectors. Native UV and XYZ vertices must be losslessly representable.
func CombineSplineGraphFourPolynomialNodes(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp, tolerance float64,
	o FourPolynomialNodeEnvelopeOptions) (*FourPolynomialNodeEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		(op != parametric.Intersection && op != parametric.Union) || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverSplineGraphFourPolynomialNodes(ctx, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	if !proof.Proven || proof.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || proof.Graph.ProjectionOrientation != 1 {
		return nil, geom.ErrConvergence
	}
	uv, xyz, e := proof.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	pieces, e := polynomialNodePieces(proof.Edges[:], uv[:])
	if e != nil {
		return nil, e
	}
	loops, e := polynomialNodeRegionLoops(proof.Regions[:], uv[:], pieces)
	if e != nil {
		return nil, e
	}
	solid, envelope, e := assemblePolynomialNodeEnvelope(ctx, base, a, b, op, tolerance, proof.Regions[:], loops, o.Proof)
	if e != nil {
		return nil, e
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	binding, e := bindPolynomialEnvelopeSources(solid, sources, op, proof.Edges[:], proof.Regions[:], xyz[:], pieces, loops, envelope)
	if e != nil {
		return nil, e
	}
	r := &FourPolynomialNodeEnvelope{solid: solid, sources: sources, axis: axis, op: op, tolerance: tolerance}
	r.history = FourPolynomialNodeEnvelopeHistory{
		Base: envelope.Base, Vertices: [8]int(binding.vertices), Edges: [10][2]int(binding.chains),
		RoofFaces: [7]int(binding.roofFaces), RoofOwners: [7]int(binding.roofOwners),
	}
	for i := range r.history.Midpoints {
		r.history.Midpoints[i] = [2]int{binding.midpoints[i+1], binding.midpoints[i+6]}
	}
	for i, node := range proof.Nodes {
		c, e := bindPolynomialEnvelopeJunction(solid, binding, proof.Edges[:], node, proof.Vertices[node.Vertex], xyz[node.Vertex])
		if e != nil {
			return nil, e
		}
		c.Order = 1
		c.DerivativeGap = new(big.Rat).Sub(node.Tangents[1][1-axis], node.Tangents[0][1-axis])
		r.contacts[i] = c
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return r, nil
}
