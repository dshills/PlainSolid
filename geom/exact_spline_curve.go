package geom

import "math"

// ExactSplineCurve is the immutable homogeneous-curve descriptor also used for
// surface iso curves. Model-curve construction marks FixedParameter as (-1,NaN).
// Its operations preserve rational coefficients instead of rebuilding float
// control points. Existing SurfaceIsoCurve values are directly compatible.
type ExactSplineCurve = SurfaceIsoCurve

// NewExactSplineCurve copies a continuous clamped positive-weight B-spline into
// exact homogeneous arithmetic. The represented source poles and weights define
// the geometry exactly; no extra weight normalization occurs. Options bound
// construction and subsequent individual evaluations. MaxPoles includes the
// internal duplicated constant auxiliary coordinate (two rows per curve pole).
func NewExactSplineCurve(c *BSpline, options SurfacePatchOptions) (*ExactSplineCurve, error) {
	o, e := patchOptions(options)
	if e != nil {
		return nil, e
	}
	if c == nil || len(c.poles) == 0 {
		return nil, ErrInvalid
	}
	if len(c.poles) > o.MaxPoles/2 {
		return nil, ErrConvergence
	}
	p, e := curveDistancePatch(c, o.MaxOperations, o.MaxCoefficientBits)
	if e != nil {
		return nil, e
	}
	p.options = o
	return &SurfaceIsoCurve{patch: p, fixedAxis: -1, fixedParameter: math.NaN(), operations: p.operations}, nil
}

// Restrict returns the exact nonempty closed original-parameter interval. Bounds
// must be represented float64 values inside Domain. Interior knots are retained
// and both newly introduced endpoints are clamped exactly by knot insertion.
// This does not reparameterize. Failure leaves the source unchanged.
func (c *SurfaceIsoCurve) Restrict(start, end float64, options SurfacePatchOptions) (*ExactSplineCurve, error) {
	o, e := patchOptions(options)
	if e != nil {
		return nil, e
	}
	if c == nil || c.patch == nil || !finite(start) || !finite(end) || start >= end {
		return nil, ErrInvalid
	}
	d := c.patch.Domain()
	if start < d.UMin || end > d.UMax {
		return nil, ErrDomain
	}
	if len(c.patch.net) > o.MaxPoles {
		return nil, ErrConvergence
	}
	p := *c.patch
	p.options = o
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	current := &p
	if start > d.UMin {
		_, right, e := current.split(start, true, b)
		if e != nil {
			return nil, e
		}
		current = right
	}
	if end < d.UMax {
		left, _, e := current.split(end, true, b)
		if e != nil {
			return nil, e
		}
		current = left
	}
	// Full-domain restrictions still verify caller coefficient limits.
	for _, h := range current.net {
		for _, v := range h {
			if e := b.check(v); e != nil {
				return nil, e
			}
		}
	}
	result := *current
	result.options = o
	result.operations = b.operations
	return &SurfaceIsoCurve{patch: &result, fixedAxis: c.fixedAxis, fixedParameter: c.fixedParameter, operations: b.operations}, nil
}

// Split creates exact closed restrictions meeting at an interior represented
// parameter. Their common endpoint is identical exact geometry. The option
// budget covers both outputs together; ExactOperations on each reports that
// shared construction cost, so callers must not sum these duplicate counters.
func (c *SurfaceIsoCurve) Split(parameter float64, options SurfacePatchOptions) (*ExactSplineCurve, *ExactSplineCurve, error) {
	o, e := patchOptions(options)
	if e != nil {
		return nil, nil, e
	}
	if c == nil || c.patch == nil {
		return nil, nil, ErrInvalid
	}
	if !finite(parameter) {
		return nil, nil, ErrDomain
	}
	if len(c.patch.net) > o.MaxPoles {
		return nil, nil, ErrConvergence
	}
	p := *c.patch
	p.options = o
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	left, right, e := p.split(parameter, true, b)
	if e != nil {
		return nil, nil, e
	}
	left.operations, right.operations = b.operations, b.operations
	return &SurfaceIsoCurve{patch: left, fixedAxis: c.fixedAxis, fixedParameter: c.fixedParameter, operations: b.operations}, &SurfaceIsoCurve{patch: right, fixedAxis: c.fixedAxis, fixedParameter: c.fixedParameter, operations: b.operations}, nil
}
