package geom

import (
	"context"
	"math"
	"math/big"
)

// SurfaceIsoCurve is an immutable rational isoparametric curve extracted without
// rounding homogeneous poles or weights. Its parameter is the unfixed original
// surface coordinate. It retains all knots in that coordinate. It is not a
// float-control BSpline approximation.
type SurfaceIsoCurve struct {
	patch          *SurfacePatch
	fixedAxis      int
	fixedParameter float64
	operations     int
}

// IsoCurve fixes original parameter axis 0 (U) or 1 (V), exactly at the supplied
// binary rational value. Endpoints and interior knots are supported. Continuity
// of the source makes the fixed-coordinate side choice immaterial for position
// and for derivatives in the unfixed coordinate. Options bound construction and
// subsequent per-call evaluation. The source remains unchanged on failure.
func (p *SurfacePatch) IsoCurve(fixedAxis int, parameter float64, options SurfacePatchOptions) (*SurfaceIsoCurve, error) {
	o, e := patchOptions(options)
	if e != nil {
		return nil, e
	}
	if p == nil || len(p.net) == 0 || fixedAxis < 0 || fixedAxis > 1 {
		return nil, ErrInvalid
	}
	d := p.Domain()
	lo, hi := d.UMin, d.UMax
	if fixedAxis == 1 {
		lo, hi = d.VMin, d.VMax
	}
	if !finite(parameter) || parameter < lo || parameter > hi {
		return nil, ErrDomain
	}
	if len(p.net) > o.MaxPoles {
		return nil, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	copyPatch := *p
	copyPatch.options = o
	current := &copyPatch
	last := parameter == hi
	if parameter > lo && parameter < hi {
		left, _, e := current.split(parameter, fixedAxis == 0, budget)
		if e != nil {
			return nil, e
		}
		current = left
		last = true
	}
	degree, count, knots := current.degreeV, current.countV, current.knotsV
	if fixedAxis == 1 {
		degree, count, knots = current.degreeU, current.countU, current.knotsU
	}
	if count > o.MaxPoles/2 {
		return nil, ErrConvergence
	}
	net := make([]exactHomogeneous, 2*count)
	for i := 0; i < count; i++ {
		index := i
		if fixedAxis == 0 {
			if last {
				index += (current.countU - 1) * current.countV
			}
		} else {
			index = i * current.countV
			if last {
				index += current.countV - 1
			}
		}
		for axis := 0; axis < 4; axis++ {
			value := current.net[index][axis]
			if e := budget.check(value); e != nil {
				return nil, e
			}
			net[2*i][axis] = new(big.Rat).Set(value)
			net[2*i+1][axis] = new(big.Rat).Set(value)
		}
	}
	curve := &SurfacePatch{degreeU: degree, degreeV: 1, countU: count, countV: 2, knotsU: append([]float64(nil), knots...), knotsV: []float64{0, 0, 1, 1}, net: net, options: o}
	if e := curve.computeBounds(budget); e != nil {
		return nil, e
	}
	curve.operations = budget.operations
	return &SurfaceIsoCurve{curve, fixedAxis, parameter, budget.operations}, nil
}

// Domain returns the original unfixed-coordinate interval, or NaNs for nil.
func (c *SurfaceIsoCurve) Domain() (float64, float64) {
	if c == nil || c.patch == nil {
		return math.NaN(), math.NaN()
	}
	d := c.patch.Domain()
	return d.UMin, d.UMax
}

// FixedParameter identifies the original fixed axis and represented value.
func (c *SurfaceIsoCurve) FixedParameter() (int, float64) {
	if c == nil || c.patch == nil {
		return -1, math.NaN()
	}
	return c.fixedAxis, c.fixedParameter
}

// ExactOperations reports extraction work, excluding later evaluations.
func (c *SurfaceIsoCurve) ExactOperations() int {
	if c == nil {
		return 0
	}
	return c.operations
}

// Eval rounds only the final exact rational point to nearest float64.
func (c *SurfaceIsoCurve) Eval(t float64) (Vec3, error) {
	if c == nil || c.patch == nil {
		return Vec3{}, ErrInvalid
	}
	return c.patch.Eval(t, 0)
}

// Derivative returns the derivative in the original unfixed parameter, with
// right-sided interior-knot and left-sided final endpoint semantics.
func (c *SurfaceIsoCurve) Derivative(t float64) (Vec3, error) {
	if c == nil || c.patch == nil {
		return Vec3{}, ErrInvalid
	}
	u, _, e := c.patch.Partials(t, 0)
	return u, e
}

// SurfaceIsoCurveJet includes numerical values and rigorous component boxes.
type SurfaceIsoCurveJet struct {
	Point, Derivative             Vec3
	PointBounds, DerivativeBounds Box
	ExactOperations               int
}

// EvaluateJet evaluates the exact extracted curve and its parameter derivative.
func (c *SurfaceIsoCurve) EvaluateJet(t float64) (SurfaceIsoCurveJet, error) {
	if c == nil || c.patch == nil {
		return SurfaceIsoCurveJet{}, ErrInvalid
	}
	j, e := c.patch.EvaluateJet(t, 0)
	return SurfaceIsoCurveJet{j.Point, j.Du, j.PointBounds, j.DuBounds, j.ExactOperations}, e
}

// BoundPlaneRange bounds the exact extracted curve's plane functional. The X
// coordinate of each witness is its curve parameter; Y is an auxiliary constant
// coordinate with no effect on position. Subdivision uses the curve axis only.
func (c *SurfaceIsoCurve) BoundPlaneRange(plane Plane, o SurfacePlaneRangeOptions) (SurfacePlaneExtrema, error) {
	return c.BoundPlaneRangeContext(context.Background(), plane, o)
}

// BoundPlaneRangeContext supports cooperative cancellation with retained bounds.
func (c *SurfaceIsoCurve) BoundPlaneRangeContext(ctx context.Context, plane Plane, o SurfacePlaneRangeOptions) (SurfacePlaneExtrema, error) {
	if c == nil || c.patch == nil {
		return SurfacePlaneExtrema{}, ErrInvalid
	}
	return boundSurfacePlaneRangeContext(ctx, c.patch, plane, o, true)
}
