package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func assembleParabolicGraphContact(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, section geom.SplineGraphParabola, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	if !section.Proven || section.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || section.TerminalMasks != ([2]uint8{1, 2}) {
		return fail(geom.ErrConvergence)
	}
	var h [3][3]*big.Rat
	for i, c := range section.Controls {
		h[i] = [3]*big.Rat{new(big.Rat).Set(c[0]), new(big.Rat).Set(c[1]), big.NewRat(1, 1)}
	}
	curve, e := newExactQuadraticUV(h, false)
	if e != nil {
		return fail(e)
	}
	left, e := exactGraphPartitionUV(section.Controls[0])
	if e != nil {
		return fail(e)
	}
	right, e := exactGraphPartitionUV(section.Controls[2])
	if e != nil {
		return fail(e)
	}
	zero := geom.Vec2{}
	br := geom.Vec2{X: 1}
	tr := geom.Vec2{X: 1, Y: 1}
	tl := geom.Vec2{Y: 1}
	lower := openGraphRegion{sign: -section.PositiveSide, loop: []openSectionPiece{{a: zero, b: br, side: 0}, {a: br, b: right, side: 1}, {a: right, b: left, c: reversePCurve(curve), side: -1}, {a: left, b: zero, side: 3}}}
	upper := openGraphRegion{sign: section.PositiveSide, loop: []openSectionPiece{{a: left, b: right, c: curve, side: -1}, {a: right, b: tr, side: 1}, {a: tr, b: tl, side: 2}, {a: tl, b: left, side: 3}}}
	solid, history, e := combineOpenGraphRegions(ctx, base, a, b, []openGraphRegion{lower, upper}, op, tolerance)
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
		q, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if q.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	history.Parabola = &section
	return solid, history, nil
}
