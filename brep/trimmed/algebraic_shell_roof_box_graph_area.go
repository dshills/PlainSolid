//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

func mixedRoofGraphArea(ctx context.Context, b *mixedRoofBox, gap float64, o MixedRoofAreaOptions) (geom.ExactParameterInterval, int, int, error) {
	var zero geom.ExactParameterInterval
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o.SurfacePatchEvaluationOptions}}
	if !finite(gap) || gap <= 0 {
		return zero, 0, 0, geom.ErrConvergence
	}
	type cell struct {
		uv    [2]geom.ExactParameterInterval
		depth int
	}
	unit := geom.ExactParameterInterval{Lower: new(big.Rat), Upper: big.NewRat(1, 1)}
	spans := b.roofSpans
	if len(spans) == 0 {
		spans = []*mixedRoofBox{b}
	}
	area := mixedRoofZeroInterval()
	visited := 0
	for _, s := range spans {
		d := s.domain
		x, err := w.rat(new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin)))
		if err != nil {
			return zero, visited, w.operations, err
		}
		y, err := w.rat(new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin)))
		if err != nil {
			return zero, visited, w.operations, err
		}
		spansize, err := w.rat(new(big.Rat).Mul(x, y))
		if err != nil {
			return zero, visited, w.operations, err
		}
		allowed, err := w.rat(new(big.Rat).Quo(ratInset(gap), big.NewRat(int64(len(spans)), 1)))
		if err != nil {
			return zero, visited, w.operations, err
		}
		if _, err = w.rat(allowed.Quo(allowed, spansize)); err != nil {
			return zero, visited, w.operations, err
		}
		numDer, err := roofAreaDerivativeGrids(s.linePower, &w)
		if err != nil {
			return zero, visited, w.operations, err
		}
		var weightDer [2]roofPowerGrid
		if s.weight != nil {
			weightDer, err = roofAreaDerivativeGrids(s.weight, &w)
			if err != nil {
				return zero, visited, w.operations, err
			}
		}
		axisActive := [2]bool{}
		for _, grid := range []roofPowerGrid{s.linePower, s.weight} {
			for i, row := range grid {
				for j, z := range row {
					if z.Sign() != 0 {
						axisActive[0] = axisActive[0] || i > 0
						axisActive[1] = axisActive[1] || j > 0
					}
				}
			}
		}
		stack := []cell{{uv: [2]geom.ExactParameterInterval{unit, unit}}}
		for len(stack) > 0 {
			if err := ctx.Err(); err != nil {
				return zero, visited, w.operations, err
			}
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited >= o.MaxCells {
				return zero, visited, w.operations, geom.ErrConvergence
			}
			visited++
			bounds, err := s.roofAreaDensity(c.uv, [2]*big.Rat{x, y}, numDer, weightDer, &w)
			if err != nil {
				return zero, visited, w.operations, err
			}
			// Bound both square roots directly by dyadic rational endpoints.
			lower, err := roofAreaSqrt(bounds.Lower, &w)
			if err != nil {
				return zero, visited, w.operations, err
			}
			upper, err := roofAreaSqrt(bounds.Upper, &w)
			if err != nil {
				return zero, visited, w.operations, err
			}
			width, err := w.rat(new(big.Rat).Sub(upper.Upper, lower.Lower))
			if err != nil {
				return zero, visited, w.operations, err
			}
			du, err := w.rat(new(big.Rat).Sub(c.uv[0].Upper, c.uv[0].Lower))
			if err != nil {
				return zero, visited, w.operations, err
			}
			dv, err := w.rat(new(big.Rat).Sub(c.uv[1].Upper, c.uv[1].Lower))
			if err != nil {
				return zero, visited, w.operations, err
			}
			if width.Cmp(allowed) <= 0 {
				a, err := w.rat(new(big.Rat).Mul(du, dv))
				if err != nil {
					return zero, visited, w.operations, err
				}
				if _, err = w.rat(a.Mul(a, spansize)); err != nil {
					return zero, visited, w.operations, err
				}
				for _, pair := range [][2]*big.Rat{{area.Lower, lower.Lower}, {area.Upper, upper.Upper}} {
					q, err := w.rat(new(big.Rat).Mul(a, pair[1]))
					if err != nil {
						return zero, visited, w.operations, err
					}
					if _, err = w.rat(pair[0].Add(pair[0], q)); err != nil {
						return zero, visited, w.operations, err
					}
				}
				continue
			}
			if c.depth >= o.MaxDepth {
				return zero, visited, w.operations, geom.ErrConvergence
			}
			a := 0
			if !axisActive[0] || axisActive[1] && dv.Cmp(du) > 0 {
				a = 1
			}
			m, err := w.rat(new(big.Rat).Add(c.uv[a].Lower, c.uv[a].Upper))
			if err != nil {
				return zero, visited, w.operations, err
			}
			if _, err = w.rat(m.Quo(m, big.NewRat(2, 1))); err != nil {
				return zero, visited, w.operations, err
			}
			c.depth++
			other := c
			c.uv[a].Upper, other.uv[a].Lower = m, m
			stack = append(stack, other, c)
		}
	}
	return area, visited, w.operations, nil
}

func roofAreaDerivativeGrids(g roofPowerGrid, w *mixedRoofLineWork) ([2]roofPowerGrid, error) {
	var out [2]roofPowerGrid
	for axis := range out {
		n, m := len(g), len(g[0])
		if axis == 0 {
			n--
		} else {
			m--
		}
		out[axis] = make(roofPowerGrid, n)
		for i := range out[axis] {
			out[axis][i] = make([]*big.Rat, m)
			for j := range out[axis][i] {
				a, b := i, j
				if axis == 0 {
					a++
				} else {
					b++
				}
				power := [2]int{a, b}[axis]
				v, err := w.rat(new(big.Rat).Mul(g[a][b], big.NewRat(int64(power), 1)))
				if err != nil {
					return out, err
				}
				out[axis][i][j] = v
			}
		}
	}
	return out, nil
}

func roofAreaSqrt(value *big.Rat, w *mixedRoofLineWork) (geom.ExactParameterInterval, error) {
	const digits = 64
	var zero geom.ExactParameterInterval
	if value.Sign() < 0 {
		return zero, geom.ErrConvergence
	}
	n := new(big.Int).Lsh(value.Num(), 2*digits)
	n.Quo(n, value.Denom()).Sqrt(n)
	den := new(big.Int).Lsh(big.NewInt(1), digits)
	a, err := w.rat(new(big.Rat).SetFrac(n, den))
	if err != nil {
		return zero, err
	}
	q, err := w.rat(new(big.Rat).Mul(a, a))
	if err != nil {
		return zero, err
	}
	if q.Cmp(value) == 0 {
		return geom.ExactParameterInterval{Lower: a, Upper: new(big.Rat).Set(a)}, nil
	}
	upper, err := w.rat(new(big.Rat).SetFrac(n.Add(n, big.NewInt(1)), den))
	if err != nil {
		return zero, err
	}
	return geom.ExactParameterInterval{Lower: a, Upper: upper}, nil
}

func roofAreaIntervalMul(a, b geom.ExactParameterInterval, w *mixedRoofLineWork) (geom.ExactParameterInterval, error) {
	var out geom.ExactParameterInterval
	for _, x := range []*big.Rat{a.Lower, a.Upper} {
		for _, y := range []*big.Rat{b.Lower, b.Upper} {
			v, err := w.rat(new(big.Rat).Mul(x, y))
			if err != nil {
				return out, err
			}
			mixedRoofInclude(&out, v)
		}
	}
	return out, nil
}

func (s *mixedRoofBox) roofAreaDensity(uv [2]geom.ExactParameterInterval, scales [2]*big.Rat, numDer, weightDer [2]roofPowerGrid, w *mixedRoofLineWork) (geom.ExactParameterInterval, error) {
	one := big.NewRat(1, 1)
	out := geom.ExactParameterInterval{Lower: new(big.Rat).Set(one), Upper: new(big.Rat).Set(one)}
	var n, wgt, den geom.ExactParameterInterval
	var err error
	if s.weight != nil {
		n, err = mixedRoofPowerRange(s.linePower, uv, w)
		if err != nil {
			return out, err
		}
		wgt, err = mixedRoofPowerRange(s.weight, uv, w)
		if err != nil {
			return out, err
		}
		wgt.Lower = maxRat(s.minWeight, wgt.Lower)
		lo, err := w.rat(new(big.Rat).Mul(wgt.Lower, wgt.Lower))
		if err != nil {
			return out, err
		}
		hi, err := w.rat(new(big.Rat).Mul(wgt.Upper, wgt.Upper))
		if err != nil {
			return out, err
		}
		den = geom.ExactParameterInterval{Lower: lo, Upper: hi}
	}
	for i := range 2 {
		v, err := mixedRoofPowerRange(numDer[i], uv, w)
		if err != nil {
			return out, err
		}
		if s.weight != nil {
			d, err := mixedRoofPowerRange(weightDer[i], uv, w)
			if err != nil {
				return out, err
			}
			a, err := roofAreaIntervalMul(v, wgt, w)
			if err != nil {
				return out, err
			}
			b, err := roofAreaIntervalMul(n, d, w)
			if err != nil {
				return out, err
			}
			lo, err := w.rat(new(big.Rat).Sub(a.Lower, b.Upper))
			if err != nil {
				return out, err
			}
			hi, err := w.rat(new(big.Rat).Sub(a.Upper, b.Lower))
			if err != nil {
				return out, err
			}
			v = geom.ExactParameterInterval{}
			for _, x := range []*big.Rat{lo, hi} {
				for _, y := range []*big.Rat{den.Lower, den.Upper} {
					z, err := w.rat(new(big.Rat).Quo(x, y))
					if err != nil {
						return out, err
					}
					mixedRoofInclude(&v, z)
				}
			}
		}
		lo, err := w.rat(new(big.Rat).Quo(v.Lower, scales[i]))
		if err != nil {
			return out, err
		}
		hi, err := w.rat(new(big.Rat).Quo(v.Upper, scales[i]))
		if err != nil {
			return out, err
		}
		maxAbs := maxRat(new(big.Rat).Abs(lo), new(big.Rat).Abs(hi))
		minAbs := new(big.Rat)
		if lo.Sign() > 0 || hi.Sign() < 0 {
			minAbs = minRat(new(big.Rat).Abs(lo), new(big.Rat).Abs(hi))
		}
		for _, p := range [][2]*big.Rat{{out.Lower, minAbs}, {out.Upper, maxAbs}} {
			a, err := w.rat(new(big.Rat).Mul(p[1], p[1]))
			if err != nil {
				return out, err
			}
			if _, err = w.rat(p[0].Add(p[0], a)); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}
