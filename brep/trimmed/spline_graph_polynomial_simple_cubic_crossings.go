package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PolynomialSimpleCubicCrossingsEnvelopeOptions bounds source preparation
// and the exact simple and order-three crossing proof.
type PolynomialSimpleCubicCrossingsEnvelopeOptions = PolynomialTangencyEnvelopeOptions

// PolynomialSimpleCubicCrossingsEnvelopeHistory maps the original branch
// restrictions, vertices and signed regions to native topology. Edges use
// increasing free order, with -1 in the second slot for unsplit restrictions.
// Midpoints indexes the two subdivided sides of the interior lens by branch.
type PolynomialSimpleCubicCrossingsEnvelopeHistory struct {
	Edges      [6][2]int
	Vertices   [6]int
	Midpoints  [2]int
	RoofFaces  [5]int
	RoofOwners [5]int // 0 for A, 1 for B
	Base       int
}

// PolynomialSimpleCubicCrossingsEnvelopeContact binds an original contact
// to its native vertex and incident roof sectors. The respective orders are
// one and three, with the original derivative gap of each order.
type PolynomialSimpleCubicCrossingsEnvelopeContact = PolynomialMixedEnvelopeContact

// PolynomialSimpleCubicCrossingsEnvelope retains an original-source Common
// or Fuse with one simple and one order-three crossing.
type PolynomialSimpleCubicCrossingsEnvelope struct {
	solid     Solid
	history   PolynomialSimpleCubicCrossingsEnvelopeHistory
	contacts  [2]PolynomialSimpleCubicCrossingsEnvelopeContact
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

// Boundary returns the native manifold boundary.
func (r *PolynomialSimpleCubicCrossingsEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the original-to-native topology ledger.
func (r *PolynomialSimpleCubicCrossingsEnvelope) History() PolynomialSimpleCubicCrossingsEnvelopeHistory {
	if r == nil {
		return PolynomialSimpleCubicCrossingsEnvelopeHistory{}
	}
	return r.history
}

// Contacts returns both exact junctions in increasing free order.
// Their rational coordinates and derivative gaps are caller-owned copies.
func (r *PolynomialSimpleCubicCrossingsEnvelope) Contacts() [2]PolynomialSimpleCubicCrossingsEnvelopeContact {
	if r == nil || r.sources[0] == nil {
		return [2]PolynomialSimpleCubicCrossingsEnvelopeContact{}
	}
	out := r.contacts
	for i := range out {
		out[i] = out[i].copy()
	}
	return out
}

// CombineSplineGraphPolynomialSimpleCubicCrossings builds Common or Fuse
// on strictly positive shared-base identity XY charts. It binds both proved
// crossings, their original roof restrictions and native vertex links.
// Every admitted vertex is exact.
func CombineSplineGraphPolynomialSimpleCubicCrossings(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp, tolerance float64,
	o PolynomialSimpleCubicCrossingsEnvelopeOptions) (*PolynomialSimpleCubicCrossingsEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		(op != parametric.Intersection && op != parametric.Union) || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverSplineGraphPolynomialSimpleCubicCrossings(ctx, patches[0], patches[1], axis, o.Proof)
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
	solid, envelope, e := assemblePolynomialSourceEnvelope(ctx, base, a, b, op, tolerance, proof.Regions[:], loops, o.Proof)
	if e != nil {
		return nil, e
	}
	sources := [3]*geom.BSplineSurface{base, a, b}
	binding, e := bindPolynomialEnvelopeSources(solid, sources, op, proof.Edges[:], proof.Regions[:], xyz[:], pieces, loops, envelope)
	if e != nil {
		return nil, e
	}
	r := &PolynomialSimpleCubicCrossingsEnvelope{solid: solid, sources: sources, axis: axis, op: op, tolerance: tolerance}
	r.history = PolynomialSimpleCubicCrossingsEnvelopeHistory{
		Base: envelope.Base, Vertices: [6]int(binding.vertices), Edges: [6][2]int(binding.chains),
		Midpoints: [2]int{binding.midpoints[1], binding.midpoints[4]},
		RoofFaces: [5]int(binding.roofFaces), RoofOwners: [5]int(binding.roofOwners),
	}
	for i, node := range proof.Nodes {
		c, e := bindPolynomialEnvelopeJunction(solid, binding, proof.Edges[:], node, proof.Vertices[node.Vertex], xyz[node.Vertex])
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
