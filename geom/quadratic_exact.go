package geom

import (
	"math"
	"math/big"
)

type exactQuadratic struct{ a, b, c *big.Rat }

func (q exactQuadratic) value(x *big.Rat) *big.Rat {
	return new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Add(new(big.Rat).Mul(q.a, x), q.b), x), q.c)
}

type quadraticRoot struct {
	parameter, lower, upper float64
	slope                   int
	exact                   *big.Rat
}

func (r quadraticRoot) compare(x *big.Rat, q exactQuadratic) int {
	if r.exact != nil {
		return r.exact.Cmp(x)
	}
	if x.Cmp(new(big.Rat).SetFloat64(r.lower)) < 0 {
		return 1
	}
	if x.Cmp(new(big.Rat).SetFloat64(r.upper)) > 0 {
		return -1
	}
	return -q.value(x).Sign() * r.slope
}
func rationalRoot(r *big.Rat) (quadraticRoot, error) {
	value, _ := r.Float64()
	if !finite(value) {
		return quadraticRoot{}, ErrConvergence
	}
	lower, upper := rationalFloatBound(r, true), rationalFloatBound(r, false)
	if !finite(lower) || !finite(upper) {
		return quadraticRoot{}, ErrConvergence
	}
	return quadraticRoot{value, lower, upper, 0, new(big.Rat).Set(r)}, nil
}
func bracketQuadraticRoot(q exactQuadratic, value float64, slope int) (quadraticRoot, error) {
	if !finite(value) {
		return quadraticRoot{}, ErrConvergence
	}
	x := new(big.Rat).SetFloat64(value)
	if q.value(x).Sign() == 0 {
		r, e := rationalRoot(x)
		r.slope = slope
		return r, e
	}
	lower, upper := math.Nextafter(value, math.Inf(-1)), math.Nextafter(value, math.Inf(1))
	if !finite(lower) || !finite(upper) {
		return quadraticRoot{}, ErrConvergence
	}
	left, right := q.value(new(big.Rat).SetFloat64(lower)).Sign(), q.value(new(big.Rat).SetFloat64(upper)).Sign()
	if left == 0 {
		r, e := rationalRoot(new(big.Rat).SetFloat64(lower))
		r.slope = slope
		return r, e
	}
	if right == 0 {
		r, e := rationalRoot(new(big.Rat).SetFloat64(upper))
		r.slope = slope
		return r, e
	}
	if left == right {
		return quadraticRoot{}, ErrConvergence
	}
	return quadraticRoot{value, lower, upper, slope, nil}, nil
}

// solveExactQuadratic uses exact coefficient/discriminant signs, a stable high
// precision q-formula, and exact sign brackets around each float64 root.
func solveExactQuadratic(q exactQuadratic, maxPrecision uint) (roots []quadraticRoot, coincident, tangent bool, precision uint, err error) {
	if q.a.Sign() == 0 {
		if q.b.Sign() == 0 {
			return nil, q.c.Sign() == 0, false, 0, nil
		}
		root, e := rationalRoot(new(big.Rat).Neg(new(big.Rat).Quo(q.c, q.b)))
		root.slope = q.b.Sign()
		return []quadraticRoot{root}, false, false, 0, e
	}
	discriminant := new(big.Rat).Sub(new(big.Rat).Mul(q.b, q.b), new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(q.a, q.c)))
	if discriminant.Sign() < 0 {
		return nil, false, false, 0, nil
	}
	if discriminant.Sign() == 0 {
		root, e := rationalRoot(new(big.Rat).Neg(new(big.Rat).Quo(q.b, new(big.Rat).Mul(big.NewRat(2, 1), q.a))))
		return []quadraticRoot{root}, false, true, 0, e
	}
	for bits := uint(64); ; bits *= 2 {
		if bits > maxPrecision {
			bits = maxPrecision
		}
		number := func(r *big.Rat) *big.Float { return new(big.Float).SetPrec(bits).SetRat(r) }
		sqrt := new(big.Float).SetPrec(bits).Sqrt(number(discriminant))
		bf := number(q.b)
		if q.b.Sign() < 0 {
			sqrt.Neg(sqrt)
		}
		stable := new(big.Float).SetPrec(bits).Add(bf, sqrt)
		stable.Mul(stable, new(big.Float).SetPrec(bits).SetFloat64(-.5))
		first := new(big.Float).SetPrec(bits).Quo(stable, number(q.a))
		second := new(big.Float).SetPrec(bits).Quo(number(q.c), stable)
		v1, _ := first.Float64()
		v2, _ := second.Float64()
		if v1 > v2 {
			v1, v2 = v2, v1
		}
		left, e1 := bracketQuadraticRoot(q, v1, -q.a.Sign())
		right, e2 := bracketQuadraticRoot(q, v2, q.a.Sign())
		if e1 == nil && e2 == nil && left.parameter < right.parameter {
			return []quadraticRoot{left, right}, false, false, bits, nil
		}
		if bits == maxPrecision {
			return nil, false, false, bits, ErrConvergence
		}
	}
}
