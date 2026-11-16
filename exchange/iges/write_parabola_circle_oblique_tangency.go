package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/parabolacircle"
	"plainsolid/geom"
)

// WriteParabolaCircleObliqueTangencyEnvelope transfers a certified original Common
// or Fuse as one type 186 solid with both original uses of the contact edges
// and its exact normal roof bridge. The bridge may be re-expressed in an
// equivalent degree-five net. All model and trim poles require exact IGES decimals.
func WriteParabolaCircleObliqueTangencyEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleObliqueTangencyEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	bridge := body.History().Bridge
	prepare := func(ctx context.Context, b trimmed.ExactBezierBoundary, proof edgecheck.Options) error {
		return parabolacircle.PrepareNormalBridge(ctx, b, bridge, proof, exactIGESSplineDecimal)
	}
	return writeExactBezierSingleIGESWithBoundary(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof}, false, prepare)
}

// VerifyParabolaCircleObliqueTangencyEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleObliqueTangencyEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleObliqueTangencyEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "nonapex-tangent parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleObliqueTangencyEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaCircleObliqueTangencyCut transfers the exact positive source layers as
// a group of separate type 186 solids with one exact shared nonapex tangent point.
func WriteParabolaCircleObliqueTangencyCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleObliqueTangencyCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	parts, contact := body.Components(), body.Contact()
	contacts := []trimmed.PolynomialMixedCutContact{{
		TwoPolynomialNodeComponentContact: trimmed.TwoPolynomialNodeComponentContact{
			Parameters: contact.Parameters, Point: contact.Point, Components: [2]int{0, 1}, Vertices: contact.Vertices,
		},
	}}
	return writeClippedCutIGESBodies(ctx, w, parts[:], contacts, unit, l, ClippedPolynomialCutIGESOptions{Proof: proof})
}

// VerifyParabolaCircleObliqueTangencyCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaCircleObliqueTangencyCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleObliqueTangencyCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleObliqueTangencyCut(ctx, w, body, unit, l, proof)
	})
}
