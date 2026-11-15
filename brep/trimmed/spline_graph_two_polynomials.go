package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func assembleTwoPolynomialGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, section geom.SplineGraphTwoPolynomials, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	if !section.Proven || section.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return fail(geom.ErrConvergence)
	}
	var curves [2]PCurve
	var starts, ends [2]geom.Vec2
	for k, controls := range section.Controls {
		h := make([][3]*big.Rat, len(controls))
		for i, c := range controls {
			h[i] = [3]*big.Rat{c[0], c[1], big.NewRat(1, 1)}
		}
		curve, e := NewExactBezierUV(h)
		if e != nil {
			return fail(e)
		}
		curves[k] = curve
		starts[k], e = exactGraphPartitionUV(controls[0])
		if e != nil {
			return fail(e)
		}
		ends[k], e = exactGraphPartitionUV(controls[len(controls)-1])
		if e != nil {
			return fail(e)
		}
	}
	zero, br, tr, tl := geom.Vec2{}, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: 1}, geom.Vec2{Y: 1}
	f := func(k int) openSectionPiece {
		return openSectionPiece{a: starts[k], b: ends[k], c: curves[k], side: -1}
	}
	back := func(k int) openSectionPiece {
		return openSectionPiece{a: ends[k], b: starts[k], c: reversePCurve(curves[k]), side: -1}
	}
	var loops [3][]openSectionPiece
	if section.FreeAxis == 0 {
		loops[0] = []openSectionPiece{{a: zero, b: br, side: 0}, {a: br, b: ends[0], side: 1}, back(0), {a: starts[0], b: zero, side: 3}}
		loops[1] = []openSectionPiece{f(0), {a: ends[0], b: ends[1], side: 1}, back(1), {a: starts[1], b: starts[0], side: 3}}
		loops[2] = []openSectionPiece{f(1), {a: ends[1], b: tr, side: 1}, {a: tr, b: tl, side: 2}, {a: tl, b: starts[1], side: 3}}
	} else if section.FreeAxis == 1 {
		loops[0] = []openSectionPiece{{a: zero, b: starts[0], side: 0}, f(0), {a: ends[0], b: tl, side: 2}, {a: tl, b: zero, side: 3}}
		loops[1] = []openSectionPiece{{a: starts[0], b: starts[1], side: 0}, f(1), {a: ends[1], b: ends[0], side: 2}, back(0)}
		loops[2] = []openSectionPiece{{a: starts[1], b: br, side: 0}, {a: br, b: tr, side: 1}, {a: tr, b: ends[1], side: 2}, back(1)}
	} else {
		return fail(geom.ErrInvalid)
	}
	// The middle Cut has two original side-wall arcs sharing both end
	// vertices. Split each straight original UV side at its exact midpoint so
	// each native wall has a noncollapsed vertical edge and at least three uses.
	// Strict ordering proves nonzero height throughout these open side intervals.
	if op == parametric.Difference && section.StripSigns[1] > 0 {
		var split []openSectionPiece
		for _, p := range loops[1] {
			if p.side < 0 {
				split = append(split, p)
				continue
			}
			halves, e := splitGraphSideAtMidpoint(p)
			if e != nil {
				return fail(e)
			}
			split = append(split, halves[:]...)
		}
		loops[1] = split
	}
	regions := make([]openGraphRegion, 3)
	for i := range regions {
		regions[i] = openGraphRegion{loop: loops[i], sign: section.StripSigns[i]}
	}
	solid, history, e := combineOpenGraphRegions(ctx, base, a, b, regions, op, tolerance, exactGraphVertexEvaluator(ctx, proof))
	if e != nil {
		return fail(e)
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, proof); e != nil {
		return fail(e)
	}
	for _, face := range solid.faces {
		if _, ok := face.Surface.(*geom.BSplineSurface); !ok {
			continue
		}
		q, e := ValidateFaceTrimLoops(ctx, face, trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if q.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	history.TwoPolynomials = &section
	return solid, history, nil
}

func splitGraphSideAtMidpoint(p openSectionPiece) ([2]openSectionPiece, error) {
	var halves [2]openSectionPiece
	xy := [2]*big.Rat{}
	for i, pair := range [2][2]float64{{p.a.X, p.b.X}, {p.a.Y, p.b.Y}} {
		xy[i] = new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(pair[0]), new(big.Rat).SetFloat64(pair[1])), big.NewRat(2, 1))
	}
	mid, e := exactGraphPartitionUV(xy)
	if e != nil {
		return halves, e
	}
	return [2]openSectionPiece{{a: p.a, b: mid, side: p.side}, {a: mid, b: p.b, side: p.side}}, nil
}
