package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"runtime"
)

func coincidentTraversalRange(direction int) [2]*big.Rat {
	if direction < 0 {
		return [2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}
	}
	return [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
}

// Candidate rational cuts use one nonconstant endpoint coordinate. Full
// restricted homogeneous identity authenticates every candidate; endpoint
// agreement alone is never accepted. This tranche admits degree<=4 single
// Bézier uses and the native descriptor's representable UV endpoints. Other
// parameter relations remain unresolved rather than becoming guessed cuts.
func refinedCoincidentWire(ctx context.Context, a, b []Use, loop int, matrix [3][4]*big.Rat, direction int, o SplineFaceCoincidenceOptions, budget int) ([]CoincidentSplineUse, int, error) {
	work := 0
	fail := func(e error) ([]CoincidentSplineUse, int, error) { return nil, work, e }
	if len(b) <= len(a) || len(a) == 0 {
		return fail(geom.ErrConvergence)
	}
	// Controls are oriented in actual wire traversal. Only exact single-span
	// original uses are admitted; normalization never changes their geometry.
	controls := func(u Use, mapped, reverse bool) (ExactBezierUV, error) {
		c, rev, e := coincidentUseCurve(ctx, u, o.Patches)
		if e != nil {
			return ExactBezierUV{}, e
		}
		if mapped {
			c, e = c.TransformRational(ctx, matrix, o.Patches)
			if e != nil {
				return ExactBezierUV{}, e
			}
		}
		net, _, e := c.BezierControlNet(ctx, o.Patches)
		if e != nil {
			return ExactBezierUV{}, e
		}
		h := make([][3]*big.Rat, len(net.Control))
		for i, p := range net.Control {
			if p[2].Sign() != 0 {
				return ExactBezierUV{}, geom.ErrInvalid
			}
			j := i
			if rev != reverse {
				j = len(h) - 1 - i
			}
			h[j] = [3]*big.Rat{p[0], p[1], p[3]}
		}
		// Elevate straight/quadratic uses exactly to the native cubic descriptor.
		for len(h) < 4 {
			n := len(h)
			next := make([][3]*big.Rat, n+1)
			next[0], next[n] = h[0], h[n-1]
			for i := 1; i < n; i++ {
				for axis := 0; axis < 3; axis++ {
					next[i][axis] = new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(int64(i), int64(n)), h[i-1][axis]), new(big.Rat).Mul(big.NewRat(int64(n-i), int64(n)), h[i][axis]))
				}
			}
			h = next
		}
		return NewExactBezierUV(h)
	}
	aa, bb := make([]ExactBezierUV, len(a)), make([]ExactBezierUV, len(b))
	for i, u := range a {
		q, e := controls(u, true, false)
		if e != nil {
			return fail(e)
		}
		aa[i] = q
	}
	for i, u := range b {
		q, e := controls(u, false, direction < 0)
		if e != nil {
			return fail(e)
		}
		bb[i] = q
	}
	for start := range b {
		ai := 0
		lo := new(big.Rat)
		var matches []CoincidentSplineUse
		for step := range b {
			if e := ctx.Err(); e != nil {
				return fail(e)
			}
			if ai == len(a) {
				break
			}
			bi := (start + direction*step + len(b)) % len(b)
			first, _, e := aa[ai].EvaluateRational(new(big.Rat))
			if e != nil {
				return fail(e)
			}
			last, _, e := aa[ai].EvaluateRational(big.NewRat(1, 1))
			if e != nil {
				return fail(e)
			}
			end, _, e := bb[bi].EvaluateRational(big.NewRat(1, 1))
			if e != nil {
				return fail(e)
			}
			axis := 0
			if first[0].Cmp(last[0]) == 0 {
				axis = 1
			}
			delta := new(big.Rat).Sub(last[axis], first[axis])
			if delta.Sign() == 0 {
				break
			}
			hi := new(big.Rat).Quo(new(big.Rat).Sub(end[axis], first[axis]), delta)
			runtime.KeepAlive(&first) // golang/go#82026: keeps first in the stack map. Remove once the Go fix is in use.
			if hi.Cmp(lo) <= 0 || hi.Cmp(big.NewRat(1, 1)) > 0 {
				break
			}
			piece := aa[ai]
			if hi.Cmp(big.NewRat(1, 1)) < 0 {
				piece, _, e = piece.Split(hi)
				if e != nil {
					return fail(e)
				}
			}
			if lo.Sign() > 0 {
				_, piece, e = piece.Split(new(big.Rat).Quo(lo, hi))
				if e != nil {
					return fail(e)
				}
			}
			ca, e := piece.descriptor(ctx, o.Patches)
			if e != nil {
				return fail(e)
			}
			cb, e := bb[bi].descriptor(ctx, o.Patches)
			if e != nil {
				return fail(e)
			}
			if budget-work < 1 {
				return fail(parametric.ErrBudget)
			}
			proof, e := geom.CompareExactCurvesContext(ctx, ca, cb, geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxOperations: budget - work, MaxCoefficientBits: o.Patches.MaxCoefficientBits})
			work += proof.ExactOperations
			if e != nil {
				return fail(e)
			}
			if !proof.Complete || !proof.Equal {
				break
			}
			matches = append(matches, CoincidentSplineUse{RangeA: [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}, RangeB: coincidentTraversalRange(direction), LoopA: loop, UseA: ai, EdgeA: a[ai].Edge, LoopB: loop, UseB: bi, EdgeB: b[bi].Edge, Reversed: direction < 0})
			lo = hi
			if hi.Cmp(big.NewRat(1, 1)) == 0 {
				ai++
				lo = new(big.Rat)
			}
		}
		if len(matches) == len(b) && ai == len(a) && lo.Sign() == 0 {
			return matches, work, nil
		}
	}
	return fail(geom.ErrConvergence)
}
