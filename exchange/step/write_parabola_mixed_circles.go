package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaMixedCirclesEnvelope transfers a certified original Common
// or Fuse as one analytic solid with both original transverse junctions and
// the separated circular seam. All model and trim poles require exact STEP decimals.
func WriteParabolaMixedCirclesEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaMixedCirclesEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof})
}

// VerifyParabolaMixedCirclesEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaMixedCirclesEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaMixedCirclesEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "crossing and separated parabola-circle envelope", func(w io.Writer) error {
		return WriteParabolaMixedCirclesEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaMixedCirclesCut transfers the exact positive source layers as
// two or three separate solids with two original shared closure points.
func WriteParabolaMixedCirclesCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaMixedCirclesCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
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
	return writeClippedCutSTEPBodies(ctx, w, parts, contacts[:], name, unit, l, ClippedPolynomialCutSTEPOptions{Proof: proof})
}

// VerifyParabolaMixedCirclesCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaMixedCirclesCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaMixedCirclesCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaMixedCirclesCut(ctx, w, body, name, unit, l, proof)
	})
}
