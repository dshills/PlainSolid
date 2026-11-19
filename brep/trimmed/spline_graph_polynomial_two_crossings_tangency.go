//go:build research

package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PolynomialTwoCrossingsTangencyEnvelopeOptions bounds source preparation
// and the exact two-crossing and one-tangency proof.
type PolynomialTwoCrossingsTangencyEnvelopeOptions = PolynomialTangencyEnvelopeOptions

// PolynomialTwoCrossingsTangencyEnvelopeHistory maps the original branch
// restrictions, vertices and signed regions to native topology. Edges use
// increasing free order, with -1 in the second slot for unsplit restrictions.
// Midpoints indexes the two lenses in free order and their sides by branch.
type PolynomialTwoCrossingsTangencyEnvelopeHistory struct {
	Edges      [8][2]int
	Vertices   [7]int
	Midpoints  [2][2]int
	RoofFaces  [6]int
	RoofOwners [6]int // 0 for A, 1 for B
	Base       int
}

// PolynomialTwoCrossingsTangencyEnvelopeContact binds an original contact
// to its native vertex and incident roof sectors. The contact order is one
// at each crossing and two at the tangency.
type PolynomialTwoCrossingsTangencyEnvelopeContact = PolynomialMixedEnvelopeContact

// PolynomialTwoCrossingsTangencyEnvelope retains an original-source Common
// or Fuse with two transverse crossings and one quadratic tangency.
type PolynomialTwoCrossingsTangencyEnvelope struct {
	solid     Solid
	history   PolynomialTwoCrossingsTangencyEnvelopeHistory
	contacts  [3]PolynomialTwoCrossingsTangencyEnvelopeContact
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

// Boundary returns the native manifold boundary.
func (r *PolynomialTwoCrossingsTangencyEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the original-to-native topology ledger.
func (r *PolynomialTwoCrossingsTangencyEnvelope) History() PolynomialTwoCrossingsTangencyEnvelopeHistory {
	if r == nil {
		return PolynomialTwoCrossingsTangencyEnvelopeHistory{}
	}
	return r.history
}

// Contacts returns the three exact junctions in increasing free order.
// Their rational coordinates and derivative gaps are caller-owned copies.
func (r *PolynomialTwoCrossingsTangencyEnvelope) Contacts() [3]PolynomialTwoCrossingsTangencyEnvelopeContact {
	if r == nil || r.sources[0] == nil {
		return [3]PolynomialTwoCrossingsTangencyEnvelopeContact{}
	}
	out := r.contacts
	for i := range out {
		out[i] = out[i].copy()
	}
	return out
}

// CombineSplineGraphPolynomialTwoCrossingsTangency builds Common or Fuse
// on strictly positive shared-base identity XY charts. It accepts any proved
// order of the two crossings and one tangency, then binds the exact original
// roof restrictions and native vertex links. Every admitted vertex is exact.
func CombineSplineGraphPolynomialTwoCrossingsTangency(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp, tolerance float64,
	o PolynomialTwoCrossingsTangencyEnvelopeOptions) (*PolynomialTwoCrossingsTangencyEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		(op != parametric.Intersection && op != parametric.Union) || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, patches[0], patches[1], axis, o.Proof)
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
	r := &PolynomialTwoCrossingsTangencyEnvelope{solid: solid, sources: sources, axis: axis, op: op, tolerance: tolerance}
	r.history = PolynomialTwoCrossingsTangencyEnvelopeHistory{
		Base: envelope.Base, Vertices: [7]int(binding.vertices), Edges: [8][2]int(binding.chains),
		Midpoints: [2][2]int{{binding.midpoints[1], binding.midpoints[5]}, {binding.midpoints[2], binding.midpoints[6]}},
		RoofFaces: [6]int(binding.roofFaces), RoofOwners: [6]int(binding.roofOwners),
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
