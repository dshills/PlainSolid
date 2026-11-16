package step

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
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof})
}

// VerifyParabolaThreeCirclesEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaThreeCirclesEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaThreeCirclesEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "three-circle parabola envelope", func(w io.Writer) error {
		return WriteParabolaThreeCirclesEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaThreeCirclesCut transfers the original positive layers
// with four independent contact occurrences at the crossing nodes. The
// separated ring retains its original hole or isolated component.
func WriteParabolaThreeCirclesCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaThreeCirclesCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
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
	return writeClippedCutSTEPBodies(ctx, w, parts, contacts, name, unit, l, ClippedPolynomialCutSTEPOptions{Proof: proof})
}

// VerifyParabolaThreeCirclesCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaThreeCirclesCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaThreeCirclesCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaThreeCirclesCut(ctx, w, body, name, unit, l, proof)
	})
}
