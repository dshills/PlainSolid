package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/parabolacircle"
	"plainsolid/geom"
)

// WriteParabolaCircleObliqueTangencyEnvelope transfers a certified original Common
// or Fuse as one analytic solid with the original tangent junction and exact normal roof bridge.
// The bridge may be re-expressed in an equivalent degree-five net. All model
// and trim poles require exact STEP decimals.
func WriteParabolaCircleObliqueTangencyEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleObliqueTangencyEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	bridge := body.History().Bridge
	prepare := func(ctx context.Context, b trimmed.ExactBezierBoundary, proof edgecheck.Options) error {
		return parabolacircle.PrepareNormalBridge(ctx, b, bridge, proof, mappedChainStepDecimal)
	}
	return writeExactBezierSingleSTEPWithBoundary(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof}, false, prepare)
}

// VerifyParabolaCircleObliqueTangencyEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleObliqueTangencyEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleObliqueTangencyEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "nonapex-tangent parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleObliqueTangencyEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaCircleObliqueTangencyCut transfers the exact positive source layers as
// a group of two separate solids with one exact shared nonapex tangent point.
func WriteParabolaCircleObliqueTangencyCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleObliqueTangencyCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	parts, contact := body.Components(), body.Contact()
	contacts := []trimmed.PolynomialMixedCutContact{{
		TwoPolynomialNodeComponentContact: trimmed.TwoPolynomialNodeComponentContact{
			Parameters: contact.Parameters, Point: contact.Point, Components: [2]int{0, 1}, Vertices: contact.Vertices,
		},
	}}
	return writeClippedCutSTEPBodies(ctx, w, parts[:], contacts, name, unit, l, ClippedPolynomialCutSTEPOptions{Proof: proof})
}

// VerifyParabolaCircleObliqueTangencyCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaCircleObliqueTangencyCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleObliqueTangencyCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleObliqueTangencyCut(ctx, w, body, name, unit, l, proof)
	})
}
