package geom

import (
	"context"
	"math/big"
)

// BoundSurfaceOrientedVolumeFlux encloses the exact integral of
// (S-origin) dot (Su cross Sv)/3 over a clamped rational spline patch.
// The result is signed in chart order; negate it for a reversed face.
// A closed embedded shell's sum is its signed material volume. The bound
// uses exact derivative intervals on knot-aligned boxes, with an optional
// number of additional bisection levels per Bézier piece.
func BoundSurfaceOrientedVolumeFlux(ctx context.Context, p *SurfacePatch, origin Vec3, depth int,
	o SurfacePatchEvaluationOptions) (lower, upper *big.Rat, err error) {
	return boundSurfaceOrientedFlux(ctx, p, origin, depth, -1, o)
}

// BoundSurfaceOrientedFirstMomentFlux encloses the flux whose sum on a
// closed embedded oriented boundary is its first moment along axis about
// origin. A depth-zero single cell gives pointwise bounds scaled by the
// cell's parameter area, which also bound the flux of clipped subsets
// when the interval is expanded to include zero.
func BoundSurfaceOrientedFirstMomentFlux(ctx context.Context, p *SurfacePatch, origin Vec3, depth, axis int,
	o SurfacePatchEvaluationOptions) (lower, upper *big.Rat, err error) {
	if axis < 0 || axis > 2 {
		return nil, nil, ErrInvalid
	}
	return boundSurfaceOrientedFlux(ctx, p, origin, depth, axis, o)
}

func boundSurfaceOrientedFlux(ctx context.Context, p *SurfacePatch, origin Vec3, depth, momentAxis int,
	o SurfacePatchEvaluationOptions) (lower, upper *big.Rat, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || !origin.IsFinite() || depth < 0 || depth > 12 {
		return nil, nil, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 2000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, nil, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	lower, upper = new(big.Rat), new(big.Rat)
	type cell struct {
		p *SurfacePatch
		d int
	}
	stack := []cell{{p, depth}}
	rat := func(f float64) *big.Rat { return new(big.Rat).SetFloat64(f) }
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return nil, nil, e
		}
		q := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		t, u, knot := patchSplitChoice(q.p)
		if knot || q.d > 0 {
			left, right, e := q.p.split(t, u, b)
			if e != nil {
				return nil, nil, e
			}
			next := q.d
			if !knot {
				next--
			}
			stack = append(stack, cell{right, next}, cell{left, next})
			continue
		}
		x, d, e := surfaceDifferentialIntervals(q.p, -1, 0, b)
		if e != nil {
			return nil, nil, e
		}
		var du, dv [3]exactInterval
		for j := range du {
			du[j], dv[j] = d[j][0], d[j][1]
		}
		n, e := intervalCross(du, dv, b)
		if e != nil {
			return nil, nil, e
		}
		r := q.p.Domain()
		scale := new(big.Rat).Mul(new(big.Rat).Sub(rat(r.UMax), rat(r.UMin)), new(big.Rat).Sub(rat(r.VMax), rat(r.VMin)))
		if momentAxis >= 0 {
			// ∇·((x_i−o_i)² e_i/2) = x_i−o_i. Squaring an interval
			// that straddles zero must retain zero as its lower bound.
			v := rat(coordinate(origin, momentAxis))
			delta, e := intervalSubtract(x[momentAxis], exactInterval{v, v}, b)
			if e != nil {
				return nil, nil, e
			}
			a, z := new(big.Rat).Mul(delta.lo, delta.lo), new(big.Rat).Mul(delta.hi, delta.hi)
			if a.Cmp(z) > 0 {
				a, z = z, a
			}
			if delta.lo.Sign() <= 0 && delta.hi.Sign() >= 0 {
				a.SetInt64(0)
			}
			scale.Quo(scale, big.NewRat(2, 1))
			t, e := intervalMultiply(exactInterval{a, z}, n[momentAxis], b)
			if e != nil {
				return nil, nil, e
			}
			t, e = intervalMultiply(t, exactInterval{scale, scale}, b)
			if e != nil {
				return nil, nil, e
			}
			if e := b.check(lower.Add(lower, t.lo)); e != nil {
				return nil, nil, e
			}
			if e := b.check(upper.Add(upper, t.hi)); e != nil {
				return nil, nil, e
			}
			continue
		}
		scale.Quo(scale, big.NewRat(3, 1))
		for j := range n {
			v := rat(coordinate(origin, j))
			delta, e := intervalSubtract(x[j], exactInterval{v, v}, b)
			if e != nil {
				return nil, nil, e
			}
			t, e := intervalMultiply(delta, n[j], b)
			if e != nil {
				return nil, nil, e
			}
			t, e = intervalMultiply(t, exactInterval{scale, scale}, b)
			if e != nil {
				return nil, nil, e
			}
			if e := b.check(lower.Add(lower, t.lo)); e != nil {
				return nil, nil, e
			}
			if e := b.check(upper.Add(upper, t.hi)); e != nil {
				return nil, nil, e
			}
		}
	}
	return lower, upper, nil
}
