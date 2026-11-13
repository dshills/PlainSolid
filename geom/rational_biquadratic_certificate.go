package geom

import "math/big"

// RationalBiquadraticCoefficients returns exact normalized homogeneous power
// coefficients Hx,Hy,Hz,W for a single-span patch of degree≤2 in each axis.
// MinimumWeight is the positive minimum homogeneous control weight, hence a
// global denominator lower bound on its domain. All returned values are owned
// by the caller. Unsupported representations return false without sampling.
func (p *SurfacePatch) RationalBiquadraticCoefficients() (coefficients [3][3][4]*big.Rat, minimumWeight *big.Rat, ok bool) {
	if p == nil || p.degreeU < 1 || p.degreeU > 2 || p.degreeV < 1 || p.degreeV > 2 || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return coefficients, nil, false
	}
	minimumWeight = new(big.Rat).Set(p.net[0][3])
	for _, h := range p.net {
		if h[3].Sign() <= 0 {
			return coefficients, nil, false
		}
		if h[3].Cmp(minimumWeight) < 0 {
			minimumWeight.Set(h[3])
		}
	}
	choose := func(n, k int) int64 {
		if k < 0 || k > n {
			return 0
		}
		if n == 2 && k == 1 {
			return 2
		}
		return 1
	}
	for a := 0; a < 3; a++ {
		for b := 0; b < 3; b++ {
			for k := 0; k < 4; k++ {
				coefficients[a][b][k] = new(big.Rat)
			}
		}
	}
	for a := 0; a <= p.degreeU; a++ {
		for b := 0; b <= p.degreeV; b++ {
			for i := 0; i <= a; i++ {
				for j := 0; j <= b; j++ {
					factor := choose(p.degreeU, i) * choose(p.degreeU-i, a-i) * choose(p.degreeV, j) * choose(p.degreeV-j, b-j)
					if (a-i+b-j)%2 != 0 {
						factor = -factor
					}
					for k := 0; k < 4; k++ {
						coefficients[a][b][k].Add(coefficients[a][b][k], new(big.Rat).Mul(p.net[i*p.countV+j][k], big.NewRat(factor, 1)))
					}
				}
			}
		}
	}
	return coefficients, minimumWeight, true
}
