package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaCircleGraphEnvelope transfers a certified original Common
// or Fuse as one analytic solid, with both proper open and closed contour
// owners. All model and trim poles require exact STEP decimals.
func WriteParabolaCircleGraphEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleGraphEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof})
}

// VerifyParabolaCircleGraphEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleGraphEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleGraphEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "parabola-circle graph envelope", func(w io.Writer) error {
		return WriteParabolaCircleGraphEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaCircleGraphCut transfers the exact positive source layers as
// one through-hole solid or two disjoint solids.
func WriteParabolaCircleGraphCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleGraphCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	parts := body.Components()
	o := ClippedPolynomialCutSTEPOptions{Proof: proof}
	switch len(parts) {
	case 1:
		return writeExactBezierSingleSTEP(ctx, w, parts[0], name, unit, l, o)
	case 2:
		return writeClippedCutSTEPBodies(ctx, w, parts, nil, name, unit, l, o)
	default:
		return geom.ErrInvalid
	}
}

// VerifyParabolaCircleGraphCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaCircleGraphCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleGraphCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "parabola-circle graph Cut", func(w io.Writer) error {
		return WriteParabolaCircleGraphCut(ctx, w, body, name, unit, l, proof)
	})
}
