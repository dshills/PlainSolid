package geom

import (
	"context"
	"math/big"
	"sort"
)

// ExactCurveComparisonOptions aligns domains only when NormalizeDomains is set;
// otherwise original domain endpoints must match. ReverseB compares A forward
// to B backward. Alignment and union knot refinement use exact rational maps,
// including normalized knots that cannot be represented in float64. Zero limits
// select 8192 common spans, 4M operations and 16384 coefficient bits.
type ExactCurveComparisonOptions struct {
	NormalizeDomains, ReverseB                  bool
	MaxSpans, MaxOperations, MaxCoefficientBits int
}

// ExactCurveComparison proves identity or nonidentity of parameterized rational
// curves. Complete && Equal proves equality everywhere after selected alignment.
// Complete && !Equal proves nonidentity (not disjointness). DifferentSpan is the
// zero-based first nonidentical common interval, DifferentAxis its coordinate.
// Common intervals use normalized coordinates internally; an error is unresolved.
type ExactCurveComparison struct {
	Equal, Complete                                              bool
	ComparedSpans, DifferentSpan, DifferentAxis, ExactOperations int
}

// CompareExactCurves tests the homogeneous polynomial identities Xa*Wb=Xb*Wa.
// Different degrees, knot partitions and weight scales are supported. Positive
// weights ensure that cross multiplication introduces no extraneous identities.
func CompareExactCurves(a, b *ExactSplineCurve, o ExactCurveComparisonOptions) (ExactCurveComparison, error) {
	return CompareExactCurvesContext(context.Background(), a, b, o)
}

// CompareExactCurvesContext checks cancellation between bounded refinement steps.
func CompareExactCurvesContext(ctx context.Context, a, b *ExactSplineCurve, o ExactCurveComparisonOptions) (r ExactCurveComparison, err error) {
	r.DifferentSpan, r.DifferentAxis = -1, -1
	if ctx == nil || a == nil || b == nil || a.patch == nil || b.patch == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	a0, a1 := a.Domain()
	b0, b1 := b.Domain()
	if !o.NormalizeDomains && (a0 != b0 || a1 != b1) {
		return r, ErrDomain
	}
	if o.MaxSpans == 0 {
		o.MaxSpans = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSpans < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	type span struct {
		lo, hi *big.Rat
		net    []exactHomogeneous
	}
	extract := func(curve *SurfaceIsoCurve, reverse bool) ([]span, error) {
		lower, upper := curve.Domain()
		origin := new(big.Rat).SetFloat64(lower)
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(upper), origin)
		normalized := func(t float64) *big.Rat {
			x := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(t), origin), width)
			if reverse {
				x.Sub(big.NewRat(1, 1), x)
			}
			return x
		}
		stack := []*SurfacePatch{curve.patch}
		var spans []span
		for len(stack) > 0 {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if k, ok := internalPatchKnot(p.knotsU, p.degreeU, p.countU); ok {
				if len(stack)+len(spans)+2 > o.MaxSpans {
					return nil, ErrConvergence
				}
				left, right, e := p.split(k, true, budget)
				if e != nil {
					return nil, e
				}
				stack = append(stack, right, left)
				continue
			}
			d := p.Domain()
			lo, hi := normalized(d.UMin), normalized(d.UMax)
			net := make([]exactHomogeneous, p.countU)
			for i := range net {
				index := i
				if reverse {
					index = len(net) - 1 - i
				}
				net[i] = p.net[index*p.countV]
			}
			if reverse {
				lo, hi = hi, lo
			}
			if e := budget.check(lo); e != nil {
				return nil, e
			}
			if e := budget.check(hi); e != nil {
				return nil, e
			}
			spans = append(spans, span{lo, hi, net})
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i].lo.Cmp(spans[j].lo) < 0 })
		return spans, nil
	}
	as, e := extract(a, false)
	if e != nil {
		return r, e
	}
	bs, e := extract(b, o.ReverseB)
	if e != nil {
		return r, e
	}
	breaks := []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}
	for _, spans := range [][]span{as, bs} {
		for _, s := range spans {
			breaks = append(breaks, s.lo, s.hi)
		}
	}
	sort.Slice(breaks, func(i, j int) bool { return breaks[i].Cmp(breaks[j]) < 0 })
	unique := breaks[:0]
	for _, x := range breaks {
		if len(unique) == 0 || x.Cmp(unique[len(unique)-1]) != 0 {
			unique = append(unique, x)
		}
	}
	if len(unique)-1 > o.MaxSpans {
		return r, ErrConvergence
	}
	ai, bi := 0, 0
	for index := 0; index < len(unique)-1; index++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		lo, hi := unique[index], unique[index+1]
		for as[ai].hi.Cmp(lo) <= 0 {
			ai++
		}
		for bs[bi].hi.Cmp(lo) <= 0 {
			bi++
		}
		var powers [2][4]rationalPolynomial
		for side, s := range []span{as[ai], bs[bi]} {
			width := new(big.Rat).Sub(s.hi, s.lo)
			localLo := new(big.Rat).Quo(new(big.Rat).Sub(lo, s.lo), width)
			localHi := new(big.Rat).Quo(new(big.Rat).Sub(hi, s.lo), width)
			net, e := restrictHomogeneousBezier(s.net, localLo, localHi, budget)
			if e != nil {
				return r, e
			}
			for axis := 0; axis < 4; axis++ {
				coefficients := make([]*big.Rat, len(net))
				for i := range net {
					coefficients[i] = net[i][axis]
				}
				powers[side][axis], e = bernsteinToPower(coefficients, budget)
				if e != nil {
					return r, e
				}
			}
		}
		r.ComparedSpans++
		for axis := 0; axis < 3; axis++ {
			left, e := polynomialProduct(powers[0][axis], powers[1][3], budget)
			if e != nil {
				return r, e
			}
			right, e := polynomialProduct(powers[1][axis], powers[0][3], budget)
			if e != nil {
				return r, e
			}
			same := len(left) == len(right)
			if same {
				for i := range left {
					if left[i].Cmp(right[i]) != 0 {
						same = false
						break
					}
				}
			}
			if !same {
				r.Complete = true
				r.DifferentSpan = index
				r.DifferentAxis = axis
				return r, nil
			}
		}
	}
	r.Complete, r.Equal = true, true
	return r, nil
}

func splitHomogeneousBezier(net []exactHomogeneous, t *big.Rat, b *polynomialBudget) ([]exactHomogeneous, []exactHomogeneous, error) {
	work := append([]exactHomogeneous(nil), net...)
	left, right := make([]exactHomogeneous, len(net)), make([]exactHomogeneous, len(net))
	left[0], right[len(net)-1] = work[0], work[len(net)-1]
	complement := new(big.Rat).Sub(big.NewRat(1, 1), t)
	for level := 1; level < len(net); level++ {
		for i := 0; i < len(net)-level; i++ {
			var h exactHomogeneous
			for axis := 0; axis < 4; axis++ {
				h[axis] = new(big.Rat).Add(new(big.Rat).Mul(complement, work[i][axis]), new(big.Rat).Mul(t, work[i+1][axis]))
				if e := b.check(h[axis]); e != nil {
					return nil, nil, e
				}
			}
			work[i] = h
		}
		left[level], right[len(net)-1-level] = work[0], work[len(net)-1-level]
	}
	return left, right, nil
}
func restrictHomogeneousBezier(net []exactHomogeneous, lo, hi *big.Rat, b *polynomialBudget) ([]exactHomogeneous, error) {
	current := net
	if hi.Cmp(big.NewRat(1, 1)) < 0 {
		left, _, e := splitHomogeneousBezier(current, hi, b)
		if e != nil {
			return nil, e
		}
		current = left
	}
	if lo.Sign() > 0 {
		_, right, e := splitHomogeneousBezier(current, new(big.Rat).Quo(lo, hi), b)
		if e != nil {
			return nil, e
		}
		current = right
	}
	return current, nil
}
