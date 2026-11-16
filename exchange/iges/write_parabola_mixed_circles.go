package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaMixedCirclesEnvelope transfers a certified original Common
// or Fuse as one type 186 solid with both original uses of each crossing
// and separated circle edge. All model and trim poles require exact IGES decimals.
func WriteParabolaMixedCirclesEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaMixedCirclesEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGES(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof})
}

// VerifyParabolaMixedCirclesEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaMixedCirclesEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaMixedCirclesEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "crossing and separated parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaMixedCirclesEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaMixedCirclesCut transfers the exact positive source layers as
// two or three type 186 solids with two exact shared closure points.
func WriteParabolaMixedCirclesCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaMixedCirclesCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	parts, exact := body.Components(), body.Contacts()
	var contacts [2]trimmed.PolynomialMixedCutContact
	for i, node := range exact {
		contacts[i].TwoPolynomialNodeComponentContact = trimmed.TwoPolynomialNodeComponentContact{
			Parameters: node.Parameters, Point: node.Point, Components: node.Components, Vertices: node.Vertices,
		}
	}
	return writeClippedCutIGESBodies(ctx, w, parts, contacts[:], unit, l, ClippedPolynomialCutIGESOptions{Proof: proof})
}

// VerifyParabolaMixedCirclesCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaMixedCirclesCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaMixedCirclesCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaMixedCirclesCut(ctx, w, body, unit, l, proof)
	})
}
