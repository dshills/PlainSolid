package geom

import "math/big"

// BilinearCoefficients returns caller-owned exact coefficients A,B,C,D for
// S(u,v)=A+B*x+C*y+D*x*y, where x,y normalize the patch domain to [0,1].
// Success certifies a single-span degree-one-by-one patch with equal positive
// homogeneous weights. Other representations return false, even if their
// geometry might reduce to bilinear after further symbolic simplification.
func (p *SurfacePatch) BilinearCoefficients() ([4][3]*big.Rat, bool) {
	var out [4][3]*big.Rat
	if p == nil || p.degreeU != 1 || p.degreeV != 1 || p.countU != 2 || p.countV != 2 || len(p.net) != 4 {
		return out, false
	}
	w := p.net[0][3]
	if w.Sign() <= 0 {
		return out, false
	}
	for _, h := range p.net {
		if h[3].Cmp(w) != 0 {
			return out, false
		}
	}
	for k := 0; k < 3; k++ {
		a := new(big.Rat).Quo(p.net[0][k], w)
		b := new(big.Rat).Quo(p.net[2][k], w)
		c := new(big.Rat).Quo(p.net[1][k], w)
		d := new(big.Rat).Quo(p.net[3][k], w)
		out[0][k] = a
		out[1][k] = new(big.Rat).Sub(b, a)
		out[2][k] = new(big.Rat).Sub(c, a)
		out[3][k] = new(big.Rat).Add(new(big.Rat).Sub(new(big.Rat).Sub(d, b), c), a)
	}
	return out, true
}
