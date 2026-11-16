package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

type parabolaCircleMeshHeight struct {
	coefficients  [9][9]*big.Rat
	degree        [2]int
	slope         [2]*big.Rat
	hessian       [3]*big.Rat // uu, uv, vv
	normalHessian [3]*big.Rat // Bernstein bounds for the optional turn proof
}

func parabolaCircleDisplayHeight(ctx context.Context, s *geom.BSplineSurface) (parabolaCircleMeshHeight, *geom.SurfacePatch, error) {
	var h parabolaCircleMeshHeight
	p, err := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if err != nil {
		return h, nil, err
	}
	if p.Domain() != (geom.ParameterRectangle{UMax: 1, VMax: 1}) {
		return h, nil, geom.ErrConvergence
	}
	if err := requireIdentityPatchXY(ctx, p, geom.SurfacePatchOptions{}); err != nil {
		return h, nil, err
	}
	c, _, ok := p.RationalBezierCoefficients(8)
	if !ok || c[0][0][3].Sign() <= 0 {
		return h, nil, geom.ErrConvergence
	}
	for i := range h.slope {
		h.slope[i] = new(big.Rat)
	}
	for i := range h.hessian {
		h.hessian[i] = new(big.Rat)
	}
	derivatives := [5]*big.Rat{h.slope[0], h.slope[1], h.hessian[0], h.hessian[1], h.hessian[2]}
	for u, row := range c {
		for v, x := range row {
			if (u != 0 || v != 0) && x[3].Sign() != 0 {
				return parabolaCircleMeshHeight{}, nil, geom.ErrConvergence
			}
			if x[2].Sign() == 0 {
				continue
			}
			q := new(big.Rat).Quo(x[2], c[0][0][3])
			h.coefficients[u][v] = q
			h.degree[0], h.degree[1] = max(h.degree[0], u), max(h.degree[1], v)
			q = new(big.Rat).Abs(q)
			for i, v := range []int{u, v, u * (u - 1), u * v, v * (v - 1)} {
				dst := derivatives[i]
				dst.Add(dst, new(big.Rat).Mul(q, big.NewRat(int64(v), 1)))
			}
		}
	}
	return h, p, nil
}

func (h parabolaCircleMeshHeight) evaluate(uv [2]*big.Rat) *big.Rat {
	value, row := new(big.Rat), new(big.Rat)
	for u := h.degree[0]; u >= 0; u-- {
		row.SetInt64(0)
		for v := h.degree[1]; v >= 0; v-- {
			row.Mul(row, uv[1])
			if x := h.coefficients[u][v]; x != nil {
				row.Add(row, x)
			}
		}
		value.Mul(value, uv[0])
		value.Add(value, row)
	}
	return value
}

func (h parabolaCircleMeshHeight) residual(du, dv *big.Rat) *big.Rat {
	// Taylor expansion about the barycenter. The weighted variance of
	// each coordinate is at most the squared range divided by four.
	r := new(big.Rat).Mul(h.hessian[0], new(big.Rat).Mul(du, du))
	q := new(big.Rat).Mul(h.hessian[1], new(big.Rat).Mul(du, dv))
	r.Add(r, q.Add(q, q))
	r.Add(r, new(big.Rat).Mul(h.hessian[2], new(big.Rat).Mul(dv, dv)))
	return r.Quo(r, big.NewRat(8, 1))
}

func parabolaCircleMeshSourceVolumes(h [2]parabolaCircleMeshHeight,
	source *geom.SplineGraphParabolaTwoCircles) [4][2]*big.Rat {
	// All native charts use exact identity XY. Result regions are the two
	// unit strips and the original circle interiors, in source order.
	free := source.FreeAxis
	p := [3]*big.Rat{}
	for i, c := range source.ParabolaControls {
		p[i] = c[1-free]
	}
	one := big.NewRat(1, 1)
	a := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(p[1], p[0]))
	b := new(big.Rat).Sub(new(big.Rat).Sub(p[2], p[0]), a)
	parabolaPowers := [10][]*big.Rat{{one}}
	for i := 1; i < len(parabolaPowers); i++ {
		previous := parabolaPowers[i-1]
		row := make([]*big.Rat, len(previous)+2)
		for j := range row {
			row[j] = new(big.Rat)
		}
		for j, x := range previous {
			for k, y := range [3]*big.Rat{p[0], a, b} {
				row[j+k].Add(row[j+k], new(big.Rat).Mul(x, y))
			}
		}
		parabolaPowers[i] = row
	}
	lower, unit := new(big.Rat), new(big.Rat)
	var disk [2]*big.Rat
	for i := range disk {
		disk[i] = new(big.Rat)
	}
	for side, height := range h {
		for u, row := range height.coefficients {
			for v, coefficient := range row {
				if coefficient == nil {
					continue
				}
				q := new(big.Rat).Set(coefficient)
				if side == 1 {
					q.Neg(q)
				}
				x, y := u, v
				if free == 1 {
					x, y = v, u
				}
				for k, c := range parabolaPowers[y+1] {
					lower.Add(lower, new(big.Rat).Mul(q, new(big.Rat).Quo(c, big.NewRat(int64((y+1)*(x+k+1)), 1))))
				}
				unit.Add(unit, new(big.Rat).Quo(q, big.NewRat(int64((u+1)*(v+1)), 1)))
				for i, circle := range source.Circles {
					disk[i].Add(disk[i], new(big.Rat).Mul(q, parabolaCircleDiskMoment(circle, u, v)))
				}
			}
		}
	}
	// Each disk integral above excludes its common factor pi.
	pi := parabolaCircleMeshPi()
	bound := func(offset, factor *big.Rat) (out [2]*big.Rat) {
		for i, v := range pi {
			out[i] = new(big.Rat).Add(offset, new(big.Rat).Mul(factor, v))
		}
		if factor.Sign() < 0 {
			out[0], out[1] = out[1], out[0]
		}
		return out
	}
	zero := new(big.Rat)
	volumes := [4][2]*big.Rat{
		bound(lower, zero), bound(new(big.Rat).Sub(unit, lower), zero), bound(zero, disk[0]), bound(zero, disk[1]),
	}
	enclosing, offset := 0, lower
	if source.CirclesAbove {
		enclosing, offset = 1, new(big.Rat).Sub(unit, lower)
	}
	volumes[enclosing] = bound(offset, new(big.Rat).Neg(new(big.Rat).Add(disk[0], disk[1])))
	return volumes
}

func parabolaCircleDiskMoment(c geom.GraphContactCircle, u, v int) *big.Rat {
	out := new(big.Rat)
	power := func(x *big.Rat, n int) *big.Rat {
		p := big.NewRat(1, 1)
		for range n {
			p.Mul(p, x)
		}
		return p
	}
	factorial := func(n int) int64 {
		product := int64(1)
		for i := 2; i <= n; i++ {
			product *= int64(i)
		}
		return product
	}
	for i := 0; 2*i <= u; i++ {
		for j := 0; 2*j <= v; j++ {
			// Translate to the circle center. Odd central moments vanish.
			// Even moments divided by pi follow from polar integration.
			k := i + j
			value := big.NewRat(factorial(2*i)*factorial(2*j),
				(int64(1)<<(2*k))*factorial(i)*factorial(j)*factorial(k+1))
			value.Mul(value, power(c.RadiusSquared, k+1))
			value.Mul(value, power(c.Center[0], u-2*i))
			value.Mul(value, power(c.Center[1], v-2*j))
			value.Mul(value, new(big.Rat).SetInt(new(big.Int).Binomial(int64(u), int64(2*i))))
			value.Mul(value, new(big.Rat).SetInt(new(big.Int).Binomial(int64(v), int64(2*j))))
			out.Add(out, value)
		}
	}
	return out
}

func parabolaCircleMeshPi() [2]*big.Rat {
	atan := func(d int64) [2]*big.Rat {
		sum := new(big.Rat)
		power := big.NewInt(d)
		var lower *big.Rat
		for i := 0; i <= 24; i++ {
			term := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Mul(power, big.NewInt(int64(2*i+1))))
			if i%2 == 0 {
				sum.Add(sum, term)
			} else {
				sum.Sub(sum, term)
			}
			if i == 23 {
				lower = new(big.Rat).Set(sum)
			}
			power.Mul(power, big.NewInt(d*d))
		}
		return [2]*big.Rat{lower, sum}
	}
	// Machin's identity with alternating rational arctangent enclosures.
	a, b := atan(5), atan(239)
	return [2]*big.Rat{
		new(big.Rat).Sub(new(big.Rat).Mul(a[0], big.NewRat(16, 1)), new(big.Rat).Mul(b[1], big.NewRat(4, 1))),
		new(big.Rat).Sub(new(big.Rat).Mul(a[1], big.NewRat(16, 1)), new(big.Rat).Mul(b[0], big.NewRat(4, 1))),
	}
}
