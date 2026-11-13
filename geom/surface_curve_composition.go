package geom

import (
	"context"
	"math/big"
)

// ComposeParameterCurve composes a single rational Bézier UV curve with a
// single rational tensor Bézier patch without rounding any coefficient. UV is
// carried in curve X/Y, with identically zero Z. The curve's original parameter
// domain is retained. Degree is at most curveDegree*(surfaceUDegree+surfaceVDegree);
// exact common homogeneous factors are cancelled when positive weights remain.
// Positive output weights are proved by requiring every input UV control point
// inside the closed surface rectangle. A curve whose control hull exceeds the
// rectangle remains unsupported even if its actual image lies inside; exact
// subdivision by the caller can establish smaller supported arcs. Degree >64,
// multiple spans, and budget exhaustion return ErrConvergence.
func (p *SurfacePatch) ComposeParameterCurve(ctx context.Context, curve *ExactSplineCurve, o SurfacePatchOptions) (*ExactSplineCurve, error) {
	if ctx == nil || p == nil || len(p.net) == 0 || curve == nil || curve.patch == nil {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	o, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	c := curve.patch
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || c.countU != c.degreeU+1 {
		return nil, ErrConvergence
	}
	degree := c.degreeU * (p.degreeU + p.degreeV)
	if degree < 1 || degree > 64 || degree+1 > o.MaxPoles/2 || len(p.net) > o.MaxPoles || len(c.net) > o.MaxPoles {
		return nil, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	d := p.Domain()
	origins := [2]*big.Rat{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.VMin)}
	widths := [2]*big.Rat{new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), origins[0]), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), origins[1])}
	var coordinates, complements [2][]*big.Rat
	for axis := 0; axis < 2; axis++ {
		coordinates[axis] = make([]*big.Rat, c.countU)
		complements[axis] = make([]*big.Rat, c.countU)
		for i := 0; i < c.countU; i++ {
			h := c.net[i*c.countV]
			if h[2].Sign() != 0 {
				return nil, ErrInvalid
			}
			value := new(big.Rat).Quo(new(big.Rat).Sub(h[axis], new(big.Rat).Mul(origins[axis], h[3])), widths[axis])
			other := new(big.Rat).Sub(h[3], value)
			if value.Sign() < 0 || other.Sign() < 0 {
				return nil, ErrConvergence
			}
			for _, x := range []*big.Rat{value, other, h[3]} {
				if e := budget.check(x); e != nil {
					return nil, e
				}
			}
			coordinates[axis][i] = value
			complements[axis][i] = other
		}
	}
	var basis [2][][]*big.Rat
	for axis, n := range []int{p.degreeU, p.degreeV} {
		powers, others := make([][]*big.Rat, n+1), make([][]*big.Rat, n+1)
		powers[0] = []*big.Rat{big.NewRat(1, 1)}
		others[0] = []*big.Rat{big.NewRat(1, 1)}
		for i := 1; i <= n; i++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			powers[i], e = exactBernsteinScalarProduct(powers[i-1], coordinates[axis], budget)
			if e != nil {
				return nil, e
			}
			others[i], e = exactBernsteinScalarProduct(others[i-1], complements[axis], budget)
			if e != nil {
				return nil, e
			}
		}
		basis[axis] = make([][]*big.Rat, n+1)
		for i := 0; i <= n; i++ {
			basis[axis][i], e = exactBernsteinScalarProduct(powers[i], others[n-i], budget)
			if e != nil {
				return nil, e
			}
			factor := new(big.Rat).SetInt(new(big.Int).Binomial(int64(n), int64(i)))
			for _, x := range basis[axis][i] {
				x.Mul(x, factor)
				if e := budget.check(x); e != nil {
					return nil, e
				}
			}
		}
	}
	net := make([]exactHomogeneous, degree+1)
	for i := range net {
		for axis := 0; axis < 4; axis++ {
			net[i][axis] = new(big.Rat)
		}
	}
	for i := 0; i < p.countU; i++ {
		for j := 0; j < p.countV; j++ {
			if e := ctx.Err(); e != nil {
				return nil, e
			}
			coefficient, e := exactBernsteinScalarProduct(basis[0][i], basis[1][j], budget)
			if e != nil {
				return nil, e
			}
			h := p.net[i*p.countV+j]
			for k, x := range coefficient {
				for axis := 0; axis < 4; axis++ {
					net[k][axis].Add(net[k][axis], new(big.Rat).Mul(h[axis], x))
					if e := budget.check(net[k][axis]); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	net, e = reduceComposedBezier(net, budget)
	if e != nil {
		return nil, e
	}
	degree = len(net) - 1
	result, e := exactCurveFromHomogeneousBezier(net, TubeAffineLineOptions{MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}, budget)
	if e != nil {
		return nil, e
	}
	lo, hi := curve.Domain()
	for i := range result.patch.knotsU {
		if i <= degree {
			result.patch.knotsU[i] = lo
		} else {
			result.patch.knotsU[i] = hi
		}
	}
	result.operations = budget.operations
	result.patch.operations = budget.operations
	return result, nil
}
func exactBernsteinScalarProduct(a, c []*big.Rat, b *polynomialBudget) ([]*big.Rat, error) {
	m, n := len(a)-1, len(c)-1
	out := make([]*big.Rat, m+n+1)
	for i := range out {
		out[i] = new(big.Rat)
	}
	for i, x := range a {
		for j, y := range c {
			factor := new(big.Rat).SetFrac(new(big.Int).Mul(new(big.Int).Binomial(int64(m), int64(i)), new(big.Int).Binomial(int64(n), int64(j))), new(big.Int).Binomial(int64(m+n), int64(i+j)))
			out[i+j].Add(out[i+j], new(big.Rat).Mul(new(big.Rat).Mul(x, y), factor))
			if e := b.check(out[i+j]); e != nil {
				return nil, e
			}
		}
	}
	return out, nil
}

// reduceComposedBezier removes exact common polynomial factors and redundant
// elevation, but only adopts the reduction if all resulting weights have one
// strict sign. Otherwise the original positive-weight net is retained.
func reduceComposedBezier(net []exactHomogeneous, b *polynomialBudget) ([]exactHomogeneous, error) {
	var polys [4]rationalPolynomial
	var common rationalPolynomial
	for axis := 0; axis < 4; axis++ {
		values := make([]*big.Rat, len(net))
		for i := range values {
			values[i] = net[i][axis]
		}
		var e error
		polys[axis], e = bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
		if len(polys[axis]) == 0 {
			continue
		}
		if common == nil {
			common = polys[axis]
		} else {
			common, e = polynomialGCD(common, polys[axis], b)
			if e != nil {
				return nil, e
			}
		}
	}
	degree := 1
	for axis := 0; axis < 4; axis++ {
		if len(common) > 1 {
			q, rem, e := polynomialDivide(polys[axis], common, b)
			if e != nil {
				return nil, e
			}
			if len(rem) != 0 {
				return nil, ErrConvergence
			}
			polys[axis] = q
		}
		degree = max(degree, len(polys[axis])-1)
	}
	if degree >= len(net)-1 {
		return net, nil
	}
	out := make([]exactHomogeneous, degree+1)
	for i := range out {
		for axis := 0; axis < 4; axis++ {
			out[i][axis] = new(big.Rat)
			for k, c := range polys[axis] {
				if k > i {
					break
				}
				factor := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(degree), int64(k)))
				out[i][axis].Add(out[i][axis], new(big.Rat).Mul(c, factor))
				if e := b.check(out[i][axis]); e != nil {
					return nil, e
				}
			}
		}
	}
	sign := out[0][3].Sign()
	if sign == 0 {
		return net, nil
	}
	for _, h := range out {
		if h[3].Sign() != sign {
			return net, nil
		}
	}
	if sign < 0 {
		for _, h := range out {
			for _, x := range h {
				x.Neg(x)
			}
		}
	}
	return out, nil
}
