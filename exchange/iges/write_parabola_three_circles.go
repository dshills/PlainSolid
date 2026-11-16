package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaThreeCirclesEnvelope transfers an original Common or Fuse
// with four transverse junctions and a separated analytic circle seam.
// Both original UV coedges must have exact portable representations.
func WriteParabolaThreeCirclesEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaThreeCirclesEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGES(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof})
}

// VerifyParabolaThreeCirclesEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaThreeCirclesEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaThreeCirclesEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "three-circle parabola envelope", func(w io.Writer) error {
		return WriteParabolaThreeCirclesEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaThreeCirclesCut transfers the original positive layers
// with four independent contact occurrences at the crossing nodes. The
// separated ring retains its original hole or isolated component.
func WriteParabolaThreeCirclesCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaThreeCirclesCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	parts, exact := body.Components(), body.Contacts()
	contacts := make([]trimmed.PolynomialMixedCutContact, 0, 4)
	for _, circle := range exact {
		for _, node := range circle {
			contacts = append(contacts, trimmed.PolynomialMixedCutContact{
				TwoPolynomialNodeComponentContact: trimmed.TwoPolynomialNodeComponentContact{
					Parameters: node.Parameters, Point: node.Point, Components: node.Components, Vertices: node.Vertices,
				},
			})
		}
	}
	return writeClippedCutIGESBodies(ctx, w, parts, contacts, unit, l, ClippedPolynomialCutIGESOptions{Proof: proof})
}

// VerifyParabolaThreeCirclesCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaThreeCirclesCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaThreeCirclesCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaThreeCirclesCut(ctx, w, body, unit, l, proof)
	})
}
