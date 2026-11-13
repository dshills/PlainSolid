package geom

import (
	"context"
	"math/big"
)

// ExactCurveEventOrder certifies the order of two independently isolated raw
// plane events on one exact curve. Order -1 means event A precedes B in the
// curve parameter, +1 follows, and zero proves identical algebraic parameters.
// Intervals are caller-owned rational enclosures in the original curve domain.
type ExactCurveEventOrder struct {
	Proven                   bool
	Order                    int
	Intervals                [2][2]*big.Rat
	Operations, Subdivisions int
}

// CompareExactCurvePlaneEvents reproves one distinct root in each supplied
// closed interval on a single rational Bezier span. A common polynomial factor
// with a root in their intersection proves equality; overlapping numeric boxes
// alone never do. Otherwise exact root refinement establishes strict order.
// Repeated roots are supported; coincident event planes and nonunique intervals
// remain unresolved. All plane coefficients are raw exact Nx,Ny,Nz,D values.
func CompareExactCurvePlaneEvents(ctx context.Context, c *ExactSplineCurve, planes [2][4]*big.Rat, intervals [2][2]*big.Rat, o ExactCurvePlaneEventOptions) (r ExactCurveEventOrder, err error) {
	if ctx == nil || c == nil || c.patch == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	p := c.patch
	if p.countU != p.degreeU+1 {
		return r, ErrConvergence
	}
	d := p.Domain()
	origin, width := new(big.Rat).SetFloat64(d.UMin), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin))
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = budget.operations }()
	var bounds [2][2]*big.Rat
	var polys [2]rationalPolynomial
	for i, interval := range intervals {
		if interval[0] == nil || interval[1] == nil || interval[0].Cmp(interval[1]) > 0 {
			return r, ErrInvalid
		}
		for j, t := range interval {
			bounds[i][j] = new(big.Rat).Quo(new(big.Rat).Sub(t, origin), width)
			if e := budget.check(bounds[i][j]); e != nil {
				return r, e
			}
		}
		if bounds[i][0].Sign() < 0 || bounds[i][1].Cmp(big.NewRat(1, 1)) > 0 {
			return r, ErrDomain
		}
		var e error
		polys[i], e = exactEventPlanePolynomial(p, planes[i], budget)
		if e != nil {
			return r, e
		}
		if len(polys[i]) == 0 {
			return r, ErrConvergence
		}
	}
	r, err = compareExactPolynomialEvents(ctx, polys, bounds, budget, o.MaxSubdivisions)
	if r.Proven {
		for i, interval := range r.Intervals {
			for j, t := range interval {
				r.Intervals[i][j] = new(big.Rat).Add(origin, new(big.Rat).Mul(width, t))
			}
		}
	}
	return r, err
}

func compareExactPolynomialEvents(ctx context.Context, polys [2]rationalPolynomial, bounds [2][2]*big.Rat, budget *polynomialBudget, maxSubdivisions int) (r ExactCurveEventOrder, err error) {
	roots := func(poly rationalPolynomial, lo, hi *big.Rat, accept func(*big.Rat, *big.Rat, int) (bool, error)) error {
		remaining := maxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return ErrConvergence
		}
		steps, e := isolatePolynomialRoots(poly, lo, hi, budget, remaining, accept)
		r.Subdivisions += steps
		return e
	}
	for i, poly := range polys {
		count := 0
		e := roots(poly, bounds[i][0], bounds[i][1], func(_, _ *big.Rat, _ int) (bool, error) { count++; return true, ctx.Err() })
		if e != nil {
			return r, e
		}
		if count != 1 {
			return r, ErrConvergence
		}
	}
	finish := func(order int) {
		r.Proven = true
		r.Order = order
		for i, interval := range bounds {
			for j, t := range interval {
				r.Intervals[i][j] = new(big.Rat).Set(t)
			}
		}
	}

	if bounds[0][1].Cmp(bounds[1][0]) < 0 {
		finish(-1)
		return r, nil
	}
	if bounds[1][1].Cmp(bounds[0][0]) < 0 {
		finish(1)
		return r, nil
	}
	gcd, e := polynomialGCD(polys[0], polys[1], budget)
	if e != nil {
		return r, e
	}
	lo, hi := bounds[0][0], bounds[0][1]
	if bounds[1][0].Cmp(lo) > 0 {
		lo = bounds[1][0]
	}
	if bounds[1][1].Cmp(hi) < 0 {
		hi = bounds[1][1]
	}
	count := 0
	e = roots(gcd, lo, hi, func(_, _ *big.Rat, _ int) (bool, error) { count++; return true, ctx.Err() })
	if e != nil {
		return r, e
	}
	if count == 1 {
		bounds = [2][2]*big.Rat{{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}, {new(big.Rat).Set(lo), new(big.Rat).Set(hi)}}
		finish(0)
		return r, nil
	}
	for {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		for i, poly := range polys {
			if bounds[i][0].Cmp(bounds[i][1]) == 0 {
				continue
			}
			target := new(big.Rat).Quo(new(big.Rat).Sub(bounds[i][1], bounds[i][0]), big.NewRat(2, 1))
			e := roots(poly, bounds[i][0], bounds[i][1], func(lo, hi *big.Rat, _ int) (bool, error) {
				if e := ctx.Err(); e != nil {
					return false, e
				}
				if new(big.Rat).Sub(hi, lo).Cmp(target) > 0 {
					return false, nil
				}
				bounds[i] = [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}
				return true, nil
			})
			if e != nil {
				return r, e
			}
		}
		if bounds[0][1].Cmp(bounds[1][0]) < 0 {
			finish(-1)
			return r, nil
		}
		if bounds[1][1].Cmp(bounds[0][0]) < 0 {
			finish(1)
			return r, nil
		}
	}
}

func exactEventPlanePolynomial(p *SurfacePatch, plane [4]*big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	normal := false
	for axis, x := range plane {
		if x == nil {
			x = new(big.Rat)
		}
		plane[axis] = new(big.Rat).Set(x)
		if e := b.check(x); e != nil {
			return nil, e
		}
		normal = normal || (axis < 3 && x.Sign() != 0)
	}
	if !normal {
		return nil, ErrInvalid
	}
	coeff := make([]*big.Rat, p.countU)
	for i := range coeff {
		coeff[i] = new(big.Rat)
		h := p.net[i*p.countV]
		for axis := 0; axis < 4; axis++ {
			coeff[i].Add(coeff[i], new(big.Rat).Mul(plane[axis], h[axis]))
			if e := b.check(coeff[i]); e != nil {
				return nil, e
			}
		}
	}
	return bernsteinToPower(coeff, b)
}
