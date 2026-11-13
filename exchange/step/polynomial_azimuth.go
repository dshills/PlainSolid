package step

import (
	"io"
	"math/big"
	"plainsolid/geom"
)

// PolynomialAzimuthEvidence bounds the numerator of angular velocity around
// the local positive Z axis through Origin. Positive proves strict increasing
// azimuth on every source span, never a particular winding count. Closure is
// reported independently using exact source endpoint coordinates. False sign
// flags indicate an unresolved Bernstein sign, not necessarily a reversal.
type PolynomialAzimuthEvidence struct {
	Spans                   []PolynomialSphereResidualSpan
	Positive, Negative      bool
	Closed, ProjectedClosed bool
	EndpointDistanceSquared *big.Rat
}

// BoundPolynomialSplineAzimuth bounds (x-origin.X)*y'-(y-origin.Y)*x' over
// complete source knot spans by exact Bernstein polynomial reconstruction.
// This additive evidence API currently uses model-space X/Y and positive Z.
// It shares the source/bit budgets of the support residual API. It does not
// prove turn count, trim coverage, inverse pcurve matching or body ownership.
func BoundPolynomialSplineAzimuth(r io.Reader, l Limits, curveID int64, origin geom.Vec3) (out PolynomialAzimuthEvidence, err error) {
	if !origin.IsFinite() {
		return out, geom.ErrDomain
	}
	p, poles, knots, e := readResidualPolynomial(r, l, curveID)
	if e != nil {
		return out, e
	}
	// The derivative curve uses trimmed knots and exact homogeneous-free poles.
	derivative := make([][3]*big.Rat, len(poles)-1)
	for i := range derivative {
		den := new(big.Rat).Sub(knots[i+p+1], knots[i+1])
		if den.Sign() == 0 {
			return out, geom.ErrDomain
		}
		scale := new(big.Rat).Quo(big.NewRat(int64(p), 1), den)
		for c := 0; c < 3; c++ {
			derivative[i][c] = new(big.Rat).Mul(new(big.Rat).Sub(poles[i+1][c], poles[i][c]), scale)
			if !residualBits(derivative[i][c]) {
				return PolynomialAzimuthEvidence{}, geom.ErrDomain
			}
		}
	}
	n := 2*p - 1
	inverse, e := residualBernsteinInverse(n)
	if e != nil {
		return out, e
	}
	ox, oy := new(big.Rat).SetFloat64(origin.X), new(big.Rat).SetFloat64(origin.Y)
	out.Positive, out.Negative = true, true
	for k := p; k < len(poles); k++ {
		a, b := knots[k], knots[k+1]
		if a.Cmp(b) == 0 {
			continue
		}
		values := make([]*big.Rat, n+1)
		for j := 0; j <= n; j++ {
			t := new(big.Rat).Add(a, new(big.Rat).Mul(new(big.Rat).Sub(b, a), big.NewRat(int64(j), int64(n))))
			v, e := residualDeBoor(p, poles, knots, k, t)
			if e != nil {
				return PolynomialAzimuthEvidence{}, e
			}
			d, e := residualDeBoor(p-1, derivative, knots[1:len(knots)-1], k-1, t)
			if e != nil {
				return PolynomialAzimuthEvidence{}, e
			}
			x, y := new(big.Rat).Sub(v[0], ox), new(big.Rat).Sub(v[1], oy)
			values[j] = new(big.Rat).Sub(new(big.Rat).Mul(x, d[1]), new(big.Rat).Mul(y, d[0]))
			if !residualBits(values[j]) {
				return PolynomialAzimuthEvidence{}, geom.ErrDomain
			}
		}
		var lo, hi *big.Rat
		for i := 0; i <= n; i++ {
			q := new(big.Rat)
			for j := 0; j <= n; j++ {
				q.Add(q, new(big.Rat).Mul(inverse[i][j], values[j]))
			}
			if !residualBits(q) {
				return PolynomialAzimuthEvidence{}, geom.ErrDomain
			}
			if lo == nil || q.Cmp(lo) < 0 {
				lo = new(big.Rat).Set(q)
			}
			if hi == nil || q.Cmp(hi) > 0 {
				hi = new(big.Rat).Set(q)
			}
		}
		out.Spans = append(out.Spans, PolynomialSphereResidualSpan{new(big.Rat).Set(a), new(big.Rat).Set(b), lo, hi})
		out.Positive = out.Positive && lo.Sign() > 0
		out.Negative = out.Negative && hi.Sign() < 0
	}
	out.Closed, out.ProjectedClosed = true, true
	out.EndpointDistanceSquared = new(big.Rat)
	for c := 0; c < 3; c++ {
		d := new(big.Rat).Sub(poles[len(poles)-1][c], poles[0][c])
		out.EndpointDistanceSquared.Add(out.EndpointDistanceSquared, new(big.Rat).Mul(d, d))
		if d.Sign() != 0 {
			out.Closed = false
			if c < 2 {
				out.ProjectedClosed = false
			}
		}
	}
	return out, nil
}
