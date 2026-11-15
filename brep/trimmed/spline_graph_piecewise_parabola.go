package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// assemblePiecewiseParabolicGraphContact consumes only an internally discovered
// complete report. Each original slab becomes a separate authored pcurve; shared
// endpoints must be losslessly native-representable. No smoothing joins or
// approximate seam splitting is performed.
func assemblePiecewiseParabolicGraphContact(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, section geom.PiecewiseSplineGraphParabola, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	if !section.Proven || section.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || len(section.Pieces) < 2 || len(section.Joins) != len(section.Pieces)-1 {
		return fail(geom.ErrConvergence)
	}
	var forward, backward []openSectionPiece
	for _, piece := range section.Pieces {
		var h [3][3]*big.Rat
		for i, c := range piece.Controls {
			h[i] = [3]*big.Rat{new(big.Rat).Set(c[0]), new(big.Rat).Set(c[1]), big.NewRat(1, 1)}
		}
		curve, e := newExactQuadraticUV(h, false)
		if e != nil {
			return fail(e)
		}
		start, e := exactGraphPartitionUV(piece.Controls[0])
		if e != nil {
			return fail(e)
		}
		end, e := exactGraphPartitionUV(piece.Controls[2])
		if e != nil {
			return fail(e)
		}
		if len(forward) > 0 && forward[len(forward)-1].b != start {
			return fail(geom.ErrConvergence)
		}
		forward = append(forward, openSectionPiece{a: start, b: end, c: curve, side: -1})
	}
	for i := len(forward) - 1; i >= 0; i-- {
		p := forward[i]
		backward = append(backward, openSectionPiece{a: p.b, b: p.a, c: reversePCurve(p.c), side: -1})
	}
	start, end := forward[0].a, forward[len(forward)-1].b
	zero, br, tr, tl := geom.Vec2{}, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: 1}, geom.Vec2{Y: 1}
	lower, upper := openGraphRegion{sign: -section.PositiveSide}, openGraphRegion{sign: section.PositiveSide}
	if section.FreeAxis == 0 {
		lower.loop = append([]openSectionPiece{{a: zero, b: br, side: 0}, {a: br, b: end, side: 1}}, backward...)
		lower.loop = append(lower.loop, openSectionPiece{a: start, b: zero, side: 3})
		upper.loop = append(append([]openSectionPiece{}, forward...), openSectionPiece{a: end, b: tr, side: 1}, openSectionPiece{a: tr, b: tl, side: 2}, openSectionPiece{a: tl, b: start, side: 3})
	} else if section.FreeAxis == 1 {
		lower.loop = append([]openSectionPiece{{a: zero, b: start, side: 0}}, forward...)
		lower.loop = append(lower.loop, openSectionPiece{a: end, b: tl, side: 2}, openSectionPiece{a: tl, b: zero, side: 3})
		upper.loop = append([]openSectionPiece{{a: start, b: br, side: 0}, {a: br, b: tr, side: 1}, {a: tr, b: end, side: 2}}, backward...)
	} else {
		return fail(geom.ErrInvalid)
	}
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
		r, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if r.Status != trimvalidate.Certified {
			return fail(geom.ErrConvergence)
		}
	}
	history.PiecewiseParabola = &section
	return solid, history, nil
}
