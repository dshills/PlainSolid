package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaCircleCrossingsEnvelope transfers a certified original Common
// or Fuse as one analytic solid with both original transverse junctions.
// All model and trim poles require exact STEP decimals.
func WriteParabolaCircleCrossingsEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleCrossingsEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof})
}

// VerifyParabolaCircleCrossingsEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleCrossingsEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleCrossingsEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "transverse parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleCrossingsEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaCircleCrossingsCut transfers the exact positive source layers as
// a group of two separate solids with two exact shared closure points.
func WriteParabolaCircleCrossingsCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleCrossingsCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
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
	return writeClippedCutSTEPBodies(ctx, w, parts[:], contacts[:], name, unit, l, ClippedPolynomialCutSTEPOptions{Proof: proof})
}

// VerifyParabolaCircleCrossingsCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaCircleCrossingsCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleCrossingsCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleCrossingsCut(ctx, w, body, name, unit, l, proof)
	})
}
