package geom

import (
	"context"
	"math/big"
)

// Use an exactly represented index as the working knot for each piece. The
// physical parameter map is stored in spans, so even a nonbinary lattice or
// a window in a distant cycle retains its original exact parameter.
func (c *PeriodicBSpline) clampedPeriodicWindow(ctx context.Context, start, end *big.Rat, o ExactBezierSpanOptions) (*ExactSplineCurve, []ExactPeriodicBezierSpan, int, error) {
	limits, err := patchOptions(o.SurfacePatchOptions)
	if err != nil {
		return nil, nil, 0, err
	}
	spans, work, err := c.ExactBezierSpans(ctx, start, end, o)
	if err != nil {
		return nil, nil, work, err
	}
	limits.MaxOperations -= work
	degree := c.degree
	if limits.MaxOperations < 1 || len(spans) > (limits.MaxPoles/2-1)/degree {
		return nil, nil, work, ErrConvergence
	}
	count := degree*len(spans) + 1
	controls := make([][4]*big.Rat, 0, count)
	knots := make([]float64, 0, count+degree+1)
	knots = append(knots, 0)
	controls = append(controls, spans[0].Control[0])
	for i, piece := range spans {
		if err := ctx.Err(); err != nil {
			return nil, nil, work, err
		}
		controls = append(controls, piece.Control[1:]...)
		for j := 0; j < degree; j++ {
			knots = append(knots, float64(i))
		}
	}
	for j := 0; j <= degree; j++ {
		knots = append(knots, float64(len(spans)))
	}
	curve, err := NewExactHomogeneousSpline(ctx, degree, controls, knots, limits)
	if err != nil {
		return nil, nil, work, err
	}
	return curve, spans, work + curve.ExactOperations(), nil
}

func periodicWindowParameter(piece ExactPeriodicBezierSpan, index int, parameter float64) *big.Rat {
	local := new(big.Rat).SetFloat64(parameter - float64(index))
	width := new(big.Rat).Sub(piece.End, piece.Start)
	return new(big.Rat).Add(piece.Start, new(big.Rat).Mul(width, local))
}
