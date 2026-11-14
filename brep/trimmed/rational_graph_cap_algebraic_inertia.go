package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

func graphStockCentroidInertia(m AlgebraicGraphStockMass, bits int) (out [3][3]geom.ExactParameterInterval, err error) {
	add := func(a, b geom.ExactParameterInterval) geom.ExactParameterInterval {
		return geom.ExactParameterInterval{Lower: new(big.Rat).Add(a.Lower, b.Lower), Upper: new(big.Rat).Add(a.Upper, b.Upper)}
	}
	sub := func(a, b geom.ExactParameterInterval) geom.ExactParameterInterval {
		return geom.ExactParameterInterval{Lower: new(big.Rat).Sub(a.Lower, b.Upper), Upper: new(big.Rat).Sub(a.Upper, b.Lower)}
	}
	product := func(a, b geom.ExactParameterInterval) geom.ExactParameterInterval {
		roots := [4]*big.Rat{new(big.Rat).Mul(a.Lower, b.Lower), new(big.Rat).Mul(a.Lower, b.Upper),
			new(big.Rat).Mul(a.Upper, b.Lower), new(big.Rat).Mul(a.Upper, b.Upper)}
		lo, hi := roots[0], roots[0]
		for _, q := range roots[1:] {
			if q.Cmp(lo) < 0 {
				lo = q
			}
			if q.Cmp(hi) > 0 {
				hi = q
			}
		}
		return geom.ExactParameterInterval{Lower: lo, Upper: hi}
	}
	cov := m.SecondMoments
	for k, ij := range [6][2]int{{0, 0}, {1, 1}, {2, 2}, {0, 1}, {0, 2}, {1, 2}} {
		// M_i C_j = M_i M_j/V for the true correlated values. Their
		// enclosing product interval remains valid under that correlation.
		t := product(m.FirstMoments[ij[0]], m.Coordinates[ij[1]])
		cov[k] = sub(cov[k], t)
		if k < 3 && cov[k].Lower.Sign() < 0 {
			cov[k].Lower.SetInt64(0)
		}
		if cov[k].Upper.Cmp(cov[k].Lower) < 0 || !graphStockMomentFits(cov[k], bits) {
			return [3][3]geom.ExactParameterInterval{}, geom.ErrConvergence
		}
	}
	for i := range out {
		out[i][i] = add(cov[(i+1)%3], cov[(i+2)%3])
	}
	for k, ij := range [3][2]int{{0, 1}, {0, 2}, {1, 2}} {
		v := cov[k+3]
		out[ij[0]][ij[1]] = geom.ExactParameterInterval{Lower: new(big.Rat).Neg(v.Upper), Upper: new(big.Rat).Neg(v.Lower)}
		out[ij[1]][ij[0]] = geom.ExactParameterInterval{Lower: new(big.Rat).Neg(v.Upper), Upper: new(big.Rat).Neg(v.Lower)}
	}
	for _, row := range out {
		for _, cell := range row {
			if !graphStockMomentFits(cell, bits) {
				return [3][3]geom.ExactParameterInterval{}, geom.ErrConvergence
			}
		}
	}
	return out, nil
}
