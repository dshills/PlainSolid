package geom

import (
	"context"
	"math/big"
	"runtime"
)

// SameParameterLines proves pointwise agreement of two affine parameter lines
// under this retained chart relation. Each target-axis offset must be constant
// and an integer multiple of its period, or zero on an unwrapped axis. It does
// not identify different traversals that happen to have the same model image.
// Both endpoints are in traversal order; wrapped coordinates may be lifted.
func (r *WrappedSurfaceChartRelation) SameParameterLines(ctx context.Context, a, b [2][2]*big.Rat, o SurfacePatchOptions) (bool, error) {
	if ctx == nil || r == nil || r.source == nil || r.target == nil {
		return false, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return false, e
	}
	limits, e := patchOptions(o)
	if e != nil {
		return false, e
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	for _, line := range [2][2][2]*big.Rat{a, b} {
		for _, point := range line {
			for _, q := range point {
				if q == nil {
					return false, ErrInvalid
				}
				if e = budget.check(q); e != nil {
					return false, e
				}
			}
		}
	}
	for owner, w := range []*WrappedSurfacePatch{r.source, r.target} {
		d := w.Domain()
		bounds := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		line := a
		if owner == 1 {
			line = b
		}
		for axis := 0; axis < 2; axis++ {
			if w.period[axis] != nil {
				continue
			}
			for _, p := range line {
				if p[axis].Cmp(new(big.Rat).SetFloat64(bounds[axis][0])) < 0 || p[axis].Cmp(new(big.Rat).SetFloat64(bounds[axis][1])) > 0 {
					return false, ErrDomain
				}
				runtime.KeepAlive(&p) // golang/go#82026: keeps p in the stack map. Remove once the Go fix is in use.
			}
		}
	}
	for i, j := range r.axis {
		var delta [2]*big.Rat
		for end := 0; end < 2; end++ {
			value := new(big.Rat).Add(r.offset[i], new(big.Rat).Mul(r.scale[i], a[end][i]))
			delta[end] = new(big.Rat).Sub(value, b[end][j])
			if e = budget.check(value); e != nil {
				return false, e
			}
			if e = budget.check(delta[end]); e != nil {
				return false, e
			}
		}
		if delta[0].Cmp(delta[1]) != 0 {
			return false, nil
		}
		if period := r.target.period[j]; period != nil {
			turns := new(big.Rat).Quo(delta[0], period)
			if e = budget.check(turns); e != nil {
				return false, e
			}
			if !turns.IsInt() {
				return false, nil
			}
		} else if delta[0].Sign() != 0 {
			return false, nil
		}
	}
	if e = ctx.Err(); e != nil {
		return false, e
	}
	return true, nil
}
