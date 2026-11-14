package geom

import (
	"context"
	"math/big"
)

// TorusQuadrant returns an exact native surface chart of the original cavity
// torus. Quadrants are 0..3 in increasing azimuth and meridian respectively.
// Each local parameter runs from 0 to 1 with angle 2*atan(t), plus the
// quadrant's multiple of pi/2. Adjacent charts have identical exact boundaries.
// The surface orientation is outward from the torus material; a cavity shell
// must reverse it. This is a support chart, not a sewn shell certificate.
//
// Construction and patch validation each receive the explicit operation limit
// (at most twice MaxOperations total); only nine positive homogeneous poles are
// authored. No Euclidean control or frame direction is rounded or normalized.
func (s ObliqueCylinderTorusCavityMaterial) TorusQuadrant(ctx context.Context, azimuth, meridian int, o SurfacePatchOptions) (*ExactBezierSurface, error) {
	if ctx == nil || s.source.R == nil || azimuth < 0 || azimuth > 3 || meridian < 0 || meridian > 3 {
		return nil, ErrInvalid
	}
	opts, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	a := &sphereOverlapArithmetic{ctx: ctx, b: &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}}
	// Homogeneous Bernstein coefficients of (1-t²,2t,1+t²).
	circle := func(q int) [3][3]*big.Rat {
		c := [3][3]*big.Rat{{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)}, {big.NewRat(0, 1), big.NewRat(2, 1), big.NewRat(2, 1)}}
		for i := range c {
			for k := 0; k < q; k++ {
				c[i][0], c[i][1] = new(big.Rat).Neg(c[i][1]), c[i][0]
			}
		}
		return c
	}
	u, v := circle(azimuth), circle(meridian)
	net := make([][4]*big.Rat, 0, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			radial := a.add(a.mul(s.source.R, v[j][2]), a.mul(s.source.r, v[j][0]))
			local := [3]*big.Rat{a.mul(u[i][0], radial), a.mul(u[i][1], radial), a.mul(a.mul(s.source.r, v[j][1]), u[i][2])}
			h := [4]*big.Rat{}
			h[3] = a.mul(u[i][2], v[j][2])
			for k := 0; k < 3; k++ {
				h[k] = a.mul(s.pose[k][3], h[3])
				for l := 0; l < 3; l++ {
					h[k] = a.add(h[k], a.mul(s.pose[k][l], local[l]))
				}
			}
			net = append(net, h)
		}
	}
	if a.err != nil {
		return nil, a.err
	}
	p, e := NewExactHomogeneousBezierSurface(ctx, 2, 2, net, o)
	if e != nil {
		return nil, e
	}
	return NewExactBezierSurface(p)
}
