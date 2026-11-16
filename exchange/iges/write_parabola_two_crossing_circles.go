package iges

import (
	"context"
	"io"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteParabolaTwoCrossingCirclesEnvelope transfers the original Common or
// Fuse with all four transverse junctions and the coedges on both source
// roofs. Nonterminating poles on a planar circular edge use its certified
// cardinal-quarter support and exact active interval when both original
// UV laws have the same restriction.
func WriteParabolaTwoCrossingCirclesEnvelope(ctx context.Context, w io.Writer, body *trimmed.ParabolaTwoCrossingCirclesEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGESWithBoundary(ctx, w, body.Boundary(), unit, l, ClippedPolynomialFuseIGESOptions{Proof: proof}, false, nil, emitTwoCrossingCirclesIGESBody(ctx, body.History().CircleEdges))
}

// VerifyParabolaTwoCrossingCirclesEnvelopeIGES checks analytic topology, units and
// original coedge ownership against the native envelope.
func VerifyParabolaTwoCrossingCirclesEnvelopeIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaTwoCrossingCirclesEnvelope,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, in, l, "two crossing circles envelope", func(w io.Writer) error {
		return WriteParabolaTwoCrossingCirclesEnvelope(ctx, w, body, unit, l, proof)
	})
}

// WriteParabolaTwoCrossingCirclesCut transfers three positive original
// components with four authenticated separate vertex occurrences. Exact
// model and trim coordinates must fit the neutral representation.
func WriteParabolaTwoCrossingCirclesCut(ctx context.Context, w io.Writer, body *trimmed.ParabolaTwoCrossingCirclesCut,
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
	var emitters [3]exactBezierIGESBodyEmitter
	for i, component := range body.History() {
		emitters[i] = emitTwoCrossingCirclesIGESBody(ctx, component.CircleEdges)
	}
	return writeClippedCutIGESBodies(ctx, w, parts[:], contacts, unit, l, ClippedPolynomialCutIGESOptions{Proof: proof}, emitters[:]...)
}

// VerifyParabolaTwoCrossingCirclesCutIGES checks the original analytic topology
// and coedges of all positive source components.
func VerifyParabolaTwoCrossingCirclesCutIGES(ctx context.Context, in io.Reader, body *trimmed.ParabolaTwoCrossingCirclesCut,
	unit float64, l Limits, proof edgecheck.Options) error {
	if body == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutIGESBodies(ctx, in, l, func(w io.Writer) error {
		return WriteParabolaTwoCrossingCirclesCut(ctx, w, body, unit, l, proof)
	})
}
