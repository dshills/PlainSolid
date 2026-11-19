package geom

import (
	"context"
	"math/big"
	"runtime"
)

// ExactParameterKnotEvent retains immutable UV-curve and unique algebraic
// parameter provenance for U=knot or V=knot. It may be a crossing or tangency;
// identically coincident knot paths do not define a unique event.
type ExactParameterKnotEvent struct {
	curve        *ExactSplineCurve
	root         *ExactGraphLineRoot
	axis         int
	knot         *big.Rat
	multiplicity int
}

func (e *ExactParameterKnotEvent) Interval() ExactParameterInterval {
	if e == nil {
		return ExactParameterInterval{}
	}
	return e.root.Interval()
}
func (e *ExactParameterKnotEvent) Multiplicity() int {
	if e == nil {
		return 0
	}
	return e.multiplicity
}

// NewExactParameterKnotEvent proves exactly one distinct root in the given
// closed normalized bracket. The UV curve must be a single positive rational
// Bézier on [0,1], with identically zero Z. No algebraic coordinate is rounded.
func NewExactParameterKnotEvent(ctx context.Context, curve *ExactSplineCurve, axis int, knot *big.Rat, interval ExactParameterInterval, o ExactCurvePlaneEventOptions) (*ExactParameterKnotEvent, error) {
	if ctx == nil || curve == nil || curve.patch == nil || axis < 0 || axis > 1 || knot == nil || interval.Lower == nil || interval.Upper == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if interval.Lower.Sign() < 0 || interval.Upper.Cmp(big.NewRat(1, 1)) > 0 || interval.Lower.Cmp(interval.Upper) > 0 {
		return nil, ErrDomain
	}
	lo, hi := curve.Domain()
	p := curve.patch
	if lo != 0 || hi != 1 || p.countU != p.degreeU+1 || p.degreeU > 64 {
		return nil, ErrConvergence
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return nil, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, x := range []*big.Rat{knot, interval.Lower, interval.Upper} {
		if e := b.check(x); e != nil {
			return nil, e
		}
	}
	values := make([]*big.Rat, p.countU)
	for i := range values {
		h := p.net[i*p.countV]
		if h[2].Sign() != 0 {
			return nil, ErrInvalid
		}
		values[i] = new(big.Rat).Sub(h[axis], new(big.Rat).Mul(knot, h[3]))
		runtime.KeepAlive(&h) // golang/go#82026: keeps h in the stack map. Remove once the Go fix is in use.
		if e := b.check(values[i]); e != nil {
			return nil, e
		}
	}
	polynomial, e := bernsteinToPower(values, b)
	if e != nil {
		return nil, e
	}
	if len(polynomial) == 0 {
		return nil, ErrConvergence
	}
	count, multiplicity := 0, 0
	steps, e := isolatePolynomialRoots(polynomial, interval.Lower, interval.Upper, b, o.MaxSubdivisions, func(_, _ *big.Rat, m int) (bool, error) { count++; multiplicity = m; return true, ctx.Err() })
	if e != nil {
		return nil, e
	}
	if count != 1 {
		return nil, ErrConvergence
	}
	root := &ExactGraphLineRoot{polynomial: polynomial, bounds: [2]*big.Rat{new(big.Rat).Set(interval.Lower), new(big.Rat).Set(interval.Upper)}, operations: b.operations, subdivisions: steps}
	return &ExactParameterKnotEvent{curve: curve, root: root, axis: axis, knot: new(big.Rat).Set(knot), multiplicity: multiplicity}, nil
}

// AlgebraicSurfaceKnotAgreement proves model point equality or non-equality at
// a shared exact UV event. It does not assert equality of neighboring arcs or
// derivatives, nor export rational parameter endpoints.
type AlgebraicSurfaceKnotAgreement struct {
	Proven, Equal            bool
	Interval                 ExactParameterInterval
	Operations, Subdivisions int
}

// CompareSurfaceKnotEventModels compares two single tensor Bézier cells at the
// event. Its knot must be a boundary of both cells on the selected axis. All UV
// coordinates are proved inside both closed cells; positivity then guarantees
// finite rational surface evaluation. Exact polynomial GCD identities at the
// unique source root prove the homogeneous model-coordinate comparisons.
func CompareSurfaceKnotEventModels(ctx context.Context, event *ExactParameterKnotEvent, a, c *SurfacePatch, o ExactCurvePlaneEventOptions) (r AlgebraicSurfaceKnotAgreement, err error) {
	if ctx == nil || event == nil || event.root == nil || event.curve == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	bounds := [2]*big.Rat{new(big.Rat).Set(event.root.bounds[0]), new(big.Rat).Set(event.root.bounds[1])}
	count := func(poly rationalPolynomial) (int, error) {
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return 0, ErrConvergence
		}
		n := 0
		steps, e := isolatePolynomialRoots(poly, bounds[0], bounds[1], b, remaining, func(_, _ *big.Rat, _ int) (bool, error) { n++; return true, ctx.Err() })
		r.Subdivisions += steps
		return n, e
	}
	for _, x := range event.root.polynomial {
		if e := b.check(x); e != nil {
			return r, e
		}
	}
	n, e := count(event.root.polynomial)
	if e != nil {
		return r, e
	}
	if n != 1 {
		return r, ErrConvergence
	}
	zeroAtRoot := func(poly rationalPolynomial) (bool, error) {
		if len(trimPolynomial(poly)) == 0 {
			return true, nil
		}
		gcd, e := polynomialGCD(event.root.polynomial, poly, b)
		if e != nil {
			return false, e
		}
		n, e := count(gcd)
		return n == 1, e
	}
	cp := event.curve.patch
	var coordinate [3]rationalPolynomial
	for axis, component := range []int{0, 1, 3} {
		values := make([]*big.Rat, cp.countU)
		for i := range values {
			values[i] = cp.net[i*cp.countV][component]
		}
		coordinate[axis], e = bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
	}
	// Closed domain membership is established by zero tests or strict interval
	// signs at the algebraic event, not by its floating representative.
	for _, p := range []*SurfacePatch{a, c} {
		if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return r, ErrConvergence
		}
		d := p.Domain()
		ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		if event.knot.Cmp(new(big.Rat).SetFloat64(ranges[event.axis][0])) != 0 && event.knot.Cmp(new(big.Rat).SetFloat64(ranges[event.axis][1])) != 0 {
			return r, ErrDomain
		}
		for axis := 0; axis < 2; axis++ {
			for end := 0; end < 2; end++ {
				poly, e := subtractScaledPolynomial(coordinate[axis], coordinate[2], new(big.Rat).SetFloat64(ranges[axis][end]), b)
				if e != nil {
					return r, e
				}
				zero, e := zeroAtRoot(poly)
				if e != nil {
					return r, e
				}
				if zero {
					continue
				}
				remaining := o.MaxSubdivisions - r.Subdivisions
				if remaining < 1 {
					return r, ErrConvergence
				}
				side := 0
				steps, e := isolatePolynomialRoots(event.root.polynomial, bounds[0], bounds[1], b, remaining, func(lo, hi *big.Rat, _ int) (bool, error) {
					if e := ctx.Err(); e != nil {
						return false, e
					}
					value, e := polynomialInterval(poly, exactInterval{lo: lo, hi: hi}, b)
					if e != nil {
						return false, e
					}
					if value.lo.Sign() > 0 {
						side = 1
					} else if value.hi.Sign() < 0 {
						side = -1
					} else {
						return false, nil
					}
					bounds = [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}
					return true, nil
				})
				r.Subdivisions += steps
				if e != nil {
					return r, e
				}
				if side == 0 {
					return r, ErrConvergence
				}
				if end == 0 && side < 0 || end == 1 && side > 0 {
					return r, ErrDomain
				}
			}
		}
	}
	left, e := symbolicSurfaceCurvePower(ctx, a, coordinate, b)
	if e != nil {
		return r, e
	}
	right, e := symbolicSurfaceCurvePower(ctx, c, coordinate, b)
	if e != nil {
		return r, e
	}
	equal := true
	for axis := 0; axis < 3; axis++ {
		x, e := polynomialProduct(left[axis], right[3], b)
		if e != nil {
			return r, e
		}
		y, e := polynomialProduct(right[axis], left[3], b)
		if e != nil {
			return r, e
		}
		difference, e := subtractScaledPolynomial(x, y, big.NewRat(1, 1), b)
		if e != nil {
			return r, e
		}
		same, e := zeroAtRoot(difference)
		if e != nil {
			return r, e
		}
		if !same {
			equal = false
			break
		}
	}
	r.Proven = true
	r.Equal = equal
	r.Interval = ExactParameterInterval{Lower: new(big.Rat).Set(bounds[0]), Upper: new(big.Rat).Set(bounds[1])}
	return r, nil
}
func subtractScaledPolynomial(a, c rationalPolynomial, scale *big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	out := make(rationalPolynomial, max(len(a), len(c)))
	for i := range out {
		out[i] = new(big.Rat)
		if i < len(a) {
			out[i].Set(a[i])
		}
		if i < len(c) {
			out[i].Sub(out[i], new(big.Rat).Mul(scale, c[i]))
		}
		if e := b.check(out[i]); e != nil {
			return nil, e
		}
	}
	return trimPolynomial(out), nil
}

// symbolicSurfaceCurvePower keeps rational continuations rather than exporting
// a positive-weight curve across an irrational cell cut.
func symbolicSurfaceCurvePower(ctx context.Context, p *SurfacePatch, uv [3]rationalPolynomial, b *polynomialBudget) (out [4]rationalPolynomial, err error) {
	degree := max(len(uv[0]), max(len(uv[1]), len(uv[2]))) - 1
	if degree*(p.degreeU+p.degreeV) > 64 {
		return out, ErrConvergence
	}
	d := p.Domain()
	ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	var basis [2][]rationalPolynomial
	for axis, n := range []int{p.degreeU, p.degreeV} {
		origin := new(big.Rat).SetFloat64(ranges[axis][0])
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(ranges[axis][1]), origin)
		coordinate, e := subtractScaledPolynomial(uv[axis], uv[2], origin, b)
		if e != nil {
			return out, e
		}
		for _, x := range coordinate {
			x.Quo(x, width)
			if e := b.check(x); e != nil {
				return out, e
			}
		}
		complement, e := subtractScaledPolynomial(uv[2], coordinate, big.NewRat(1, 1), b)
		if e != nil {
			return out, e
		}
		powers, others := make([]rationalPolynomial, n+1), make([]rationalPolynomial, n+1)
		powers[0] = rationalPolynomial{big.NewRat(1, 1)}
		others[0] = rationalPolynomial{big.NewRat(1, 1)}
		for i := 1; i <= n; i++ {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			powers[i], e = polynomialProduct(powers[i-1], coordinate, b)
			if e != nil {
				return out, e
			}
			others[i], e = polynomialProduct(others[i-1], complement, b)
			if e != nil {
				return out, e
			}
		}
		basis[axis] = make([]rationalPolynomial, n+1)
		for i := range basis[axis] {
			basis[axis][i], e = polynomialProduct(powers[i], others[n-i], b)
			if e != nil {
				return out, e
			}
			factor := new(big.Rat).SetInt(new(big.Int).Binomial(int64(n), int64(i)))
			for _, x := range basis[axis][i] {
				x.Mul(x, factor)
				if e := b.check(x); e != nil {
					return out, e
				}
			}
		}
	}
	for i := 0; i < p.countU; i++ {
		for j := 0; j < p.countV; j++ {
			product, e := polynomialProduct(basis[0][i], basis[1][j], b)
			if e != nil {
				return out, e
			}
			h := p.net[i*p.countV+j]
			for axis := 0; axis < 4; axis++ {
				for len(out[axis]) < len(product) {
					out[axis] = append(out[axis], new(big.Rat))
				}
				for k, x := range product {
					out[axis][k].Add(out[axis][k], new(big.Rat).Mul(h[axis], x))
					if e := b.check(out[axis][k]); e != nil {
						return out, e
					}
				}
			}
		}
	}
	for axis := range out {
		out[axis] = trimPolynomial(out[axis])
	}
	return out, nil
}
