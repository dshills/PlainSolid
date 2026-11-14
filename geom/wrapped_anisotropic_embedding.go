package geom

import (
	"context"
	"math/big"
)

// WrappedAnisotropicEmbedding authenticates an original weighted quadrant
// chart with independently affine positive X/Y control scales. These scales
// are not claimed to be ellipse radii unless the profile is independently
// proved circular. No rounded normalized surrogate is constructed.
type WrappedAnisotropicEmbedding struct {
	Center                 [2]*big.Rat
	LowerZ, UpperZ         *big.Rat
	LowerScale, UpperScale [2]*big.Rat
	QuarterParameters      [5]float64
	Complete               bool
	ExactOperations        int
}

type wrappedAnisotropicBudget struct {
	ctx  context.Context
	work polynomialBudget
	err  error
}

func (b *wrappedAnisotropicBudget) keep(v *big.Rat) *big.Rat {
	if b.err != nil {
		return new(big.Rat)
	}
	if b.err = b.ctx.Err(); b.err != nil {
		return new(big.Rat)
	}
	if b.err = b.work.check(v); b.err != nil {
		return new(big.Rat)
	}
	return v
}
func (b *wrappedAnisotropicBudget) add(a, c *big.Rat) *big.Rat {
	if b.err != nil {
		return new(big.Rat)
	}
	return b.keep(new(big.Rat).Add(a, c))
}
func (b *wrappedAnisotropicBudget) sub(a, c *big.Rat) *big.Rat {
	if b.err != nil {
		return new(big.Rat)
	}
	return b.keep(new(big.Rat).Sub(a, c))
}
func (b *wrappedAnisotropicBudget) mul(a, c *big.Rat) *big.Rat {
	if b.err != nil {
		return new(big.Rat)
	}
	return b.keep(new(big.Rat).Mul(a, c))
}
func (b *wrappedAnisotropicBudget) div(a, c *big.Rat) *big.Rat {
	if b.err != nil {
		return new(big.Rat)
	}
	if c.Sign() == 0 {
		b.err = ErrConvergence
		return new(big.Rat)
	}
	return b.keep(new(big.Rat).Quo(a, c))
}

// CertifyWrappedAnisotropicEmbedding requires four original positive-weight
// quadratic U quadrants and one affine-height V span. At every height the
// profile is diag(sx,sy) times the same strictly polar-monotone rational square
// profile. Positive sx,sy preserve simplicity and one-sided regularity; strictly increasing
// Z separates all sections. Therefore the chart is an outward embedded lateral
// boundary, with two nonintersecting terminal planar disks. This is a source
// proof only, not a certification of arbitrary surrounding native topology.
// In particular, dot(Su cross Sv, (x-cx,y-cy,0)) is
// sx*sy*Z_v*(f*g_prime-g*f_prime)>0 for the normalized profile (f,g).
// Knot joins need only be C0; no unproved C1 seam is asserted.
func CertifyWrappedAnisotropicEmbedding(ctx context.Context, w *WrappedSurfacePatch, o SurfacePatchOptions) (WrappedAnisotropicEmbedding, error) {
	var empty WrappedAnisotropicEmbedding
	options, e := patchOptions(o)
	if ctx == nil || e != nil || w == nil || w.patch == nil {
		return empty, ErrInvalid
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	p := w.patch
	if w.seam.PositionMatched != ([2]bool{true, false}) || p.countU != 9 || p.countV != 2 || p.degreeU != 2 || p.degreeV != 1 || len(p.net) > options.MaxPoles {
		return empty, ErrConvergence
	}
	k := p.knotsU
	if len(k) != 12 || k[0] != k[1] || k[1] != k[2] || k[3] != k[4] || k[5] != k[6] || k[7] != k[8] || k[9] != k[10] || k[10] != k[11] {
		return empty, ErrConvergence
	}
	cuts := [5]float64{k[2], k[3], k[5], k[7], k[9]}
	for i := 0; i < 4; i++ {
		if cuts[i] >= cuts[i+1] {
			return empty, ErrConvergence
		}
	}
	v := p.knotsV
	if len(v) != 4 || v[0] != v[1] || v[2] != v[3] || v[1] >= v[2] {
		return empty, ErrConvergence
	}
	b := &wrappedAnisotropicBudget{ctx: ctx, work: polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}}
	for _, h := range p.net {
		for _, x := range h {
			b.keep(x)
		}
		if h[3].Sign() <= 0 {
			return empty, ErrConvergence
		}
	}
	if b.err != nil {
		return empty, b.err
	}
	at := func(i, j, axis int) *big.Rat { return b.div(p.net[2*i+j][axis], p.net[2*i+j][3]) }
	center := [2]*big.Rat{b.div(b.add(at(0, 0, 0), at(4, 0, 0)), big.NewRat(2, 1)), b.div(b.add(at(2, 0, 1), at(6, 0, 1)), big.NewRat(2, 1))}
	scales := [2][2]*big.Rat{}
	z := [2]*big.Rat{}
	for j := 0; j < 2; j++ {
		scales[j] = [2]*big.Rat{b.sub(at(0, j, 0), center[0]), b.sub(at(2, j, 1), center[1])}
		z[j] = at(0, j, 2)
	}
	if b.err != nil {
		return empty, b.err
	}
	if z[0].Cmp(z[1]) >= 0 {
		return empty, ErrConvergence
	}
	for _, s := range scales {
		if s[0].Sign() <= 0 || s[1].Sign() <= 0 {
			return empty, ErrConvergence
		}
	}
	xy := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	for i, q := range xy {
		if p.net[2*i][3].Cmp(p.net[2*i+1][3]) != 0 {
			return empty, ErrConvergence
		}
		for j := 0; j < 2; j++ {
			want := [3]*big.Rat{b.add(center[0], b.mul(scales[j][0], big.NewRat(q[0], 1))), b.add(center[1], b.mul(scales[j][1], big.NewRat(q[1], 1))), z[j]}
			for axis, c := range want {
				h := b.mul(c, p.net[2*i+j][3])
				if b.err != nil {
					return empty, b.err
				}
				if h.Cmp(p.net[2*i+j][axis]) != 0 {
					return empty, ErrConvergence
				}
			}
		}
	}
	return WrappedAnisotropicEmbedding{Center: center, LowerZ: z[0], UpperZ: z[1], LowerScale: scales[0], UpperScale: scales[1], QuarterParameters: cuts, Complete: true, ExactOperations: b.work.operations}, nil
}
