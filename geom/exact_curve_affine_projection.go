package geom

import (
	"context"
	"math/big"
)

// CurveProjectionParameterMap is the exact relation q=Offset+Scale*t from an
// original curve parameter t to the selected model-coordinate projection q.
type CurveProjectionParameterMap struct{ Offset, Scale *big.Rat }

// AffineProjectionCurveHit carries separate ascending original parameter
// brackets. Parameters are nil for unrecovered algebraic roots. Evidence is the
// exact common projected-coordinate root and its one-sided span diagnostics.
type AffineProjectionCurveHit struct {
	Lower, Upper, Parameters [2]*big.Rat
	Evidence                 SharedProjectionCurveHit
	ExactProjectionSpan      [2]*big.Rat
}

// AffineProjectionCurveCoincidence maps a whole common projected-coordinate
// interval back to both original curves. Negative scales reverse correspondence.
type AffineProjectionCurveCoincidence struct {
	Lower, Upper        [2]*big.Rat
	Projection          ParameterSpan
	ExactProjectionSpan [2]*big.Rat
}

// AffineProjectionCurveIntersection permits independent affine projection
// speeds and orientations. All rational outputs are caller-owned. Complete is
// exhaustive within this proved subdomain, including coincident intervals.
type AffineProjectionCurveIntersection struct {
	Maps                     [2]CurveProjectionParameterMap
	Hits                     []AffineProjectionCurveHit
	Coincident               []AffineProjectionCurveCoincidence
	Complete                 bool
	Operations, Subdivisions int
}

// IntersectExactCurvesAffineProjection intersects spatial exact curves whose
// selected projection is nonconstant affine in each independent parameter.
// Each support is reparameterized losslessly to the common projection q, then
// all contact follows from exact homogeneous polynomial GCDs. Different domains,
// directions, degrees, positive weights and knot partitions are permitted.
// Projected knots and interval maps remain rational, even when not representable
// as floats. Nonaffine projection returns ErrConvergence. ExactProjectionSpan
// retains each rational span; Evidence.Span and Projection are outward float
// display bounds only (not an exact partition when knots are nonbinary).
func IntersectExactCurvesAffineProjection(ctx context.Context, a, c *ExactSplineCurve, covector Vec3, o ExactCurveSegmentOptions) (AffineProjectionCurveIntersection, error) {
	return intersectExactCurvesRationalProjection(ctx, a, c, covector, o)
}
