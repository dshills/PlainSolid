package geom

import (
	"context"
	"math/big"
)

// ComposedBezierProjection retains a normalized exact 2D curve and its original
// parameter range. Curve(s)=(source[a+(b-a)s][Axes[0]],source[...][Axes[1]],0).
// Both parameter orientation and the ordered coordinate axes are preserved.
type ComposedBezierProjection struct {
	Curve         *ExactSplineCurve
	OriginalRange [2]*big.Rat
	Axes          [2]int
	Operations    int
}

// ProjectBezier2D requires rational hints equal to the immutable domain roots.
// It converts projected homogeneous degree <=4 exactly to Bernstein form and
// requires all weight coefficients strictly positive. A positive denominator
// lacking this sufficient coefficient proof remains unresolved (no rounding or
// subdivision is attempted). Algebraic domain ends cannot be rationalized.
// Work is cumulative across endpoint verification, conversion and construction.
func (c *ExactComposedCurveCell) ProjectBezier2D(ctx context.Context, axes [2]int, ends [2]*big.Rat, o ExactCurvePlaneEventOptions) (ComposedBezierProjection, error) {
	var zero ComposedBezierProjection
	if ctx == nil || c == nil || axes[0] < 0 || axes[0] > 2 || axes[1] < 0 || axes[1] > 2 || axes[0] == axes[1] || ends[0] == nil || ends[1] == nil || ends[0].Cmp(ends[1]) >= 0 {
		return zero, ErrInvalid
	}
	opts, e := graphRootOptions(o)
	if e != nil {
		return zero, e
	}
	used := 0
	for i, q := range ends {
		root, n, e := NewExactRationalRoot(ctx, q, opts)
		if e != nil {
			return zero, e
		}
		used += n
		opts.MaxOperations -= n
		if opts.MaxOperations < 1 {
			return zero, ErrConvergence
		}
		original, e := c.EndpointRoot(i)
		if e != nil {
			return zero, e
		}
		cmp, e := original.Compare(ctx, root, opts)
		used += cmp.Operations
		opts.MaxOperations -= cmp.Operations
		opts.MaxSubdivisions -= cmp.Subdivisions
		if e != nil {
			return zero, e
		}
		if !cmp.Proven {
			return zero, ErrConvergence
		}
		if cmp.Order != 0 {
			return zero, ErrDomain
		}
		if opts.MaxOperations < 1 || opts.MaxSubdivisions < 1 {
			return zero, ErrConvergence
		}
	}
	b := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	linear := rationalPolynomial{new(big.Rat).Set(ends[0]), new(big.Rat).Sub(ends[1], ends[0])}
	var power [4]rationalPolynomial
	degree := 1
	for output, input := range []int{axes[0], axes[1], -1, 3} {
		if input < 0 {
			continue
		}
		if len(c.power[input]) > 5 {
			return zero, ErrConvergence
		}
		power[output], e = composeProjectionPolynomial(c.power[input], linear, b)
		if e != nil {
			return zero, e
		}
		degree = max(degree, len(power[output])-1)
	}
	net := make([][4]*big.Rat, degree+1)
	for i := range net {
		for axis := 0; axis < 4; axis++ {
			net[i][axis] = new(big.Rat)
			for j := 0; j <= i && j < len(power[axis]); j++ {
				ratio := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(int64(degree), int64(j)))
				net[i][axis].Add(net[i][axis], new(big.Rat).Mul(power[axis][j], ratio))
				if e := b.check(net[i][axis]); e != nil {
					return zero, e
				}
			}
		}
		if net[i][3].Sign() <= 0 {
			return zero, ErrConvergence
		}
	}
	used += b.operations
	opts.MaxOperations -= b.operations
	if opts.MaxOperations < 1 {
		return zero, ErrConvergence
	}
	curve, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	used += curve.operations
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return ComposedBezierProjection{Curve: curve, OriginalRange: [2]*big.Rat{new(big.Rat).Set(ends[0]), new(big.Rat).Set(ends[1])}, Axes: axes, Operations: used}, nil
}
