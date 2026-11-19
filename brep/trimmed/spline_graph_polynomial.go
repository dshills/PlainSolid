//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func assemblePolynomialGraphContact(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, section geom.SplineGraphPolynomialContact, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	if !section.Proven || section.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || (section.FreeAxis != 0 && section.FreeAxis != 1) || (section.FreeAxis == 0 && section.TerminalMasks != ([2]uint8{1, 2})) || (section.FreeAxis == 1 && section.TerminalMasks != ([2]uint8{4, 8})) {
		return fail(geom.ErrConvergence)
	}
	h := make([][3]*big.Rat, len(section.Controls))
	for i, c := range section.Controls {
		h[i] = [3]*big.Rat{new(big.Rat).Set(c[0]), new(big.Rat).Set(c[1]), big.NewRat(1, 1)}
	}
	curve, e := NewExactBezierUV(h)
	if e != nil {
		return fail(e)
	}
	left, e := exactGraphPartitionUV(section.Controls[0])
	if e != nil {
		return fail(e)
	}
	right, e := exactGraphPartitionUV(section.Controls[len(section.Controls)-1])
	if e != nil {
		return fail(e)
	}
	zero := geom.Vec2{}
	br := geom.Vec2{X: 1}
	tr := geom.Vec2{X: 1, Y: 1}
	tl := geom.Vec2{Y: 1}
	lower := openGraphRegion{sign: -section.PositiveSide, loop: []openSectionPiece{{a: zero, b: br, side: 0}, {a: br, b: right, side: 1}, {a: right, b: left, c: reversePCurve(curve), side: -1}, {a: left, b: zero, side: 3}}}
	upper := openGraphRegion{sign: section.PositiveSide, loop: []openSectionPiece{{a: left, b: right, c: curve, side: -1}, {a: right, b: tr, side: 1}, {a: tr, b: tl, side: 2}, {a: tl, b: left, side: 3}}}
	if section.FreeAxis == 1 {
		// Left and right regions retain CCW original-UV orientation.
		lower.loop = []openSectionPiece{{a: zero, b: left, side: 0}, {a: left, b: right, c: curve, side: -1}, {a: right, b: tl, side: 2}, {a: tl, b: zero, side: 3}}
		upper.loop = []openSectionPiece{{a: left, b: br, side: 0}, {a: br, b: tr, side: 1}, {a: tr, b: right, side: 2}, {a: right, b: left, c: reversePCurve(curve), side: -1}}
	}
	solid, history, e := combineOpenGraphRegions(ctx, base, a, b, []openGraphRegion{lower, upper}, op, tolerance, exactGraphVertexEvaluator(ctx, proof))
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
	history.Polynomial = &section
	return solid, history, nil
}
