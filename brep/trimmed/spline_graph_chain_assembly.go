package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// Shared exact graph-chain region construction. Callers must independently
// authenticate complete contact and occupancy before entering this assembler.
func assembleOpenGraphChain(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, forward []openSectionPiece, axis, sign int, proof geom.GraphPointContactOptions, evaluate ...graphVertexEvaluator) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	if len(forward) < 2 || len(forward) > 32 || (sign != 1 && sign != -1) {
		return fail(geom.ErrInvalid)
	}
	for i, p := range forward {
		if p.side != -1 || (i > 0 && forward[i-1].b != p.a) {
			return fail(geom.ErrConvergence)
		}
	}
	var backward []openSectionPiece
	for i := len(forward) - 1; i >= 0; i-- {
		p := forward[i]
		backward = append(backward, openSectionPiece{a: p.b, b: p.a, c: reversePCurve(p.c), side: -1})
	}
	start, end := forward[0].a, forward[len(forward)-1].b
	zero, br, tr, tl := geom.Vec2{}, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: 1}, geom.Vec2{Y: 1}
	lower, upper := openGraphRegion{sign: -sign}, openGraphRegion{sign: sign}
	if axis == 0 {
		lower.loop = append([]openSectionPiece{{a: zero, b: br, side: 0}, {a: br, b: end, side: 1}}, backward...)
		lower.loop = append(lower.loop, openSectionPiece{a: start, b: zero, side: 3})
		upper.loop = append(append([]openSectionPiece{}, forward...), openSectionPiece{a: end, b: tr, side: 1}, openSectionPiece{a: tr, b: tl, side: 2}, openSectionPiece{a: tl, b: start, side: 3})
	} else if axis == 1 {
		lower.loop = append([]openSectionPiece{{a: zero, b: start, side: 0}}, forward...)
		lower.loop = append(lower.loop, openSectionPiece{a: end, b: tl, side: 2}, openSectionPiece{a: tl, b: zero, side: 3})
		upper.loop = append([]openSectionPiece{{a: start, b: br, side: 0}, {a: br, b: tr, side: 1}, {a: tr, b: end, side: 2}}, backward...)
	} else {
		return fail(geom.ErrInvalid)
	}
	solid, history, e := combineOpenGraphRegions(ctx, base, a, b, []openGraphRegion{lower, upper}, op, tolerance, evaluate...)
	if e != nil {
		return fail(e)
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, proof); e != nil {
		return fail(e)
	}
	for _, f := range solid.faces {
		if _, ok := f.Surface.(*geom.BSplineSurface); !ok {
			continue
		}
		r, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if r.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	return solid, history, nil
}
