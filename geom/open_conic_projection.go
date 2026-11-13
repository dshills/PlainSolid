package geom

import (
	"math"
	"math/big"
)

// ConicProjectionOptions bounds projection over the closed parameter arc.
// Zero limits select parameter tolerance 1e-10, 1M exact operations, 16384
// coefficient bits and 4096 root subdivisions. Start/End are required.
type ConicProjectionOptions struct {
	Start, End, ParameterTolerance                     float64
	MaxOperations, MaxCoefficientBits, MaxSubdivisions int
}

// ConicProjectionCandidate encloses one stationary point or endpoint and its
// exact squared distance for the real conic defined by stored coefficients.
// Point is floating evaluation at the bracket midpoint, not the exact root.
type ConicProjectionCandidate struct {
	Parameter, ParameterMin, ParameterMax  float64
	Point                                  Vec3
	SquaredDistanceMin, SquaredDistanceMax float64
	Multiplicity                           int
	Endpoint                               bool
}

// ConicProjection retains every candidate not excluded by rigorous squared
// distance comparison. Multiple candidates indicate unresolved ordering or a
// nonunique minimum; no arbitrary tie is silently declared unique. Complete
// means all stationary roots and endpoints were considered. On error partial
// candidates must not be treated as an exhaustive nearest-point result.
type ConicProjection struct {
	Candidates                             []ConicProjectionCandidate
	SquaredDistanceMin, SquaredDistanceMax float64
	UniqueMinimum, Complete                bool
	ExactOperations, Subdivisions          int
}

func polynomialDifference(a, c rationalPolynomial, b *polynomialBudget) (rationalPolynomial, error) {
	out := make(rationalPolynomial, max(len(a), len(c)))
	for i := range out {
		out[i] = new(big.Rat)
		if i < len(a) {
			out[i].Set(a[i])
		}
		if i < len(c) {
			out[i].Sub(out[i], c[i])
		}
		if e := b.check(out[i]); e != nil {
			return nil, e
		}
	}
	return trimPolynomial(out), nil
}
func conicWorldPolynomials(frame Frame, a, minor float64, branch int) ([3]rationalPolynomial, rationalPolynomial) {
	var numerator [3]rationalPolynomial
	weight := rationalPolynomial{big.NewRat(1, 1)}
	if branch != 0 {
		weight = rationalPolynomial{big.NewRat(1, 1), new(big.Rat), big.NewRat(-1, 1)}
	}
	for axis := 0; axis < 3; axis++ {
		origin := new(big.Rat).SetFloat64(coordinate(frame.origin, axis))
		x := new(big.Rat).Mul(new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(coordinate(frame.x, axis)))
		yScale := a
		if branch != 0 {
			yScale = minor
			x.Mul(x, big.NewRat(int64(branch), 1))
		}
		y := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(new(big.Rat).SetFloat64(yScale), new(big.Rat).SetFloat64(coordinate(frame.y, axis))))
		if branch == 0 {
			numerator[axis] = rationalPolynomial{origin, y, x}
		} else {
			numerator[axis] = rationalPolynomial{new(big.Rat).Add(origin, x), y, new(big.Rat).Sub(x, origin)}
		}
	}
	return numerator, weight
}
func polynomialInterval(p rationalPolynomial, x exactInterval, b *polynomialBudget) (exactInterval, error) {
	out := exactInterval{new(big.Rat), new(big.Rat)}
	for i := len(p) - 1; i >= 0; i-- {
		var e error
		out, e = intervalMultiply(out, x, b)
		if e != nil {
			return out, e
		}
		out, e = boundedExactInterval(new(big.Rat).Add(out.lo, p[i]), new(big.Rat).Add(out.hi, p[i]), b)
		if e != nil {
			return out, e
		}
	}
	return out, nil
}
func conicDistanceInterval(n [3]rationalPolynomial, w rationalPolynomial, p Vec3, lo, hi *big.Rat, b *polynomialBudget) (exactInterval, error) {
	x := exactInterval{lo, hi}
	weight, e := polynomialInterval(w, x, b)
	if e != nil {
		return exactInterval{}, e
	}
	sum := exactInterval{new(big.Rat), new(big.Rat)}
	for axis := 0; axis < 3; axis++ {
		v, e := polynomialInterval(n[axis], x, b)
		if e != nil {
			return sum, e
		}
		v, e = intervalDivide(v, weight, b)
		if e != nil {
			return sum, e
		}
		query := new(big.Rat).SetFloat64(coordinate(p, axis))
		v, e = intervalSubtract(v, exactInterval{query, query}, b)
		if e != nil {
			return sum, e
		}
		aa, cc := new(big.Rat).Mul(v.lo, v.lo), new(big.Rat).Mul(v.hi, v.hi)
		lower, upper := aa, cc
		if lower.Cmp(upper) > 0 {
			lower, upper = upper, lower
		}
		if v.lo.Sign() <= 0 && v.hi.Sign() >= 0 {
			lower = new(big.Rat)
		}
		sum, e = boundedExactInterval(new(big.Rat).Add(sum.lo, lower), new(big.Rat).Add(sum.hi, upper), b)
		if e != nil {
			return sum, e
		}
	}
	return sum, nil
}

// Project returns a globally bounded nearest-point candidate set on a parabola
// arc. Exact polynomial stationary-root isolation includes repeated roots and
// endpoints; squared-distance intervals govern candidate elimination.
func (p Parabola) Project(query Vec3, o ConicProjectionOptions) (ConicProjection, error) {
	if _, e := p.Eval(0); e != nil {
		return ConicProjection{}, e
	}
	return projectOpenConic(p, p.frame, p.focal, 0, 0, query, o)
}

// Project is the hyperbola counterpart, with Start/End strictly inside (-1,1).
func (h Hyperbola) Project(query Vec3, o ConicProjectionOptions) (ConicProjection, error) {
	if e := h.check(o.Start); e != nil {
		return ConicProjection{}, e
	}
	if e := h.check(o.End); e != nil {
		return ConicProjection{}, e
	}
	return projectOpenConic(h, h.frame, h.a, h.b, h.branch, query, o)
}
func projectOpenConic(curve Curve, frame Frame, a, minor float64, branch int, query Vec3, o ConicProjectionOptions) (result ConicProjection, err error) {
	if !query.IsFinite() || !finite(o.Start) || !finite(o.End) || o.Start >= o.End {
		return result, ErrInvalid
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 4096
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return result, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = b.operations }()
	n, w := conicWorldPolynomials(frame, a, minor, branch)
	stationary := rationalPolynomial{new(big.Rat)}
	for axis := 0; axis < 3; axis++ {
		pw := make(rationalPolynomial, len(w))
		for i, v := range w {
			pw[i] = new(big.Rat).Mul(v, new(big.Rat).SetFloat64(coordinate(query, axis)))
		}
		delta, e := polynomialDifference(n[axis], pw, b)
		if e != nil {
			return result, e
		}
		first, e := polynomialProduct(polynomialDerivative(n[axis]), w, b)
		if e != nil {
			return result, e
		}
		second, e := polynomialProduct(n[axis], polynomialDerivative(w), b)
		if e != nil {
			return result, e
		}
		derivative, e := polynomialDifference(first, second, b)
		if e != nil {
			return result, e
		}
		term, e := polynomialProduct(delta, derivative, b)
		if e != nil {
			return result, e
		}
		negative := make(rationalPolynomial, len(term))
		for i, v := range term {
			negative[i] = new(big.Rat).Neg(v)
		}
		stationary, e = polynomialDifference(stationary, negative, b)
		if e != nil {
			return result, e
		}
	}
	distances := []exactInterval{}
	appendCandidate := func(lo, hi *big.Rat, m int, endpoint bool) (bool, error) {
		lower, upper := rationalFloatBound(lo, true), rationalFloatBound(hi, false)
		if upper-lower > o.ParameterTolerance {
			return false, nil
		}
		distance, e := conicDistanceInterval(n, w, query, lo, hi, b)
		if e != nil {
			return false, e
		}
		parameter := lower/2 + upper/2
		parameter = min(upper, max(lower, parameter))
		point, e := curve.Eval(parameter)
		if e != nil {
			return false, e
		}
		result.Candidates = append(result.Candidates, ConicProjectionCandidate{parameter, lower, upper, point, rationalFloatBound(distance.lo, true), rationalFloatBound(distance.hi, false), m, endpoint})
		distances = append(distances, distance)
		return true, nil
	}
	start, end := new(big.Rat).SetFloat64(o.Start), new(big.Rat).SetFloat64(o.End)
	for _, x := range []*big.Rat{start, end} {
		if _, e := appendCandidate(x, x, 0, true); e != nil {
			return result, e
		}
	}
	if len(stationary) == 0 {
		return result, ErrDegenerate
	}
	result.Subdivisions, err = isolatePolynomialRoots(stationary, start, end, b, o.MaxSubdivisions, func(lo, hi *big.Rat, m int) (bool, error) {
		if lo.Cmp(hi) == 0 && (lo.Cmp(start) == 0 || lo.Cmp(end) == 0) {
			return true, nil
		}
		return appendCandidate(lo, hi, m, false)
	})
	if err != nil {
		return result, err
	}
	bestUpper := distances[0].hi
	bestLower := distances[0].lo
	for _, d := range distances {
		if d.hi.Cmp(bestUpper) < 0 {
			bestUpper = d.hi
		}
		if d.lo.Cmp(bestLower) < 0 {
			bestLower = d.lo
		}
	}
	retained := []ConicProjectionCandidate{}
	for i, c := range result.Candidates {
		if distances[i].lo.Cmp(bestUpper) <= 0 {
			retained = append(retained, c)
		}
	}
	result.Candidates = retained
	result.SquaredDistanceMin = rationalFloatBound(bestLower, true)
	result.SquaredDistanceMax = rationalFloatBound(bestUpper, false)
	result.UniqueMinimum = len(retained) == 1
	result.Complete = true
	if math.IsNaN(result.SquaredDistanceMax) {
		return result, ErrInvalid
	}
	return result, nil
}
