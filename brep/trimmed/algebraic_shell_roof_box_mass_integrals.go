package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

func roofPower(x *big.Rat, n int, w *mixedRoofLineWork) (*big.Rat, error) {
	v := big.NewRat(1, 1)
	for range n {
		if _, err := w.rat(v.Mul(v, x)); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func roofMonomialIntegral(a, b *big.Rat, n int, w *mixedRoofLineWork) (*big.Rat, error) {
	lo, err := roofPower(a, n+1, w)
	if err != nil {
		return nil, err
	}
	hi, err := roofPower(b, n+1, w)
	if err != nil {
		return nil, err
	}
	if _, err = w.rat(hi.Sub(hi, lo)); err != nil {
		return nil, err
	}
	return w.rat(hi.Quo(hi, big.NewRat(int64(n+1), 1)))
}

func roofSignedAxisIntegrals(a geom.ExactParameterInterval, n int, w *mixedRoofLineWork) ([2]*big.Rat, error) {
	out := [2]*big.Rat{new(big.Rat), new(big.Rat)} // integrated negative and positive parts
	var err error
	if n%2 == 0 {
		out[1], err = roofMonomialIntegral(a.Lower, a.Upper, n, w)
		return out, err
	}
	if a.Lower.Sign() < 0 {
		out[0], err = roofMonomialIntegral(a.Lower, minRat(a.Upper, new(big.Rat)), n, w)
		if err != nil {
			return out, err
		}
	}
	if a.Upper.Sign() > 0 {
		out[1], err = roofMonomialIntegral(maxRat(a.Lower, new(big.Rat)), a.Upper, n, w)
	}
	return out, err
}

// Integrate each signed monomial exactly over the constant bounding columns.
// The integral over the unknown remainder can have either sign. Its positive
// and negative parts are bounded separately by the outer column.
func enclosedRoofCellMoments(xy [2]geom.ExactParameterInterval, inner, outer []geom.ExactParameterInterval, w *mixedRoofLineWork) ([10]geom.ExactParameterInterval, error) {
	var out [10]geom.ExactParameterInterval
	var axis [2][3][2]*big.Rat
	for i, x := range xy {
		for n := range 3 {
			v, err := roofSignedAxisIntegrals(x, n, w)
			if err != nil {
				return out, err
			}
			axis[i][n] = v
		}
	}
	column := func(spans []geom.ExactParameterInterval) ([3][2]*big.Rat, error) {
		var zz [3][2]*big.Rat
		for n := range zz {
			zz[n] = [2]*big.Rat{new(big.Rat), new(big.Rat)}
			for _, span := range spans {
				v, err := roofSignedAxisIntegrals(span, n, w)
				if err != nil {
					return zz, err
				}
				for i := range v {
					if _, err = w.rat(zz[n][i].Add(zz[n][i], v[i])); err != nil {
						return zz, err
					}
				}
			}
		}
		return zz, nil
	}
	lo, err := column(inner)
	if err != nil {
		return out, err
	}
	hi, err := column(outer)
	if err != nil {
		return out, err
	}
	for m, e := range mixedRoofMomentAxes {
		out[m] = mixedRoofZeroInterval()
		for i, x := range axis[0][e[0]] {
			for j, y := range axis[1][e[1]] {
				if x.Sign() == 0 || y.Sign() == 0 {
					continue
				}
				xy, err := w.rat(new(big.Rat).Mul(x, y))
				if err != nil {
					return out, err
				}
				for k := range 2 {
					l, h := lo[e[2]][k], hi[e[2]][k]
					if (i+j+k)%2 == 0 { // even count of negative factors
						l, h = h, l
					}
					for _, endpoints := range [2][2]*big.Rat{{out[m].Lower, l}, {out[m].Upper, h}} {
						p, err := w.rat(new(big.Rat).Mul(xy, endpoints[1]))
						if err != nil {
							return out, err
						}
						if _, err = w.rat(endpoints[0].Add(endpoints[0], p)); err != nil {
							return out, err
						}
					}
				}
			}
		}
	}
	return out, nil
}

type roofPowerGrid [][]*big.Rat

func roofMultiplyPower(a, b roofPowerGrid, w *mixedRoofLineWork) (roofPowerGrid, error) {
	out := make(roofPowerGrid, len(a)+len(b)-1)
	for i := range out {
		out[i] = make([]*big.Rat, len(a[0])+len(b[0])-1)
		for j := range out[i] {
			out[i][j] = new(big.Rat)
		}
	}
	for i, row := range a {
		for j, x := range row {
			if x.Sign() == 0 {
				continue
			}
			for k, row := range b {
				for l, y := range row {
					if y.Sign() == 0 {
						continue
					}
					p, err := w.rat(new(big.Rat).Mul(x, y))
					if err != nil {
						return nil, err
					}
					d := out[i+k][j+l]
					if _, err = w.rat(d.Add(d, p)); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return out, nil
}

func roofWeightedAxisIntegrals(norm geom.ExactParameterInterval, a, length *big.Rat, degree int, w *mixedRoofLineWork) ([3][]*big.Rat, error) {
	var powers [3][]*big.Rat
	base := make([]*big.Rat, degree+3)
	for i := range base {
		r, err := roofMonomialIntegral(norm.Lower, norm.Upper, i, w)
		if err != nil {
			return powers, err
		}
		base[i], err = w.rat(r.Mul(r, length))
		if err != nil {
			return powers, err
		}
	}
	for n := range powers {
		powers[n] = make([]*big.Rat, degree+1)
	}
	for d := 0; d <= degree; d++ {
		powers[0][d] = base[d]
		// x = a+length*u. Integrate u^d*x, then multiply its linear
		// integral recurrence by x once more for the second moment.
		first := func(i int) (*big.Rat, error) {
			v, err := w.rat(new(big.Rat).Mul(a, base[i]))
			if err != nil {
				return nil, err
			}
			t, err := w.rat(new(big.Rat).Mul(length, base[i+1]))
			if err != nil {
				return nil, err
			}
			return w.rat(v.Add(v, t))
		}
		v, err := first(d)
		if err != nil {
			return powers, err
		}
		powers[1][d] = v
		u, err := first(d + 1)
		if err != nil {
			return powers, err
		}
		x, err := w.rat(new(big.Rat).Mul(a, v))
		if err != nil {
			return powers, err
		}
		y, err := w.rat(new(big.Rat).Mul(length, u))
		if err != nil {
			return powers, err
		}
		powers[2][d], err = w.rat(x.Add(x, y))
		if err != nil {
			return powers, err
		}
	}
	return powers, nil
}

func (q *MixedRoofMaterialQuery) exactRoofCellMoments(xy [2]geom.ExactParameterInterval, supports [2]*mixedRoofBox, uv [2][2]geom.ExactParameterInterval, roof, floor [2]int, powers map[*mixedRoofBox][3]roofPowerGrid, w *mixedRoofLineWork) ([10]geom.ExactParameterInterval, error) {
	var out [10]geom.ExactParameterInterval
	for m := range out {
		out[m] = mixedRoofZeroInterval()
	}
	for i, source := range q.box {
		if floor[i] != 0 {
			var axes [3][3]*big.Rat
			for n := range 3 {
				for a := range 3 {
					var err error
					if a < 2 {
						axes[a][n], err = roofMonomialIntegral(xy[a].Lower, xy[a].Upper, n, w)
					} else {
						axes[a][n], err = roofMonomialIntegral(new(big.Rat), source.floor, n, w)
					}
					if err != nil {
						return out, err
					}
				}
			}
			for m, e := range mixedRoofMomentAxes {
				v, err := w.rat(new(big.Rat).Mul(axes[0][e[0]], axes[1][e[1]]))
				if err != nil {
					return out, err
				}
				if _, err = w.rat(v.Mul(v, axes[2][e[2]])); err != nil {
					return out, err
				}
				if err = addExactRoofMoment(&out[m], v, floor[i], w); err != nil {
					return out, err
				}
			}
		}
		if roof[i] == 0 {
			continue
		}
		b := supports[i]
		p := powers[b]
		if p[0] == nil {
			p[0] = b.linePower
			for a := 1; a <= 2; a++ {
				var err error
				p[a], err = roofMultiplyPower(p[a-1], p[0], w)
				if err != nil {
					return out, err
				}
			}
			powers[b] = p
		}
		var axes [2][3][]*big.Rat
		for a, g := range [2][2]float64{{b.domain.UMin, b.domain.UMax}, {b.domain.VMin, b.domain.VMax}} {
			lo, hi := ratInset(g[0]), ratInset(g[1])
			width, err := w.rat(new(big.Rat).Sub(hi, lo))
			if err != nil {
				return out, err
			}
			deg := len(p[2]) - 1
			if a == 1 {
				deg = len(p[2][0]) - 1
			}
			axes[a], err = roofWeightedAxisIntegrals(uv[i][a], lo, width, deg, w)
			if err != nil {
				return out, err
			}
		}
		for m, e := range mixedRoofMomentAxes {
			value := new(big.Rat)
			for a, row := range p[e[2]] {
				for c, v := range row {
					if v.Sign() == 0 {
						continue
					}
					x, err := w.rat(new(big.Rat).Mul(axes[0][e[0]][a], axes[1][e[1]][c]))
					if err != nil {
						return out, err
					}
					if _, err = w.rat(x.Mul(x, v)); err != nil {
						return out, err
					}
					if _, err = w.rat(value.Add(value, x)); err != nil {
						return out, err
					}
				}
			}
			if _, err := w.rat(value.Quo(value, big.NewRat(int64(e[2]+1), 1))); err != nil {
				return out, err
			}
			if err := addExactRoofMoment(&out[m], value, roof[i], w); err != nil {
				return out, err
			}
		}
	}
	for m := range out {
		out[m].Upper.Set(out[m].Lower)
	}
	return out, nil
}

func addExactRoofMoment(out *geom.ExactParameterInterval, v *big.Rat, sign int, w *mixedRoofLineWork) error {
	if sign < 0 {
		out.Lower.Sub(out.Lower, v)
	} else {
		out.Lower.Add(out.Lower, v)
	}
	_, err := w.rat(out.Lower)
	return err
}
