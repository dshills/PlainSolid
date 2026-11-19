//go:build research

package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PolynomialTwoTangenciesEnvelopeOptions bounds source preparation
// and the exact two-tangency proof.
type PolynomialTwoTangenciesEnvelopeOptions = PolynomialTangencyEnvelopeOptions

// PolynomialTwoTangenciesEnvelopeHistory maps the original branch
// restrictions, vertices and signed regions to native topology. Edges use
// increasing free order, with -1 in the second slot for unsplit restrictions.
// Midpoints indexes the two subdivided sides of the interior lens by branch.
type PolynomialTwoTangenciesEnvelopeHistory struct {
	Edges      [6][2]int
	Vertices   [6]int
	Midpoints  [2]int
	RoofFaces  [5]int
	RoofOwners [5]int // 0 for A, 1 for B
	Base       int
}

// PolynomialTwoTangenciesEnvelopeContact binds an original contact
// to its native vertex and incident roof sectors. Both contacts have order
// two and retain the original second derivative gap.
type PolynomialTwoTangenciesEnvelopeContact = PolynomialMixedEnvelopeContact

// PolynomialTwoTangenciesEnvelope retains an original-source Common
// or Fuse with two distinct quadratic tangencies.
type PolynomialTwoTangenciesEnvelope struct {
	solid     Solid
	history   PolynomialTwoTangenciesEnvelopeHistory
	contacts  [2]PolynomialTwoTangenciesEnvelopeContact
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

// Boundary returns the native manifold boundary.
func (r *PolynomialTwoTangenciesEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the original-to-native topology ledger.
func (r *PolynomialTwoTangenciesEnvelope) History() PolynomialTwoTangenciesEnvelopeHistory {
	if r == nil {
		return PolynomialTwoTangenciesEnvelopeHistory{}
	}
	return r.history
}

// Contacts returns both exact junctions in increasing free order.
// Their rational coordinates and derivative gaps are caller-owned copies.
func (r *PolynomialTwoTangenciesEnvelope) Contacts() [2]PolynomialTwoTangenciesEnvelopeContact {
	if r == nil || r.sources[0] == nil {
		return [2]PolynomialTwoTangenciesEnvelopeContact{}
	}
	out := r.contacts
	for i := range out {
		out[i] = out[i].copy()
	}
	return out
}

// CombineSplineGraphPolynomialTwoTangencies builds Common or Fuse
// on strictly positive shared-base identity XY charts. It binds the two
// proved quadratic tangencies, exact original roof restrictions and native
// vertex links. Every admitted vertex is exact.
func CombineSplineGraphPolynomialTwoTangencies(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp, tolerance float64,
	o PolynomialTwoTangenciesEnvelopeOptions) (*PolynomialTwoTangenciesEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		(op != parametric.Intersection && op != parametric.Union) || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverSplineGraphPolynomialTwoTangencies(ctx, patches[0], patches[1], axis, o.Proof)
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
	r := &PolynomialTwoTangenciesEnvelope{solid: solid, sources: sources, axis: axis, op: op, tolerance: tolerance}
	r.history = PolynomialTwoTangenciesEnvelopeHistory{
		Base: envelope.Base, Vertices: [6]int(binding.vertices), Edges: [6][2]int(binding.chains),
		Midpoints: [2]int{binding.midpoints[1], binding.midpoints[4]},
		RoofFaces: [5]int(binding.roofFaces), RoofOwners: [5]int(binding.roofOwners),
	}
	for i, node := range proof.Nodes {
		c, e := bindPolynomialEnvelopeJunction(solid, binding, proof.Edges[:], node, proof.Vertices[node.Vertex], xyz[node.Vertex])
		if e != nil {
			return nil, e
		}
		c.Order, c.DerivativeGap = 2, proof.SecondDerivativeGaps[i]
		r.contacts[i] = c
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return r, nil
}
