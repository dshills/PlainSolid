package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func assemblePolynomialNodeGraphBoolean(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, node geom.SplineGraphPolynomialNode, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}, e
	}
	if !node.Proven || node.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || node.Graph.ProjectionOrientation != 1 {
		return fail(geom.ErrConvergence)
	}
	if op == parametric.Difference {
		return fail(fmt.Errorf("trimmed: polynomial node Cut has opposite material sectors meeting at the node: %w", geom.ErrDegenerate))
	}
	loops, e := polynomialNodeSectorLoops(node)
	if e != nil {
		return fail(e)
	}
	regions := make([]graphEnvelopeRegion, len(loops))
	for i, loop := range loops {
		regions[i] = graphEnvelopeRegion{loop: loop, first: (node.Sectors[i].HeightSide > 0) == (op == parametric.Union)}
	}
	solid, history, e := assembleSplineEnvelope(ctx, base, a, b, regions, tolerance, exactGraphVertexEvaluator(ctx, proof))
	if e != nil {
		return fail(e)
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, proof); e != nil {
		return fail(e)
	}
	for i := range regions {
		r, e := ValidateFaceTrimLoops(ctx, solid.faces[i], trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if r.Status != trimvalidate.Certified {
			return fail(fmt.Errorf("trimmed: polynomial node sector %d: %s: %w", i, r.Reason, geom.ErrConvergence))
		}
	}
	history.PolynomialNode = &node
	return solid, history, nil
}

func polynomialNodeSectorLoops(node geom.SplineGraphPolynomialNode) ([4][]openSectionPiece, error) {
	var loops [4][]openSectionPiece
	center, e := exactGraphPartitionUV(node.Parameters)
	if e != nil {
		return loops, e
	}
	if _, e = geom.RepresentedExactPoint(node.Point); e != nil {
		return loops, e
	}
	var rays [4]openSectionPiece
	for k, controls := range node.Controls {
		h := make([][3]*big.Rat, len(controls))
		for i, c := range controls {
			h[i] = [3]*big.Rat{c[0], c[1], big.NewRat(1, 1)}
		}
		curve, e := NewExactBezierUV(h)
		if e != nil {
			return loops, e
		}
		// The free coordinate of each exact Bézier trim is affine on [0,1].
		left, right, e := curve.Split(node.Parameters[node.FreeAxis])
		if e != nil {
			return loops, e
		}
		if left.ends[1] != center || right.ends[0] != center {
			return loops, geom.ErrConvergence
		}
		rays[2*k] = openSectionPiece{a: center, b: left.ends[0], c: left.reversed(), side: -1}
		rays[2*k+1] = openSectionPiece{a: center, b: right.ends[1], c: right, side: -1}
	}
	for i, sector := range node.Sectors {
		boundary := sector.PositiveBoundary
		if boundary.ParameterDirection != 1 {
			return loops, geom.ErrConvergence
		}
		start, end := rays[boundary.StartRay], rays[boundary.EndRay]
		arc, e := saddleSquareArc(start.b, end.b)
		if e != nil {
			return loops, e
		}
		loop := append([]openSectionPiece{start}, arc...)
		loops[i] = append(loop, openSectionPiece{a: end.b, b: center, c: reversePCurve(end.c), side: -1})
	}
	return loops, nil
}
