package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaCircleGraphEnvelope transfers a certified original Common
// or Fuse as one type 186 solid with both directed original uses of each
// contact edge. All model and trim poles require exact IGES decimals.
func WriteParabolaCircleGraphEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleGraphEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGES(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof})
}

// VerifyParabolaCircleGraphEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleGraphEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleGraphEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "parabola-circle graph envelope", func(w io.Writer) error {
		return WriteParabolaCircleGraphEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaCircleGraphCut transfers the exact positive source layers as
// one through-hole solid or two disjoint type 186 solids in a group.
func WriteParabolaCircleGraphCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleGraphCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	parts := body.Components()
	o := ClippedPolynomialCutIGESOptions{Proof: proof}
	switch len(parts) {
	case 1:
		return writeExactBezierSingleIGES(ctx, w, parts[0], unit, l, o)
	case 2:
		return writeClippedCutIGESBodies(ctx, w, parts, nil, unit, l, o)
	default:
		return geom.ErrInvalid
	}
}

// VerifyParabolaCircleGraphCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaCircleGraphCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleGraphCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	write := func(w io.Writer) error {
		return WriteParabolaCircleGraphCut(ctx, w, body, unit, l, proof)
	}
	switch len(body.Components()) {
	case 1:
		return verifyExactBezierSingleIGES(ctx, in, l, "parabola-circle graph Cut", write)
	case 2:
		return verifyClippedCutIGESBodies(ctx, in, l, write)
	default:
		return geom.ErrInvalid
	}
}
