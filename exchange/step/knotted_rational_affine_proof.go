package step

import (
	"math/big"
	"plainsolid/geom"
)

// Restrict the original quadratic Cox-de Boor basis exactly to one nonempty
// span, expressed in powers of local t in [0,1]. Zero denominators contribute
// zero, as required by repeated-knot B-spline recursion.
func quadraticOriginalSpanBasis(knots [3]*big.Rat, span int) ([4][3]*big.Rat, error) {
	var out [4][3]*big.Rat
	full := []*big.Rat{knots[0], knots[0], knots[0], knots[1], knots[2], knots[2], knots[2]}
	lo, hi := knots[span], knots[span+1]
	width := new(big.Rat).Sub(hi, lo)
	var basis func(int, int) [3]*big.Rat
	basis = func(i, d int) [3]*big.Rat {
		q := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		if d == 0 {
			if full[i].Cmp(lo) <= 0 && full[i+1].Cmp(hi) >= 0 && full[i].Cmp(full[i+1]) < 0 {
				q[0].SetInt64(1)
			}
			return q
		}
		add := func(child [3]*big.Rat, a, b *big.Rat) {
			for k := 0; k < d; k++ {
				q[k].Add(q[k], new(big.Rat).Mul(a, child[k]))
				q[k+1].Add(q[k+1], new(big.Rat).Mul(b, child[k]))
			}
		}
		den := new(big.Rat).Sub(full[i+d], full[i])
		if den.Sign() != 0 {
			add(basis(i, d-1), new(big.Rat).Quo(new(big.Rat).Sub(lo, full[i]), den), new(big.Rat).Quo(width, den))
		}
		den = new(big.Rat).Sub(full[i+d+1], full[i+1])
		if den.Sign() != 0 {
			add(basis(i+1, d-1), new(big.Rat).Quo(new(big.Rat).Sub(full[i+d+1], lo), den), new(big.Rat).Neg(new(big.Rat).Quo(width, den)))
		}
		return q
	}
	for i := range out {
		out[i] = basis(i, 2)
		for _, q := range out[i] {
			if q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
				return out, geom.ErrConvergence
			}
		}
	}
	return out, nil
}

func proveKnottedRationalAffine(p [4][4][3]*big.Rat, w [4][4]*big.Rat, u, v [3]*big.Rat) error {
	for us := 0; us < 2; us++ {
		bu, e := quadraticOriginalSpanBasis(u, us)
		if e != nil {
			return e
		}
		for vs := 0; vs < 2; vs++ {
			bv, e := quadraticOriginalSpanBasis(v, vs)
			if e != nil {
				return e
			}
			var weight [3][3]*big.Rat
			var numerator [3][3][3]*big.Rat
			for a := 0; a < 3; a++ {
				for b := 0; b < 3; b++ {
					weight[a][b] = new(big.Rat)
					for c := 0; c < 3; c++ {
						numerator[a][b][c] = new(big.Rat)
					}
				}
			}
			for i := 0; i < 4; i++ {
				for j := 0; j < 4; j++ {
					for a := 0; a < 3; a++ {
						for b := 0; b < 3; b++ {
							if bu[i][a].Sign() == 0 || bv[j][b].Sign() == 0 {
								continue
							}
							q := new(big.Rat).Mul(new(big.Rat).Mul(bu[i][a], bv[j][b]), w[i][j])
							weight[a][b].Add(weight[a][b], q)
							for c := 0; c < 3; c++ {
								if p[i][j][c].Sign() != 0 {
									numerator[a][b][c].Add(numerator[a][b][c], new(big.Rat).Mul(q, p[i][j][c]))
								}
							}
						}
					}
				}
			}
			for c := 0; c < 3; c++ {
				du := new(big.Rat).Sub(p[3][0][c], p[0][0][c])
				dv := new(big.Rat).Sub(p[0][3][c], p[0][0][c])
				base := new(big.Rat).Set(p[0][0][c])
				urange := new(big.Rat).Sub(u[2], u[0])
				vrange := new(big.Rat).Sub(v[2], v[0])
				base.Add(base, new(big.Rat).Mul(du, new(big.Rat).Quo(new(big.Rat).Sub(u[us], u[0]), urange)))
				base.Add(base, new(big.Rat).Mul(dv, new(big.Rat).Quo(new(big.Rat).Sub(v[vs], v[0]), vrange)))
				su := new(big.Rat).Mul(du, new(big.Rat).Quo(new(big.Rat).Sub(u[us+1], u[us]), urange))
				sv := new(big.Rat).Mul(dv, new(big.Rat).Quo(new(big.Rat).Sub(v[vs+1], v[vs]), vrange))
				for a := 0; a <= 3; a++ {
					for b := 0; b <= 3; b++ {
						lhs, rhs := new(big.Rat), new(big.Rat)
						if a < 3 && b < 3 {
							lhs.Set(numerator[a][b][c])
							rhs.Mul(base, weight[a][b])
						}
						if a > 0 && b < 3 {
							rhs.Add(rhs, new(big.Rat).Mul(su, weight[a-1][b]))
						}
						if b > 0 && a < 3 {
							rhs.Add(rhs, new(big.Rat).Mul(sv, weight[a][b-1]))
						}
						if lhs.Num().BitLen() > 4096 || lhs.Denom().BitLen() > 4096 || rhs.Num().BitLen() > 4096 || rhs.Denom().BitLen() > 4096 || lhs.Cmp(rhs) != 0 {
							return geom.ErrInvalid
						}
					}
				}
			}
		}
	}
	return nil
}
