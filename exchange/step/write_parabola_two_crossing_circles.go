package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaTwoCrossingCirclesEnvelope transfers the original Common or
// Fuse with all four transverse junctions and the coedges on both source
// roofs. Exact model and trim coordinates must fit the neutral representation.
func WriteParabolaTwoCrossingCirclesEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaTwoCrossingCirclesEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof})
}

// VerifyParabolaTwoCrossingCirclesEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaTwoCrossingCirclesEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaTwoCrossingCirclesEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "two crossing circles envelope", func(w io.Writer) error {
		return WriteParabolaTwoCrossingCirclesEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaTwoCrossingCirclesCut transfers three positive original
// components with four authenticated separate vertex occurrences. Exact
// model and trim coordinates must fit the neutral representation.
func WriteParabolaTwoCrossingCirclesCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaTwoCrossingCirclesCut,
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
	return writeClippedCutSTEPBodies(ctx, w, parts[:], contacts, name, unit, l, ClippedPolynomialCutSTEPOptions{Proof: proof})
}

// VerifyParabolaTwoCrossingCirclesCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaTwoCrossingCirclesCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaTwoCrossingCirclesCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaTwoCrossingCirclesCut(ctx, w, body, name, unit, l, proof)
	})
}
