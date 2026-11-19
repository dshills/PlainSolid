//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func assemblePiecewisePolynomialGraphContact(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, section geom.PiecewiseSplineGraphPolynomial, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{}, e
	}
	if !section.Proven || section.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || len(section.Pieces) < 2 || len(section.Joins) != len(section.Pieces)-1 {
		return fail(geom.ErrConvergence)
	}
	var forward []openSectionPiece
	for _, piece := range section.Pieces {
		controls := piece.Controls()
		h := make([][3]*big.Rat, len(controls))
		for i, c := range controls {
			h[i] = [3]*big.Rat{c[0], c[1], big.NewRat(1, 1)}
		}
		curve, e := NewExactBezierUV(h)
		if e != nil {
			return fail(e)
		}
		start, e := exactGraphPartitionUV(controls[0])
		if e != nil {
			return fail(e)
		}
		end, e := exactGraphPartitionUV(controls[len(controls)-1])
		if e != nil {
			return fail(e)
		}
		forward = append(forward, openSectionPiece{a: start, b: end, c: curve, side: -1})
	}
	solid, history, e := assembleOpenGraphChain(ctx, base, a, b, op, tolerance, forward, section.FreeAxis, section.PositiveSide, proof, exactGraphVertexEvaluator(ctx, proof))
	if e != nil {
		return fail(e)
	}
	history.PiecewisePolynomial = &section
	return solid, history, nil
}
