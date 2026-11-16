package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaTwoCirclesGraphEnvelope transfers a certified original Common
// or Fuse as one analytic solid, with one open contour and two closed rings.
// All model and trim poles require exact STEP decimals.
func WriteParabolaTwoCirclesGraphEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaTwoCirclesGraphEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof})
}

// VerifyParabolaTwoCirclesGraphEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaTwoCirclesGraphEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaTwoCirclesGraphEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "parabola and two circles graph envelope", func(w io.Writer) error {
		return WriteParabolaTwoCirclesGraphEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaTwoCirclesGraphCut transfers the exact positive source layers as
// one solid with two through holes or three disjoint solids.
func WriteParabolaTwoCirclesGraphCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaTwoCirclesGraphCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	parts := body.Components()
	o := ClippedPolynomialCutSTEPOptions{Proof: proof}
	switch len(parts) {
	case 1:
		return writeExactBezierSingleSTEP(ctx, w, parts[0], name, unit, l, o)
	case 3:
		return writeClippedCutSTEPBodies(ctx, w, parts, nil, name, unit, l, o)
	default:
		return geom.ErrInvalid
	}
}

// VerifyParabolaTwoCirclesGraphCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaTwoCirclesGraphCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaTwoCirclesGraphCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "parabola and two circles graph Cut", func(w io.Writer) error {
		return WriteParabolaTwoCirclesGraphCut(ctx, w, body, name, unit, l, proof)
	})
}
