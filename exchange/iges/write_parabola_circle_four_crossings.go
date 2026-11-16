package iges

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
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGESWithBoundary(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof}, false, nil, emitParabolaCircleIGESBody(ctx, [2][]int{body.History().CircleEdges}, true))
}

// VerifyParabolaCircleFourCrossingsEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaCircleFourCrossingsEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleFourCrossingsEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "four crossings of one circle envelope", func(w io.Writer) error {
		return WriteParabolaCircleFourCrossingsEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaCircleFourCrossingsCut transfers the positive original
// components with four authenticated contacts at separate vertices. Exact
// model and trim coordinates must fit the neutral representation.
func WriteParabolaCircleFourCrossingsCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaCircleFourCrossingsCut,
	unit float64, l Limits, proof edgecheck.Options) error {
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
	emitters := make([]exactBezierIGESBodyEmitter, len(parts))
	for i, component := range body.History() {
		emitters[i] = emitParabolaCircleIGESBody(ctx, [2][]int{component.CircleEdges}, true)
	}
	return writeClippedCutIGESBodies(ctx, w, parts, contacts[:], unit, l, ClippedPolynomialCutIGESOptions{Proof: proof}, emitters...)
}

// VerifyParabolaCircleFourCrossingsCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaCircleFourCrossingsCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaCircleFourCrossingsCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaCircleFourCrossingsCut(ctx, w, body, unit, l, proof)
	})
}
