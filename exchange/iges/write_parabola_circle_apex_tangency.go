package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaCircleApexTangencyEnvelope transfers a certified original Common
// or Fuse as one type 186 solid with both directed original uses of each
// contact edge. All model and trim poles require exact IGES decimals.
func WriteParabolaCircleApexTangencyEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleApexTangencyEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGES(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof})
}

// VerifyParabolaCircleApexTangencyEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleApexTangencyEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleApexTangencyEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "apex-tangent parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleApexTangencyEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaCircleApexTangencyCut transfers the exact positive source layers as
// a group of separate type 186 solids with one exact shared apex.
func WriteParabolaCircleApexTangencyCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleApexTangencyCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	parts, apex := body.Components(), body.Contact()
	contacts := []trimmed.PolynomialMixedCutContact{{
		TwoPolynomialNodeComponentContact: trimmed.TwoPolynomialNodeComponentContact{
			Parameters: apex.Parameters, Point: apex.Point, Components: [2]int{0, 1}, Vertices: apex.Vertices,
		},
	}}
	return writeClippedCutIGESBodies(ctx, w, parts[:], contacts, unit, l, ClippedPolynomialCutIGESOptions{Proof: proof})
}

// VerifyParabolaCircleApexTangencyCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaCircleApexTangencyCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleApexTangencyCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleApexTangencyCut(ctx, w, body, unit, l, proof)
	})
}
