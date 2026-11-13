package geom

import "math/big"

// exactSpanLengthBounds selects a rigorous geometric enclosure for a positive
// rational quadratic Bezier span. Write its curve as
// P0 + alpha(t)(P1-P0) + beta(t)(P2-P1), where alpha=1-R0 and beta=R2.
// For positive weights both alpha and beta are monotone from0 to1:
// alpha'=2*w0*(1-t)*(w1*(1-t)+w2*t)/D² >=0 and
// beta'=2*w2*t*(w0*(1-t)+w1*t)/D² >=0.
// Integrating the triangle inequality therefore bounds length by the two-edge
// Euclidean control polygon, independently of weights and curvature. The chord
// is a lower bound. Exact homogeneous subdivision yields convergent bounds.
// Other degrees/representations retain the conservative derivative enclosure.
func exactSpanLengthBounds(p *SurfacePatch, b *polynomialBudget) (float64, float64, error) {
	if p.degreeU != 2 || p.countU != 3 {
		return spanLengthBounds(p, b)
	}
	var poles [3][3]*big.Rat
	for i := 0; i < 3; i++ {
		h := p.net[i*p.countV]
		if h[3].Sign() <= 0 {
			return 0, 0, ErrInvalid
		}
		for k := 0; k < 3; k++ {
			poles[i][k] = new(big.Rat).Quo(h[k], h[3])
			if e := b.check(poles[i][k]); e != nil {
				return 0, 0, e
			}
		}
	}
	squared := func(i, j int) (*big.Rat, error) {
		s := new(big.Rat)
		for k := 0; k < 3; k++ {
			d := new(big.Rat).Sub(poles[i][k], poles[j][k])
			s.Add(s, new(big.Rat).Mul(d, d))
			if e := b.check(s); e != nil {
				return nil, e
			}
		}
		return s, nil
	}
	chord, e := squared(0, 2)
	if e != nil {
		return 0, 0, e
	}
	a, e := squared(0, 1)
	if e != nil {
		return 0, 0, e
	}
	c, e := squared(1, 2)
	if e != nil {
		return 0, 0, e
	}
	left, right := outwardSqrtRat(a, false), outwardSqrtRat(c, false)
	if !finite(left) || !finite(right) {
		return spanLengthBounds(p, b)
	}
	upper := new(big.Rat).Add(new(big.Rat).SetFloat64(left), new(big.Rat).SetFloat64(right))
	if e := b.check(upper); e != nil {
		return 0, 0, e
	}
	return outwardSqrtRat(chord, true), rationalFloatBound(upper, false), nil
}
