package geom

import "math/big"

// BiquadraticCoefficients certifies a single-span polynomial patch of degree
// at most two in each coordinate, using equal positive homogeneous weights.
// The caller-owned exact coefficient [i][j][axis] multiplies x^i*y^j, where
// x,y normalize the patch domain to [0,1]. Unused coefficients are exact zero.
// Other representations return false; no sampled polynomial detection occurs.
func (p *SurfacePatch) BiquadraticCoefficients() ([3][3][3]*big.Rat, bool) {
	var result [3][3][3]*big.Rat
	if p == nil || p.degreeU < 1 || p.degreeU > 2 || p.degreeV < 1 || p.degreeV > 2 || p.countU != p.degreeU+1 || p.countV != p.degreeV+1 {
		return result, false
	}
	weight := p.net[0][3]
	if weight.Sign() <= 0 {
		return result, false
	}
	for _, h := range p.net {
		if h[3].Cmp(weight) != 0 {
			return result, false
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
			for k := 0; k < 3; k++ {
				result[a][b][k] = new(big.Rat)
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
					for k := 0; k < 3; k++ {
						v := new(big.Rat).Quo(p.net[i*p.countV+j][k], weight)
						v.Mul(v, big.NewRat(factor, 1))
						result[a][b][k].Add(result[a][b][k], v)
					}
				}
			}
		}
	}
	return result, true
}
