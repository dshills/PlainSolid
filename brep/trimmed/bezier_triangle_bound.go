package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

type BezierTriangleBoundOptions struct{ MaxDegree, MaxOperations, MaxBits int }
type BezierTriangleDeviation struct {
	SquaredDistance *big.Rat
	BothDirections  bool
	Operations      int
}
type bezierTriangleBudget struct {
	ctx               context.Context
	used, limit, bits int
}

func (b *bezierTriangleBudget) check(q *big.Rat) error {
	b.used++
	if b.used > b.limit {
		return geom.ErrConvergence
	}
	if e := b.ctx.Err(); e != nil {
		return e
	}
	if q == nil {
		return geom.ErrInvalid
	}
	if q.Num().BitLen() > b.bits || q.Denom().BitLen() > b.bits {
		return geom.ErrConvergence
	}
	return nil
}
func (b *bezierTriangleBudget) addProduct(dst, a, c *big.Rat) error {
	x := new(big.Rat).Mul(a, c)
	if e := b.check(x); e != nil {
		return e
	}
	dst.Add(dst, x)
	return b.check(dst)
}

type trianglePower [][]*big.Rat

func makeTrianglePower(n int) trianglePower {
	p := make(trianglePower, n+1)
	for i := range p {
		p[i] = make([]*big.Rat, n+1-i)
		for j := range p[i] {
			p[i][j] = new(big.Rat)
		}
	}
	return p
}
func multiplyTrianglePower(a, c trianglePower, b *bezierTriangleBudget) (trianglePower, error) {
	n := len(a) - 1
	p := makeTrianglePower(n)
	for i := range a {
		for j, x := range a[i] {
			if x.Sign() == 0 {
				continue
			}
			for k := 0; k <= n-i-j; k++ {
				for l := 0; l <= n-i-j-k; l++ {
					if c[k][l].Sign() == 0 {
						continue
					}
					if e := b.addProduct(p[i+k][j+l], x, c[k][l]); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	return p, nil
}

// BoundRationalBezierTriangleDeviation bounds both directed distances through
// the exact common UV barycentric correspondence. It forms Hxyz-W*T exactly,
// converts that polynomial to triangular Bernstein controls, and divides the
// largest control norm by a proved positive denominator lower bound. Display
// targets may be arbitrary rational points, so all prior rounding is included.
// This certifies image distance, not surface or triangle embedding. Degree is
// bounded (default4,max6), arithmetic default2M operations/16384bits; failures
// return no distance certificate. Source must be one positive-weight span.
func BoundRationalBezierTriangleDeviation(ctx context.Context, p *geom.SurfacePatch, uv [3][2]*big.Rat, model [3][3]*big.Rat, o BezierTriangleBoundOptions) (out BezierTriangleDeviation, err error) {
	if ctx == nil || p == nil {
		return out, geom.ErrInvalid
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 4
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxDegree < 1 || o.MaxDegree > 6 || o.MaxOperations < 1 || o.MaxOperations > 100000000 || o.MaxBits < 64 || o.MaxBits > 65536 {
		return out, geom.ErrInvalid
	}
	budget := &bezierTriangleBudget{ctx: ctx, limit: o.MaxOperations, bits: o.MaxBits}
	defer func() {
		out.Operations = budget.used
		if err != nil {
			out.SquaredDistance = nil
			out.BothDirections = false
		}
	}()
	domain := p.Domain()
	if !finite(domain.UMin) || !finite(domain.UMax) || !finite(domain.VMin) || !finite(domain.VMax) {
		return out, geom.ErrInvalid
	}
	origin := [2]*big.Rat{ratInset(domain.UMin), ratInset(domain.VMin)}
	width := [2]*big.Rat{new(big.Rat).Sub(ratInset(domain.UMax), origin[0]), new(big.Rat).Sub(ratInset(domain.VMax), origin[1])}
	for axis := range width {
		if e := budget.check(origin[axis]); e != nil {
			return out, e
		}
		if e := budget.check(width[axis]); e != nil {
			return out, e
		}
		if width[axis].Sign() <= 0 {
			return out, geom.ErrInvalid
		}
	}
	for i := range uv {
		for axis, x := range uv[i] {
			if e := budget.check(x); e != nil {
				return out, e
			}
			if x.Cmp(origin[axis]) < 0 || x.Cmp(new(big.Rat).Add(origin[axis], width[axis])) > 0 {
				return out, geom.ErrDomain
			}
		}
		for _, x := range model[i] {
			if e := budget.check(x); e != nil {
				return out, e
			}
		}
	}
	c, minW, ok := p.RationalBezierCoefficients(o.MaxDegree)
	if !ok {
		return out, geom.ErrConvergence
	}
	// Exact homogeneous support, not the rectangular storage degree, controls
	// the required total degree after affine UV composition. Include one
	// extra degree only for W times the affine target. No nonzero term may
	// be truncated by multiplyTrianglePower's finite triangular storage.
	du, dv, n := 0, 0, 1
	for i, row := range c {
		for j, h := range row {
			for axis, q := range h {
				if e := budget.check(q); e != nil {
					return out, e
				}
				if q.Sign() == 0 {
					continue
				}
				if i > du {
					du = i
				}
				if j > dv {
					dv = j
				}
				degree := i + j
				if axis == 3 {
					degree++
				}
				if degree > n {
					n = degree
				}
			}
		}
	}
	powers := [2][]trianglePower{}
	for axis, degree := range []int{du, dv} {
		linear := makeTrianglePower(n)
		values := [3]*big.Rat{}
		for i := range uv {
			values[i] = new(big.Rat).Quo(new(big.Rat).Sub(uv[i][axis], origin[axis]), width[axis])
			if e := budget.check(values[i]); e != nil {
				return out, e
			}
		}
		linear[0][0].Set(values[0])
		linear[1][0].Sub(values[1], values[0])
		linear[0][1].Sub(values[2], values[0])
		powers[axis] = make([]trianglePower, degree+1)
		powers[axis][0] = makeTrianglePower(n)
		powers[axis][0][0][0].SetInt64(1)
		for i := 1; i <= degree; i++ {
			v, e := multiplyTrianglePower(powers[axis][i-1], linear, budget)
			if e != nil {
				return out, e
			}
			powers[axis][i] = v
		}
	}
	var h [4]trianglePower
	for k := range h {
		h[k] = makeTrianglePower(n)
	}
	for i := range c {
		for j := range c[i] {
			nonzero := false
			for _, v := range c[i][j] {
				if e := budget.check(v); e != nil {
					return out, e
				}
				nonzero = nonzero || v.Sign() != 0
			}
			if !nonzero {
				continue
			}
			term, e := multiplyTrianglePower(powers[0][i], powers[1][j], budget)
			if e != nil {
				return out, e
			}
			for a := range term {
				for z, v := range term[a] {
					if v.Sign() == 0 {
						continue
					}
					for k := 0; k < 4; k++ {
						if c[i][j][k].Sign() != 0 {
							if e := budget.addProduct(h[k][a][z], v, c[i][j][k]); e != nil {
								return out, e
							}
						}
					}
				}
			}
		}
	}
	var residual [3]trianglePower
	for k := 0; k < 3; k++ {
		target := makeTrianglePower(n)
		target[0][0].Set(model[0][k])
		target[1][0].Sub(model[1][k], model[0][k])
		target[0][1].Sub(model[2][k], model[0][k])
		product, e := multiplyTrianglePower(h[3], target, budget)
		if e != nil {
			return out, e
		}
		residual[k] = makeTrianglePower(n)
		for i := range product {
			for j := range product[i] {
				residual[k][i][j].Sub(h[k][i][j], product[i][j])
				if e := budget.check(residual[k][i][j]); e != nil {
					return out, e
				}
			}
		}
	}
	falling := func(n, k int) int64 {
		x := int64(1)
		for i := 0; i < k; i++ {
			x *= int64(n - i)
		}
		return x
	}
	bound := new(big.Rat)
	for i := 0; i <= n; i++ {
		for j := 0; j <= n-i; j++ {
			control := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
			for a := 0; a <= i; a++ {
				for z := 0; z <= j; z++ {
					factor := big.NewRat(falling(i, a)*falling(j, z), falling(n, a+z))
					for k := range control {
						if residual[k][a][z].Sign() != 0 {
							if e := budget.addProduct(control[k], residual[k][a][z], factor); e != nil {
								return out, e
							}
						}
					}
				}
			}
			squared := new(big.Rat)
			for _, x := range control {
				if e := budget.addProduct(squared, x, x); e != nil {
					return out, e
				}
			}
			if squared.Cmp(bound) > 0 {
				bound.Set(squared)
			}
		}
	}
	if e := budget.check(minW); e != nil {
		return out, e
	}
	bound.Quo(bound, new(big.Rat).Mul(minW, minW))
	if e := budget.check(bound); e != nil {
		return out, e
	}
	out.SquaredDistance = bound
	out.BothDirections = true
	return out, nil
}
