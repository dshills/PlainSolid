package geom

import (
	"context"
	"math/big"
)

// sweepHomogeneousAffineSpans authenticates an original constant-weight
// affine net and returns its homogeneous U and V side translations.
func sweepHomogeneousAffineSpans(ctx context.Context, p *SurfacePatch, s *boundedRatArithmetic) (u, v [3]*big.Rat, affine bool, err error) {
	h := p.net[0]
	for _, pole := range p.net {
		if pole[3].Cmp(h[3]) != 0 {
			return u, v, false, nil
		}
	}
	for i := range u {
		u[i] = s.sub(p.net[p.degreeU*p.countV][i], h[i])
		v[i] = s.sub(p.net[p.degreeV][i], h[i])
	}
	for i := 0; i <= p.degreeU; i++ {
		if err := ctx.Err(); err != nil {
			return u, v, false, err
		}
		t := big.NewRat(int64(i), int64(p.degreeU))
		for j := 0; j <= p.degreeV; j++ {
			r := big.NewRat(int64(j), int64(p.degreeV))
			for k := range u {
				expect := s.add(h[k], s.add(s.mul(t, u[k]), s.mul(r, v[k])))
				if s.err != nil {
					return u, v, false, s.err
				}
				if p.net[i*p.countV+j][k].Cmp(expect) != 0 {
					return u, v, false, nil
				}
			}
		}
	}
	return u, v, true, nil
}
