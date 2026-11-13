package geom

import (
	"context"
	"math/big"
)

// NewCanonicalWrappedCylinder constructs the exact circular-profile wrapped
// lateral chart of a Z-axis cylinder, using asymmetric rational quadrant
// weights with exact circle identities. Base.Z is the lower cap height.
// All model control coordinates must be exactly representable; the constructor
// refuses a rounded translation or height addition.
func NewCanonicalWrappedCylinder(ctx context.Context, base Vec3, radius, height float64, o SurfacePatchOptions) (*WrappedSurfacePatch, error) {
	if ctx == nil || !base.IsFinite() || !finite(radius) || !finite(height) || radius <= 0 || height <= 0 {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	c := [3]*big.Rat{new(big.Rat).SetFloat64(base.X), new(big.Rat).SetFloat64(base.Y), new(big.Rat).SetFloat64(base.Z)}
	r, h := new(big.Rat).SetFloat64(radius), new(big.Rat).SetFloat64(height)
	q := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	w := [9]float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	// Adjacent odd control weights agree at the junctions in this
	// asymmetric basis, so equal span lengths preserve first derivatives.
	ku := []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}
	kv := []float64{0, 0, 1, 1}
	var poles []Vec3
	var weights []float64
	for i, xy := range q {
		for j := range 2 {
			p, e := RepresentedExactPoint([3]*big.Rat{
				new(big.Rat).Add(c[0], new(big.Rat).Mul(r, big.NewRat(xy[0], 1))),
				new(big.Rat).Add(c[1], new(big.Rat).Mul(r, big.NewRat(xy[1], 1))),
				new(big.Rat).Add(c[2], new(big.Rat).Mul(h, big.NewRat(int64(j), 1))),
			})
			if e != nil {
				return nil, e
			}
			poles, weights = append(poles, p), append(weights, w[i])
		}
	}
	s, e := NewBSplineSurface(2, 1, 9, 2, poles, ku, kv, weights)
	if e != nil {
		return nil, e
	}
	p, e := NewSurfacePatch(s, o)
	if e != nil {
		return nil, e
	}
	out, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{
		WrapU: true, ContinuityOrder: 1, MaxPoles: o.MaxPoles,
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	return out, e
}
