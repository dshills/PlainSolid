package geom

import "math/big"

// SurfacePlaneTangency is an isolated point of contact certified by a
// definite quadratic homogeneous plane numerator. Side is +1 or -1 for
// the strict plane sign elsewhere on Domain. UV and Bounds enclose the
// exact point; the position may not be representable as float64.
type SurfacePlaneTangency struct {
	Domain ParameterRectangle
	UV     UVBox
	Bounds Box
	Side   int
}

// definiteQuadraticTangency uses a complete-square argument in normalized UV.
// A definite Hessian and exact zero at its interior critical point exclude all
// other roots in the cell, without an epsilon or a sampled multiplicity test.
func definiteQuadraticTangency(p *SurfacePatch, numerator []*big.Rat, b *polynomialBudget) (SurfacePlaneTangency, bool, error) {
	var result SurfacePlaneTangency
	if p.degreeU != 2 || p.degreeV != 2 || len(numerator) != 9 {
		return result, false, nil
	}
	var v [3]rationalPolynomial
	for j := range 3 {
		q, e := bernsteinToPower([]*big.Rat{numerator[j], numerator[j+3], numerator[j+6]}, b)
		if e != nil {
			return result, false, e
		}
		v[j] = q
	}
	coefficient := func(p rationalPolynomial, i int) *big.Rat {
		if i >= len(p) {
			return new(big.Rat)
		}
		return p[i]
	}
	var q [3]rationalPolynomial
	for i := range 3 {
		r, e := bernsteinToPower([]*big.Rat{coefficient(v[0], i), coefficient(v[1], i), coefficient(v[2], i)}, b)
		if e != nil {
			return result, false, e
		}
		q[i] = r
		if len(r) > 3-i {
			return result, false, nil
		}
	}
	// f=c+du*u+dv*v+a*u²+cross*u*v+d*v².
	c, du, dv := coefficient(q[0], 0), coefficient(q[1], 0), coefficient(q[0], 1)
	a, cross, d := coefficient(q[2], 0), coefficient(q[1], 1), coefficient(q[0], 2)
	aa, dd := new(big.Rat).Mul(a, big.NewRat(2, 1)), new(big.Rat).Mul(d, big.NewRat(2, 1))
	det := new(big.Rat).Sub(new(big.Rat).Mul(aa, dd), new(big.Rat).Mul(cross, cross))
	if e := b.check(det); e != nil {
		return result, false, e
	}
	if det.Sign() <= 0 || a.Sign() == 0 {
		return result, false, nil
	}
	solve := func(a, b, c *big.Rat) *big.Rat {
		return new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(cross, a), new(big.Rat).Mul(b, c)), det)
	}
	u, v0 := solve(dv, dd, du), solve(du, aa, dv)
	for _, x := range []*big.Rat{u, v0} {
		if e := b.check(x); e != nil {
			return result, false, e
		}
		if x.Sign() <= 0 || x.Cmp(big.NewRat(1, 1)) >= 0 {
			return result, false, nil
		}
	}
	// Stationarity gives f(uv)=c+(du*u+dv*v)/2 exactly.
	f := new(big.Rat).Add(new(big.Rat).Mul(du, u), new(big.Rat).Mul(dv, v0))
	f.Mul(f, big.NewRat(1, 2)).Add(f, c)
	if e := b.check(f); e != nil {
		return result, false, e
	}
	if f.Sign() != 0 {
		return result, false, nil
	}
	domain := p.Domain()
	x := parameterMap(ParameterSpan{Start: domain.UMin, End: domain.UMax}, u)
	y := parameterMap(ParameterSpan{Start: domain.VMin, End: domain.VMax}, v0)
	pt, _, e := exactSurfaceJet(p, x, y, b)
	if e != nil {
		return result, false, e
	}
	result = SurfacePlaneTangency{
		Domain: domain, Side: a.Sign(),
		UV: UVBox{Vec2{rationalFloatBound(x, true), rationalFloatBound(y, true)}, Vec2{rationalFloatBound(x, false), rationalFloatBound(y, false)}},
		Bounds: Box{Vec3{rationalFloatBound(pt[0], true), rationalFloatBound(pt[1], true), rationalFloatBound(pt[2], true)},
			Vec3{rationalFloatBound(pt[0], false), rationalFloatBound(pt[1], false), rationalFloatBound(pt[2], false)}},
	}
	return result, true, nil
}
