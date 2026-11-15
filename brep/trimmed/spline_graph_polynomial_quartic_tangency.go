package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PolynomialQuarticTangencyEnvelopeOptions bounds source preparation
// and the exact order-four tangency proof.
type PolynomialQuarticTangencyEnvelopeOptions = PolynomialTangencyEnvelopeOptions

// PolynomialQuarticTangencyEnvelopeHistory maps the original branch
// restrictions, vertices and signed roof regions to native topology. The
// four original branch pieces remain unsplit, with -1 in the second slot.
type PolynomialQuarticTangencyEnvelopeHistory struct {
	Edges      [4][2]int
	Vertices   [5]int
	RoofFaces  [4]int
	RoofOwners [4]int // 0 for A, 1 for B
	Base       int
}

// PolynomialQuarticTangencyEnvelopeContact binds an original contact
// to its native vertex and incident roof sectors. It has order four and
// retains the original fourth derivative gap.
type PolynomialQuarticTangencyEnvelopeContact = PolynomialMixedEnvelopeContact

// PolynomialQuarticTangencyEnvelope retains an original-source Common
// or Fuse with one isolated order-four contact.
type PolynomialQuarticTangencyEnvelope struct {
	solid     Solid
	history   PolynomialQuarticTangencyEnvelopeHistory
	contact   PolynomialQuarticTangencyEnvelopeContact
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

// Boundary returns the native manifold boundary.
func (r *PolynomialQuarticTangencyEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the original-to-native topology ledger.
func (r *PolynomialQuarticTangencyEnvelope) History() PolynomialQuarticTangencyEnvelopeHistory {
	if r == nil {
		return PolynomialQuarticTangencyEnvelopeHistory{}
	}
	return r.history
}

// Contact returns the exact order-four junction. Its rational fields are
// caller-owned copies.
func (r *PolynomialQuarticTangencyEnvelope) Contact() PolynomialQuarticTangencyEnvelopeContact {
	if r == nil || r.sources[0] == nil {
		return PolynomialQuarticTangencyEnvelopeContact{}
	}
	return r.contact.copy()
}

// CombineSplineGraphPolynomialQuarticTangency builds Common or Fuse
// on strictly positive shared-base identity XY charts. It binds the proved
// order-four tangency, the original roof restrictions and the native vertex
// link. Every admitted vertex is exact.
func CombineSplineGraphPolynomialQuarticTangency(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp, tolerance float64,
	o PolynomialQuarticTangencyEnvelopeOptions) (*PolynomialQuarticTangencyEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		(op != parametric.Intersection && op != parametric.Union) || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverSplineGraphPolynomialQuarticTangency(ctx, patches[0], patches[1], axis, o.Proof)
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
	r := &PolynomialQuarticTangencyEnvelope{solid: solid, sources: sources, axis: axis, op: op, tolerance: tolerance}
	r.history = PolynomialQuarticTangencyEnvelopeHistory{
		Base: envelope.Base, Vertices: [5]int(binding.vertices), Edges: [4][2]int(binding.chains),
		RoofFaces: [4]int(binding.roofFaces), RoofOwners: [4]int(binding.roofOwners),
	}
	node := proof.Node
	r.contact, e = bindPolynomialEnvelopeJunction(solid, binding, proof.Edges[:], node, proof.Vertices[node.Vertex], xyz[node.Vertex])
	if e != nil {
		return nil, e
	}
	r.contact.Order, r.contact.DerivativeGap = 4, proof.FourthDerivativeGap
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return r, nil
}
