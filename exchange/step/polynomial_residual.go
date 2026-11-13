package step

import (
	"fmt"
	"io"
	"math/big"
	"plainsolid/geom"
)

// PolynomialSphereResidual is continuous evidence about a curve's supporting
// sphere equation, not a trim, topology, nearest-point or material certificate.
// Values use exact STEP decimal poles/knots and represented input center/radius.
type PolynomialSphereResidual struct {
	Spans                                    []PolynomialSphereResidualSpan
	MaxAbsoluteResidual, RadialDistanceUpper *big.Rat
}
type PolynomialSphereResidualSpan struct {
	Start, End   *big.Rat
	Lower, Upper *big.Rat
}

// BoundPolynomialSplineSphereResidual encloses |C(t)-center|²-radius² on every
// nonempty knot span via exact degree-2p Bernstein coefficients. The radial
// distance upper bound divides the absolute residual by radius. Scope: simple
// nonrational clamped 3D STEP B-splines, degree1..8, at most64 spans/512 poles.
// Decimal size and rational coefficient bit budgets fail closed. No acceptance
// of a body or its source ownership is implied by a small residual.
func BoundPolynomialSplineSphereResidual(r io.Reader, l Limits, curveID int64, center geom.Vec3, radius float64) (PolynomialSphereResidual, error) {
	return boundPolynomialSplineSupportResidual(r, l, curveID, center, radius, nil)
}

func boundPolynomialSplineSupportResidual(r io.Reader, l Limits, curveID int64, center geom.Vec3, radius float64, cylinderAxis *geom.Vec3) (out PolynomialSphereResidual, err error) {
	fail := func() (PolynomialSphereResidual, error) {
		return PolynomialSphereResidual{}, fmt.Errorf("step: unsupported polynomial residual input or budget")
	}
	if !center.IsFinite() || !finiteNumber(radius) || radius <= 0 {
		return fail()
	}
	var axis [3]*big.Rat
	axisSquared := new(big.Rat)
	if cylinderAxis != nil {
		if !cylinderAxis.IsFinite() {
			return fail()
		}
		axis = [3]*big.Rat{new(big.Rat).SetFloat64(cylinderAxis.X), new(big.Rat).SetFloat64(cylinderAxis.Y), new(big.Rat).SetFloat64(cylinderAxis.Z)}
		for _, q := range axis {
			axisSquared.Add(axisSquared, new(big.Rat).Mul(q, q))
		}
		if axisSquared.Sign() == 0 {
			return fail()
		}
	}
	p, poles, knots, e := readResidualPolynomial(r, l, curveID)
	if e != nil {
		return out, e
	}
	n := 2 * p
	inverse, e := residualBernsteinInverse(n)
	if e != nil {
		return fail()
	}
	c := [3]*big.Rat{new(big.Rat).SetFloat64(center.X), new(big.Rat).SetFloat64(center.Y), new(big.Rat).SetFloat64(center.Z)}
	rr := new(big.Rat).SetFloat64(radius)
	r2 := new(big.Rat).Mul(rr, rr)
	out.MaxAbsoluteResidual = new(big.Rat)
	for k := p; k < len(poles); k++ {
		if knots[k].Cmp(knots[k+1]) == 0 {
			continue
		}
		a, b := knots[k], knots[k+1]
		values := make([]*big.Rat, n+1)
		for j := 0; j <= n; j++ {
			t := new(big.Rat).Add(a, new(big.Rat).Mul(new(big.Rat).Sub(b, a), big.NewRat(int64(j), int64(n))))
			point, e := residualDeBoor(p, poles, knots, k, t)
			if e != nil {
				return fail()
			}
			v := new(big.Rat).Neg(r2)
			dot := new(big.Rat)
			for coordinate := 0; coordinate < 3; coordinate++ {
				d := new(big.Rat).Sub(point[coordinate], c[coordinate])
				v.Add(v, new(big.Rat).Mul(d, d))
				if cylinderAxis != nil {
					dot.Add(dot, new(big.Rat).Mul(d, axis[coordinate]))
				}
			}
			if cylinderAxis != nil {
				v.Sub(v, new(big.Rat).Quo(new(big.Rat).Mul(dot, dot), axisSquared))
			}
			if !residualBits(v) {
				return fail()
			}
			values[j] = v
		}
		var lo, hi *big.Rat
		for i := 0; i <= n; i++ {
			q := new(big.Rat)
			for j := 0; j <= n; j++ {
				q.Add(q, new(big.Rat).Mul(inverse[i][j], values[j]))
			}
			if !residualBits(q) {
				return fail()
			}
			if lo == nil || q.Cmp(lo) < 0 {
				lo = new(big.Rat).Set(q)
			}
			if hi == nil || q.Cmp(hi) > 0 {
				hi = new(big.Rat).Set(q)
			}
		}
		out.Spans = append(out.Spans, PolynomialSphereResidualSpan{new(big.Rat).Set(a), new(big.Rat).Set(b), lo, hi})
		for _, v := range []*big.Rat{lo, hi} {
			abs := new(big.Rat).Abs(v)
			if abs.Cmp(out.MaxAbsoluteResidual) > 0 {
				out.MaxAbsoluteResidual = abs
			}
		}
	}
	out.RadialDistanceUpper = new(big.Rat).Quo(out.MaxAbsoluteResidual, rr)
	return out, nil
}
func residualBits(q *big.Rat) bool { return q.Num().BitLen() <= 16384 && q.Denom().BitLen() <= 16384 }
func residualDeBoor(p int, poles [][3]*big.Rat, knots []*big.Rat, k int, t *big.Rat) ([3]*big.Rat, error) {
	d := make([][3]*big.Rat, p+1)
	for j := 0; j <= p; j++ {
		for c := 0; c < 3; c++ {
			d[j][c] = new(big.Rat).Set(poles[k-p+j][c])
		}
	}
	for r := 1; r <= p; r++ {
		for j := p; j >= r; j-- {
			i := k - p + j
			den := new(big.Rat).Sub(knots[i+p-r+1], knots[i])
			if den.Sign() == 0 {
				return [3]*big.Rat{}, geom.ErrDomain
			}
			alpha := new(big.Rat).Quo(new(big.Rat).Sub(t, knots[i]), den)
			for c := 0; c < 3; c++ {
				d[j][c].Add(new(big.Rat).Mul(new(big.Rat).Sub(big.NewRat(1, 1), alpha), d[j-1][c]), new(big.Rat).Mul(alpha, d[j][c]))
				if !residualBits(d[j][c]) {
					return [3]*big.Rat{}, geom.ErrDomain
				}
			}
		}
	}
	return d[p], nil
}
func residualBernsteinInverse(n int) ([][]*big.Rat, error) {
	a := make([][]*big.Rat, n+1)
	for i := 0; i <= n; i++ {
		a[i] = make([]*big.Rat, 2*(n+1))
		t := big.NewRat(int64(i), int64(n))
		u := new(big.Rat).Sub(big.NewRat(1, 1), t)
		for j := 0; j <= n; j++ {
			v := new(big.Rat).SetInt(new(big.Int).Binomial(int64(n), int64(j)))
			for k := 0; k < j; k++ {
				v.Mul(v, t)
			}
			for k := j; k < n; k++ {
				v.Mul(v, u)
			}
			a[i][j] = v
			a[i][n+1+j] = new(big.Rat)
			if i == j {
				a[i][n+1+j].SetInt64(1)
			}
		}
	}
	for col := 0; col <= n; col++ {
		pivot := col
		for pivot <= n && a[pivot][col].Sign() == 0 {
			pivot++
		}
		if pivot > n {
			return nil, geom.ErrDomain
		}
		a[pivot], a[col] = a[col], a[pivot]
		q := new(big.Rat).Set(a[col][col])
		for j := 0; j < 2*(n+1); j++ {
			a[col][j].Quo(a[col][j], q)
		}
		for i := 0; i <= n; i++ {
			if i == col {
				continue
			}
			q := new(big.Rat).Set(a[i][col])
			for j := 0; j < 2*(n+1); j++ {
				a[i][j].Sub(a[i][j], new(big.Rat).Mul(q, a[col][j]))
			}
		}
	}
	out := make([][]*big.Rat, n+1)
	for i := range out {
		out[i] = a[i][n+1:]
	}
	return out, nil
}
