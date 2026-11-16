package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/parabolacircle"
	"plainsolid/geom"
)

// WriteParabolaCircleOsculationEnvelope transfers a certified original Common
// or Fuse as one type 186 solid with the original fourth-order contact
// edges, subdivision vertices and same-roof bridge. When the chart is
// transposed and the disk is above the parabola, its parabolic rays use
// an equivalent rational clock. All poles require exact IGES decimals.
func WriteParabolaCircleOsculationEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleOsculationEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	history := body.History()
	prepare := func(ctx context.Context, b trimmed.ExactBezierBoundary, proof edgecheck.Options) error {
		return parabolacircle.PrepareOsculationParabolaRays(ctx, b, history.Bridge, history.Vertex, history.Rays[0], proof)
	}
	return writeExactBezierSingleIGESWithBoundary(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof}, false, prepare)
}

// VerifyParabolaCircleOsculationEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleOsculationEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleOsculationEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "osculating parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleOsculationEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaCircleOsculationCut transfers the exact positive source layers as
// a group of separate type 186 solids with one exact shared fourth-order contact point.
func WriteParabolaCircleOsculationCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleOsculationCut,
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

// VerifyParabolaCircleOsculationCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaCircleOsculationCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleOsculationCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleOsculationCut(ctx, w, body, unit, l, proof)
	})
}
