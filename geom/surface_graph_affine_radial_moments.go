package geom

import (
	"context"
	"math/big"
)

// BoundVolumeBands encloses volume between the authenticated original graphs
// over complete physical circular bands. The caller must separately prove that
// these uncut bands are retained material; this does not assemble a solid.
func (r *AffineRadialContours) BoundVolumeBands(ctx context.Context, bands []int, o RadialGraphVolumeOptions) (RadialGraphVolume, error) {
	if r == nil || r.radial == nil {
		return RadialGraphVolume{}, ErrInvalid
	}
	return r.radial.BoundVolumeBands(ctx, bands, o)
}

// PlanarMomentsBands returns physical XY moments, not normalized-chart moments.
// It retains the constant positive height-denominator and complete-band gates.
func (r *AffineRadialContours) PlanarMomentsBands(ctx context.Context, bands []int, o RadialGraphMomentOptions) (RadialGraphPlanarMoments, error) {
	if r == nil || r.radial == nil {
		return RadialGraphPlanarMoments{}, ErrInvalid
	}
	return r.radial.PlanarMomentsBands(ctx, bands, o)
}

// MassMomentsBands includes the physical Z first moment and centroid. Original
// normalized height polynomials are transported exactly to physical XY before
// angular integration. Individual roofs may be nonradial. Complete material
// ownership must be proved independently; no rounded coordinate inversion occurs.
func (r *AffineRadialContours) MassMomentsBands(ctx context.Context, bands []int, o RadialGraphMomentOptions) (RadialGraphMassMoments, error) {
	if r == nil || r.radial == nil {
		return RadialGraphMassMoments{}, ErrInvalid
	}
	return r.radial.MassMomentsBands(ctx, bands, o)
}

func radialPhysicalHeightPower(ctx context.Context, p radialPower, rectangle [2][2]*big.Rat, b *polynomialBudget) (radialPower, error) {
	var powers [2][]rationalPolynomial
	for axis, count := range []int{len(p), len(p[0])} {
		if count > 9 {
			return nil, ErrConvergence
		}
		lo, hi := rectangle[axis][0], rectangle[axis][1]
		if lo == nil || hi == nil {
			return nil, ErrInvalid
		}
		if e := b.check(lo); e != nil {
			return nil, e
		}
		if e := b.check(hi); e != nil {
			return nil, e
		}
		width := new(big.Rat).Sub(hi, lo)
		if e := b.check(width); e != nil {
			return nil, e
		}
		if width.Sign() <= 0 {
			return nil, ErrInvalid
		}
		slope := new(big.Rat).Inv(width)
		offset := new(big.Rat).Mul(new(big.Rat).Neg(lo), slope)
		if e := b.check(slope); e != nil {
			return nil, e
		}
		if e := b.check(offset); e != nil {
			return nil, e
		}
		powers[axis] = make([]rationalPolynomial, count)
		powers[axis][0] = rationalPolynomial{big.NewRat(1, 1)}
		for i := 1; i < count; i++ {
			q := make(rationalPolynomial, i+1)
			for j := range q {
				q[j] = new(big.Rat)
			}
			for j, c := range powers[axis][i-1] {
				for k, factor := range []*big.Rat{offset, slope} {
					term := new(big.Rat).Mul(c, factor)
					if e := b.check(term); e != nil {
						return nil, e
					}
					q[j+k].Add(q[j+k], term)
					if e := b.check(q[j+k]); e != nil {
						return nil, e
					}
				}
			}
			powers[axis][i] = q
		}
	}
	out := newRadialPower(len(p), len(p[0]))
	for i, row := range p {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for j, c := range row {
			if e := b.check(c); e != nil {
				return nil, e
			}
			if c.Sign() == 0 {
				continue
			}
			for x, a := range powers[0][i] {
				for y, d := range powers[1][j] {
					term := new(big.Rat).Mul(c, a)
					if e := b.check(term); e != nil {
						return nil, e
					}
					term.Mul(term, d)
					if e := b.check(term); e != nil {
						return nil, e
					}
					out[x][y].Add(out[x][y], term)
					if e := b.check(out[x][y]); e != nil {
						return nil, e
					}
				}
			}
		}
	}
	return out, nil
}
