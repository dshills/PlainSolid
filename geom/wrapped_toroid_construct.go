package geom

import (
	"context"
	"math/big"
)

// NewCanonicalWrappedCircularToroid constructs the exact axis-aligned
// circular toroid with the stated center and radii. Both parameter profiles
// are exact circles using asymmetric positive rational quadrant weights.
// AxialMinorRadius may differ from RadialMinorRadius. Control coordinates
// must be exactly representable; no control point is rounded.
func NewCanonicalWrappedCircularToroid(ctx context.Context, center Vec3, major, radialMinor, axialMinor float64, o SurfacePatchOptions) (*WrappedSurfacePatch, error) {
	if ctx == nil || !center.IsFinite() || !finite(major) || !finite(radialMinor) || !finite(axialMinor) ||
		radialMinor <= 0 || axialMinor <= 0 || major <= radialMinor {
		return nil, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	c := [3]*big.Rat{new(big.Rat).SetFloat64(center.X), new(big.Rat).SetFloat64(center.Y), new(big.Rat).SetFloat64(center.Z)}
	R, r, h := new(big.Rat).SetFloat64(major), new(big.Rat).SetFloat64(radialMinor), new(big.Rat).SetFloat64(axialMinor)
	q := [9][2]int64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	w := [9]float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	k := []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}
	poles := make([]Vec3, 0, 81)
	weights := make([]float64, 0, 81)
	for i, a := range q {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for j, b := range q {
			rho := new(big.Rat).Add(R, new(big.Rat).Mul(r, big.NewRat(b[0], 1)))
			p, e := RepresentedExactPoint([3]*big.Rat{
				new(big.Rat).Add(c[0], new(big.Rat).Mul(rho, big.NewRat(a[0], 1))),
				new(big.Rat).Add(c[1], new(big.Rat).Mul(rho, big.NewRat(a[1], 1))),
				new(big.Rat).Add(c[2], new(big.Rat).Mul(h, big.NewRat(b[1], 1))),
			})
			if e != nil {
				return nil, e
			}
			poles, weights = append(poles, p), append(weights, w[i]*w[j])
		}
	}
	s, e := NewBSplineSurface(2, 2, 9, 9, poles, k, k, weights)
	if e != nil {
		return nil, e
	}
	p, e := NewSurfacePatch(s, o)
	if e != nil {
		return nil, e
	}
	out, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{
		WrapU: true, WrapV: true, ContinuityOrder: 1,
		MaxPoles: o.MaxPoles, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	return out, e
}
