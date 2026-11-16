package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaCircleCrossingsEnvelope transfers a certified original Common
// or Fuse as one type 186 solid with both directed original uses of each
// contact edge. All model and trim poles require exact IGES decimals.
func WriteParabolaCircleCrossingsEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleCrossingsEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGES(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof})
}

// VerifyParabolaCircleCrossingsEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleCrossingsEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleCrossingsEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "transverse parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleCrossingsEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaCircleCrossingsCut transfers the exact positive source layers as
// a group of separate type 186 solids with two exact shared closure points.
func WriteParabolaCircleCrossingsCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleCrossingsCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	parts, exact := body.Components(), body.Contacts()
	var contacts [2]trimmed.PolynomialMixedCutContact
	for i, node := range exact {
		contacts[i].TwoPolynomialNodeComponentContact = trimmed.TwoPolynomialNodeComponentContact{
			Parameters: node.Parameters, Point: node.Point, Components: [2]int{0, 1}, Vertices: node.Vertices,
		}
	}
	return writeClippedCutIGESBodies(ctx, w, parts[:], contacts[:], unit, l, ClippedPolynomialCutIGESOptions{Proof: proof})
}

// VerifyParabolaCircleCrossingsCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaCircleCrossingsCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleCrossingsCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleCrossingsCut(ctx, w, body, unit, l, proof)
	})
}
