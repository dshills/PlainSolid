package geom

import (
	"context"
	"math/big"
)

// RationalProjectedJacobianZero proves whether the projected signed Jacobian
// vanishes identically on a positive-weight single-span tensor patch. The
// proof constructs the polynomial numerator over the original chart. A nonzero
// numerator proves that the Jacobian does not vanish identically.
func (p *SurfacePatch) RationalProjectedJacobianZero(ctx context.Context, axes [2]int, o SurfacePatchEvaluationOptions) (zero bool, operations int, err error) {
	if ctx == nil || p == nil || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return false, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return false, 0, e
	}
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || len(p.net) == 0 || p.degreeU > 6 || p.degreeV > 6 {
		return false, 0, ErrUnrepresentable
	}
	lim, e := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return false, 0, e
	}
	b := &polynomialBudget{maxOperations: lim.MaxOperations, maxBits: lim.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	type power [][]*big.Rat
	makePower := func(n, m int) power {
		a := make(power, n)
		for i := range a {
			a[i] = make([]*big.Rat, m)
			for j := range a[i] {
				a[i][j] = new(big.Rat)
			}
		}
		return a
	}
	var h [3]power
	nu, nv := p.countU, p.countV
	for k, a := range [3]int{axes[0], axes[1], 3} {
		h[k] = makePower(nu, nv)
		row := make([]rationalPolynomial, nu)
		for i := range row {
			control := make([]*big.Rat, nv)
			for j := range control {
				control[j] = p.net[i*nv+j][a]
				if a == 3 && control[j].Sign() <= 0 {
					return false, 0, ErrInvalid
				}
			}
			row[i], e = bernsteinToPower(control, b)
			if e != nil {
				return false, 0, e
			}
		}
		for j := range nv {
			control := make([]*big.Rat, nu)
			for i := range control {
				control[i] = new(big.Rat)
				if j < len(row[i]) {
					control[i].Set(row[i][j])
				}
			}
			column, e := bernsteinToPower(control, b)
			if e != nil {
				return false, 0, e
			}
			for i, x := range column {
				h[k][i][j] = x
			}
			if e := ctx.Err(); e != nil {
				return false, 0, e
			}
		}
	}
	// A rate numerator is (dX)*W - X*(dW). Compute it as a single
	// antisymmetric product so exact cancellations happen before the
	// determinant products. The denominator of either rate is W².
	var rate [2][2]power
	for dim := range rate {
		rate[dim][0], rate[dim][1] = makePower(2*nu-2, 2*nv-1), makePower(2*nu-1, 2*nv-2)
		for i := range nu {
			for j := range nv {
				for k := range nu {
					for l := range nv {
						if i == 0 && j == 0 {
							continue
						}
						x := new(big.Rat).Sub(new(big.Rat).Mul(h[dim][i][j], h[2][k][l]), new(big.Rat).Mul(h[2][i][j], h[dim][k][l]))
						if i > 0 {
							q := rate[dim][0][i+k-1][j+l]
							q.Add(q, new(big.Rat).Mul(x, big.NewRat(int64(i), 1)))
							if e := b.check(q); e != nil {
								return false, 0, e
							}
						}
						if j > 0 {
							q := rate[dim][1][i+k][j+l-1]
							q.Add(q, new(big.Rat).Mul(x, big.NewRat(int64(j), 1)))
							if e := b.check(q); e != nil {
								return false, 0, e
							}
						}
					}
				}
			}
			if e := ctx.Err(); e != nil {
				return false, 0, e
			}
		}
	}
	det := makePower(4*nu-4, 4*nv-4)
	for i, row := range rate[0][0] {
		for j, x := range row {
			y := rate[1][0][i][j]
			if x.Sign() == 0 && y.Sign() == 0 {
				continue
			}
			for k, other := range rate[1][1] {
				for l, z := range other {
					v := new(big.Rat).Sub(new(big.Rat).Mul(x, z), new(big.Rat).Mul(y, rate[0][1][k][l]))
					q := det[i+k][j+l]
					q.Add(q, v)
					if e := b.check(q); e != nil {
						return false, 0, e
					}
				}
			}
		}
		if e := ctx.Err(); e != nil {
			return false, 0, e
		}
	}
	for _, row := range det {
		for _, v := range row {
			if v.Sign() != 0 {
				return false, 0, nil
			}
		}
	}
	return true, 0, nil
}
