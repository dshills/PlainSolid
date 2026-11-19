//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
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
	var forward []openSectionPiece
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
	solid, history, e := assembleOpenGraphChain(ctx, base, a, b, op, tolerance, forward, section.FreeAxis, section.PositiveSide, proof)
	if e != nil {
		return fail(e)
	}
	history.PiecewiseParabola = &section
	return solid, history, nil
}
