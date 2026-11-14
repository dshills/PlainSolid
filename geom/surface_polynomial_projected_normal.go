package geom

import (
	"context"
	"math/big"
)

// ProjectedJacobianZero proves whether a single polynomial Bézier patch has
// identically zero signed Jacobian in the selected projection. Its normalized
// chart coordinates suffice for the zero test. A varying weight or multiple
// knot spans are outside this exact certificate.
func (p *SurfacePatch) ProjectedJacobianZero(ctx context.Context, axes [2]int, o SurfacePatchEvaluationOptions) (zero bool, operations int, err error) {
	if ctx == nil || p == nil || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] {
		return false, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return false, 0, e
	}
	lim, e := patchOptions(SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return false, 0, e
	}
	b := &polynomialBudget{maxOperations: lim.MaxOperations, maxBits: lim.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	if p.countU != p.degreeU+1 || p.countV != p.degreeV+1 || len(p.net) == 0 || p.degreeU > 6 || p.degreeV > 6 {
		return false, 0, ErrUnrepresentable
	}
	u, v := p.countU, p.countV
	w := p.net[0][3]
	for _, h := range p.net {
		if e := b.check(h[3]); e != nil {
			return false, 0, e
		}
		if h[3].Cmp(w) != 0 {
			return false, 0, ErrUnrepresentable
		}
	}
	// Convert once to exact power coefficients, charging the hull-to-power work.
	coef := [2][]rationalPolynomial{}
	for k, a := range axes {
		rows := make([]rationalPolynomial, u)
		for i := range rows {
			h := make([]*big.Rat, v)
			for j := range h {
				h[j] = p.net[i*v+j][a]
			}
			rows[i], e = bernsteinToPower(h, b)
			if e != nil {
				return false, 0, e
			}
		}
		coef[k] = make([]rationalPolynomial, u)
		for j := 0; j < v; j++ {
			h := make([]*big.Rat, u)
			for i := range h {
				h[i] = new(big.Rat)
				if j < len(rows[i]) {
					h[i] = rows[i][j]
				}
			}
			power, e := bernsteinToPower(h, b)
			if e != nil {
				return false, 0, e
			}
			for i := 0; i < u; i++ {
				x := new(big.Rat)
				if i < len(power) {
					x = power[i]
				}
				coef[k][i] = append(coef[k][i], x)
			}
		}
	}
	det := make([][]*big.Rat, 2*u-2)
	for i := range det {
		det[i] = make([]*big.Rat, 2*v-2)
		for j := range det[i] {
			det[i][j] = new(big.Rat)
		}
	}
	for i := 1; i < u; i++ {
		for j := 0; j < v; j++ {
			for m := 0; m < u; m++ {
				for n := 1; n < v; n++ {
					x := new(big.Rat).Mul(coef[0][i][j], coef[1][m][n])
					y := new(big.Rat).Mul(coef[1][i][j], coef[0][m][n])
					x.Sub(x, y).Mul(x, big.NewRat(int64(i*n), 1))
					q := det[i+m-1][j+n-1]
					q.Add(q, x)
					if e := b.check(q); e != nil {
						return false, 0, e
					}
				}
			}
			if e := ctx.Err(); e != nil {
				return false, 0, e
			}
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
