package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaTwoCirclesGraphEnvelope transfers a certified original Common
// or Fuse as one type 186 solid with both directed original uses of each
// contact edge. All model and trim poles require exact IGES decimals.
func WriteParabolaTwoCirclesGraphEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaTwoCirclesGraphEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGES(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof})
}

// VerifyParabolaTwoCirclesGraphEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaTwoCirclesGraphEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaTwoCirclesGraphEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "parabola and two circles graph envelope", func(w io.Writer) error {
		return WriteParabolaTwoCirclesGraphEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaTwoCirclesGraphCut transfers the exact positive source layers as
// one type 186 solid or a group of separate type 186 solids.
func WriteParabolaTwoCirclesGraphCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaTwoCirclesGraphCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	parts := body.Components()
	o := ClippedPolynomialCutIGESOptions{Proof: proof}
	switch len(parts) {
	case 1:
		return writeExactBezierSingleIGES(ctx, w, parts[0], unit, l, o)
	case 2, 3:
		return writeClippedCutIGESBodies(ctx, w, parts, nil, unit, l, o)
	default:
		return geom.ErrInvalid
	}
}

// VerifyParabolaTwoCirclesGraphCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaTwoCirclesGraphCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaTwoCirclesGraphCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	write := func(w io.Writer) error {
		return WriteParabolaTwoCirclesGraphCut(ctx, w, body, unit, l, proof)
	}
	switch len(body.Components()) {
	case 1:
		return verifyExactBezierSingleIGES(ctx, in, l, "parabola and two circles graph Cut", write)
	case 2, 3:
		return verifyClippedCutIGESBodies(ctx, in, l, write)
	default:
		return geom.ErrInvalid
	}
}
