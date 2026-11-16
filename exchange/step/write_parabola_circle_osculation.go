package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/internal/parabolacircle"
	"plainsolid/geom"
)

// WriteParabolaCircleOsculationEnvelope transfers a certified original Common
// or Fuse as one analytic solid with the original fourth-order junction,
// parabola subdivisions and roof bridge. When the chart is transposed
// and the disk is above the parabola, its parabolic rays use an equivalent
// rational clock. All model and trim poles require exact STEP decimals.
func WriteParabolaCircleOsculationEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleOsculationEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	history := body.History()
	prepare := func(ctx context.Context, b trimmed.ExactBezierBoundary, proof edgecheck.Options) error {
		return parabolacircle.PrepareOsculationParabolaRays(ctx, b, history.Bridge, history.Vertex, history.Rays[0], proof)
	}
	return writeExactBezierSingleSTEPWithBoundary(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof}, false, prepare)
}

// VerifyParabolaCircleOsculationEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleOsculationEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleOsculationEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "osculating parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleOsculationEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaCircleOsculationCut transfers the exact positive source layers as
// a group of two separate solids with one exact shared fourth-order contact point.
func WriteParabolaCircleOsculationCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleOsculationCut,
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

// VerifyParabolaCircleOsculationCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaCircleOsculationCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleOsculationCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleOsculationCut(ctx, w, body, name, unit, l, proof)
	})
}
