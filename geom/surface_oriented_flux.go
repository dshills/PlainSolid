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
