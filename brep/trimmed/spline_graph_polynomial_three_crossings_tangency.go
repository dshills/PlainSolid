//go:build research

package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PolynomialThreeCrossingsTangencyEnvelopeOptions bounds source preparation
// and the exact three-crossing and one-tangency proof.
type PolynomialThreeCrossingsTangencyEnvelopeOptions = PolynomialTangencyEnvelopeOptions

// PolynomialThreeCrossingsTangencyEnvelopeHistory maps the original branch
// restrictions, vertices and signed regions to native topology. Edges use
// increasing free order, with -1 in the second slot for unsplit restrictions.
// Midpoints indexes the three lenses in free order and their sides by branch.
type PolynomialThreeCrossingsTangencyEnvelopeHistory struct {
	Edges      [10][2]int
	Vertices   [8]int
	Midpoints  [3][2]int
	RoofFaces  [7]int
	RoofOwners [7]int // 0 for A, 1 for B
	Base       int
}

// PolynomialThreeCrossingsTangencyEnvelopeContact binds an original contact
// to its native vertex and incident roof sectors. The contact order is one
// at each crossing and two at the tangency.
type PolynomialThreeCrossingsTangencyEnvelopeContact = PolynomialMixedEnvelopeContact

// PolynomialThreeCrossingsTangencyEnvelope retains an original-source Common
// or Fuse with three transverse crossings and one quadratic tangency.
type PolynomialThreeCrossingsTangencyEnvelope struct {
	solid     Solid
	history   PolynomialThreeCrossingsTangencyEnvelopeHistory
	contacts  [4]PolynomialThreeCrossingsTangencyEnvelopeContact
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

// Boundary returns the native manifold boundary.
func (r *PolynomialThreeCrossingsTangencyEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the original-to-native topology ledger.
func (r *PolynomialThreeCrossingsTangencyEnvelope) History() PolynomialThreeCrossingsTangencyEnvelopeHistory {
	if r == nil {
		return PolynomialThreeCrossingsTangencyEnvelopeHistory{}
	}
	return r.history
}

// Contacts returns the four exact junctions in increasing free order.
// Their rational coordinates and derivative gaps are caller-owned copies.
func (r *PolynomialThreeCrossingsTangencyEnvelope) Contacts() [4]PolynomialThreeCrossingsTangencyEnvelopeContact {
	if r == nil || r.sources[0] == nil {
		return [4]PolynomialThreeCrossingsTangencyEnvelopeContact{}
	}
	out := r.contacts
	for i := range out {
		out[i] = out[i].copy()
	}
	return out
}

// CombineSplineGraphPolynomialThreeCrossingsTangency builds Common or Fuse
// on strictly positive shared-base identity XY charts. It accepts any proved
// order of the three crossings and one tangency, then binds the exact original
// roof restrictions and native vertex links. Every admitted vertex is exact.
func CombineSplineGraphPolynomialThreeCrossingsTangency(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp, tolerance float64,
	o PolynomialThreeCrossingsTangencyEnvelopeOptions) (*PolynomialThreeCrossingsTangencyEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		(op != parametric.Intersection && op != parametric.Union) || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverSplineGraphPolynomialThreeCrossingsTangency(ctx, patches[0], patches[1], axis, o.Proof)
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
	r := &PolynomialThreeCrossingsTangencyEnvelope{solid: solid, sources: sources, axis: axis, op: op, tolerance: tolerance}
	r.history = PolynomialThreeCrossingsTangencyEnvelopeHistory{
		Base: envelope.Base, Vertices: [8]int(binding.vertices), Edges: [10][2]int(binding.chains),
		Midpoints: [3][2]int{{binding.midpoints[1], binding.midpoints[6]}, {binding.midpoints[2], binding.midpoints[7]}, {binding.midpoints[3], binding.midpoints[8]}},
		RoofFaces: [7]int(binding.roofFaces), RoofOwners: [7]int(binding.roofOwners),
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
