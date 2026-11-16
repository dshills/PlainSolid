package geom

import (
	"context"
	"math/big"
)

// RationalSplineCurveJet retains an exact point and physical derivative in
// the curve's original parameter, with outward binary64 boxes. Returned
// coordinates are independent of the source and of one another.
type RationalSplineCurveJet struct {
	Point, Derivative             [3]*big.Rat
	PointBounds, DerivativeBounds Box
	ExactOperations               int
}

// EvaluateRational evaluates an exact spline or isoparametric curve without
// rounding its parameter, point, or derivative. At an interior original
// knot it uses the right span; at the final station it uses the left span.
func (c *SurfaceIsoCurve) EvaluateRational(ctx context.Context, parameter *big.Rat, o SurfacePatchEvaluationOptions) (RationalSplineCurveJet, error) {
	if c == nil || c.patch == nil || parameter == nil {
		return RationalSplineCurveJet{}, ErrInvalid
	}
	q, err := c.patch.EvaluateRational(ctx, parameter, new(big.Rat), o)
	return RationalSplineCurveJet{Point: q.Point, Derivative: q.Du, PointBounds: q.PointBounds,
		DerivativeBounds: q.DuBounds, ExactOperations: q.ExactOperations}, err
}
