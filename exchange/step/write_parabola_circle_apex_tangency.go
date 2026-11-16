package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaCircleApexTangencyEnvelope transfers a certified original Common
// or Fuse as one analytic solid with the original tangent junction and enclosing-roof bridge.
// All model and trim poles require exact STEP decimals.
func WriteParabolaCircleApexTangencyEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleApexTangencyEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof})
}

// VerifyParabolaCircleApexTangencyEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleApexTangencyEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleApexTangencyEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "apex-tangent parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleApexTangencyEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaCircleApexTangencyCut transfers the exact positive source layers as
// a group of two separate solids with one exact shared apex.
func WriteParabolaCircleApexTangencyCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleApexTangencyCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	parts, apex := body.Components(), body.Contact()
	contacts := []trimmed.PolynomialMixedCutContact{{
		TwoPolynomialNodeComponentContact: trimmed.TwoPolynomialNodeComponentContact{
			Parameters: apex.Parameters, Point: apex.Point, Components: [2]int{0, 1}, Vertices: apex.Vertices,
		},
	}}
	return writeClippedCutSTEPBodies(ctx, w, parts[:], contacts, name, unit, l, ClippedPolynomialCutSTEPOptions{Proof: proof})
}

// VerifyParabolaCircleApexTangencyCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaCircleApexTangencyCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleApexTangencyCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleApexTangencyCut(ctx, w, body, name, unit, l, proof)
	})
}
