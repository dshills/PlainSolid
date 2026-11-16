package step

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaCircleFourCrossingsEnvelope transfers the original Common or
// Fuse with all four transverse junctions and the coedges on both source
// roofs. Nonterminating poles on a planar circular edge use its certified
// original-quarter support and exact active interval when both original
// UV laws have the same restriction.
func WriteParabolaCircleFourCrossingsEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleFourCrossingsEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEPWithBoundary(ctx, w, body.Boundary(), name, unit, l, ClippedPolynomialFuseSTEPOptions{Proof: proof}, false, nil, emitParabolaCircleSTEPBody([2][]int{body.History().CircleEdges}, true))
}

// VerifyParabolaCircleFourCrossingsEnvelopeSTEP checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleFourCrossingsEnvelopeSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleFourCrossingsEnvelope,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, in, l, "four crossings of one circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleFourCrossingsEnvelope(ctx, w, body, name, unit, l, proof)
	})
}

// WriteParabolaCircleFourCrossingsCut transfers the positive original
// components with four authenticated contacts at separate vertices. Exact
// model and trim coordinates must fit the neutral representation.
func WriteParabolaCircleFourCrossingsCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleFourCrossingsCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	parts, exact := body.Components(), body.Contacts()
	var contacts [4]trimmed.PolynomialMixedCutContact
	for i, node := range exact {
		contacts[i].TwoPolynomialNodeComponentContact = trimmed.TwoPolynomialNodeComponentContact{
			Parameters: node.Parameters, Point: node.Point, Components: node.Components, Vertices: node.Vertices,
		}
	}
	emitters := make([]exactBezierSTEPBodyEmitter, len(parts))
	for i, component := range body.History() {
		emitters[i] = emitParabolaCircleSTEPBody([2][]int{component.CircleEdges}, true)
	}
	return writeClippedCutSTEPBodies(ctx, w, parts, contacts[:], name, unit, l, ClippedPolynomialCutSTEPOptions{Proof: proof}, emitters...)
}

// VerifyParabolaCircleFourCrossingsCutSTEP checks each component's original analytic
// topology and coedges against the native Cut.
func VerifyParabolaCircleFourCrossingsCutSTEP(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleFourCrossingsCut,
	name string, unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleFourCrossingsCut(ctx, w, body, name, unit, l, proof)
	})
}
