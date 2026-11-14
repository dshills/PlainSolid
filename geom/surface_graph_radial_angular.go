package geom

import (
	"context"
	"math/big"
)

// Dense exact power coefficients indexed by original world X,Y exponents.
// The original charts prove X=u,Y=v; no numerical coordinate inversion occurs.
type radialPower [][]*big.Rat

func newRadialPower(n, m int) radialPower {
	p := make(radialPower, n)
	for i := range p {
		p[i] = make([]*big.Rat, m)
		for j := range p[i] {
			p[i][j] = new(big.Rat)
		}
	}
	return p
}

func radialHeightPowers(ctx context.Context, r *SplineGraphRadialContours, b *polynomialBudget) ([2]radialPower, error) {
	var out [2]radialPower
	for k, p := range r.supports {
		if p == nil || p.degreeU > 8 || p.degreeV > 8 || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
			return [2]radialPower{}, ErrConvergence
		}
		w := p.net[0][3]
		if w.Sign() <= 0 {
			return [2]radialPower{}, ErrConvergence
		}
		rows := make([]rationalPolynomial, p.countU)
		for i := range rows {
			if e := ctx.Err(); e != nil {
				return [2]radialPower{}, e
			}
			values := make([]*big.Rat, p.countV)
			for j := range values {
				h := p.net[i*p.countV+j]
				if h[3].Cmp(w) != 0 {
					return [2]radialPower{}, ErrConvergence
				}
				for _, q := range []*big.Rat{h[2], h[3]} {
					if e := b.check(q); e != nil {
						return [2]radialPower{}, e
					}
				}
				values[j] = new(big.Rat).Quo(h[2], w)
				if e := b.check(values[j]); e != nil {
					return [2]radialPower{}, e
				}
			}
			var e error
			rows[i], e = bernsteinToPower(values, b)
			if e != nil {
				return [2]radialPower{}, e
			}
		}
		out[k] = newRadialPower(p.countU, p.countV)
		for j := 0; j < p.countV; j++ {
			values := make([]*big.Rat, p.countU)
			for i := range values {
				values[i] = new(big.Rat)
				if j < len(rows[i]) {
					values[i].Set(rows[i][j])
				}
			}
			column, e := bernsteinToPower(values, b)
			if e != nil {
				return [2]radialPower{}, e
			}
			for i, c := range column {
				out[k][i][j].Set(c)
			}
		}
	}
	return out, nil
}

func radialPowerAdd(a, c radialPower, b *polynomialBudget) (radialPower, error) {
	out := newRadialPower(max(len(a), len(c)), max(len(a[0]), len(c[0])))
	for i := range out {
		for j := range out[i] {
			if i < len(a) && j < len(a[i]) {
				out[i][j].Set(a[i][j])
			}
			if i < len(c) && j < len(c[i]) {
				out[i][j].Add(out[i][j], c[i][j])
			}
			if e := b.check(out[i][j]); e != nil {
				return nil, e
			}
		}
	}
	return out, nil
}

func radialPowerMultiply(ctx context.Context, a, c radialPower, b *polynomialBudget) (radialPower, error) {
	n, m := len(a)+len(c)-1, len(a[0])+len(c[0])-1
	if n > 17 || m > 17 {
		return nil, ErrConvergence
	}
	out := newRadialPower(n, m)
	for i, row := range a {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for j, x := range row {
			if x.Sign() == 0 {
				continue
			}
			for k, col := range c {
				for l, y := range col {
					if y.Sign() == 0 {
						continue
					}
					term := new(big.Rat).Mul(x, y)
					if e := b.check(term); e != nil {
						return nil, e
					}
					out[i+k][j+l].Add(out[i+k][j+l], term)
					if e := b.check(out[i+k][j+l]); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	return out, nil
}

// radialAngularPower gives the exact circular mean in q=radius^2. The bounded
// products have degree<=16 in each world axis; therefore mean degree<=16.
// mean(cos^(2m) sin^(2n))=C(2m,m)C(2n,n)/(4^(m+n)C(m+n,m)).
func radialAngularPower(ctx context.Context, p radialPower, center [2]*big.Rat, b *polynomialBudget) (rationalPolynomial, error) {
	nu, nv := len(p), len(p[0])
	if nu > 17 || nv > 17 {
		return nil, ErrConvergence
	}
	mean := make(rationalPolynomial, (nu+nv-2)/2+1)
	for i := range mean {
		mean[i] = new(big.Rat)
	}
	var powers [2]rationalPolynomial
	for axis, n := range []int{nu, nv} {
		powers[axis] = make(rationalPolynomial, n)
		powers[axis][0] = big.NewRat(1, 1)
		for i := 1; i < n; i++ {
			powers[axis][i] = new(big.Rat).Mul(powers[axis][i-1], center[axis])
			if e := b.check(powers[axis][i]); e != nil {
				return nil, e
			}
		}
	}
	for i, row := range p {
		for j, c := range row {
			if c.Sign() == 0 {
				continue
			}
			for a := 0; a <= i; a += 2 {
				for d := 0; d <= j; d += 2 {
					if e := ctx.Err(); e != nil {
						return nil, e
					}
					m, n := int64(a/2), int64(d/2)
					numerator := new(big.Int).Mul(new(big.Int).Binomial(2*m, m), new(big.Int).Binomial(2*n, n))
					denominator := new(big.Int).Mul(new(big.Int).Lsh(big.NewInt(1), uint(2*(m+n))), new(big.Int).Binomial(m+n, m))
					term := new(big.Rat).SetFrac(numerator, denominator)
					for _, factor := range []*big.Rat{c, new(big.Rat).SetInt(new(big.Int).Binomial(int64(i), int64(a))), new(big.Rat).SetInt(new(big.Int).Binomial(int64(j), int64(d))), powers[0][i-a], powers[1][j-d]} {
						term.Mul(term, factor)
						if e := b.check(term); e != nil {
							return nil, e
						}
					}
					mean[(a+d)/2].Add(mean[(a+d)/2], term)
					if e := b.check(mean[(a+d)/2]); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	return trimPolynomial(mean), nil
}
