package geom

import (
	"context"
	"math"
	"math/big"
)

// SurfaceTubeIntervalOptions bounds uniform enclosure contraction. Zero limits
// select 24 iterations, 8M operations and 16384 coefficient bits. Positive
// tolerances request dependent parameter and model-box widths respectively;
// zero disables that goal. With both disabled, Complete means representable
// contraction has stabilized, not a precision claim. A resource limit preserves
// the latest proven enclosure when HasBounds is true.
type SurfaceTubeIntervalOptions struct {
	ParameterTolerance, BoxTolerance                 float64
	MaxIterations, MaxOperations, MaxCoefficientBits int
}

// SurfaceTubeIntervalEnclosure contains every unique branch root for every free
// parameter in the requested closed interval. ParametersA/B include the entire
// interval; Bounds encloses the shared model image. HasBounds certifies this
// even when Complete is false or the call returns cancellation/ErrConvergence.
// Complete only describes the requested refinement goal, never branch coverage.
type SurfaceTubeIntervalEnclosure struct {
	ParametersA, ParametersB    UVBox
	Bounds                      Box
	HasBounds, Complete         bool
	Iterations, ExactOperations int
}

// EncloseInterval contracts dependent coordinates with uniform Krawczyk images.
// The private original tube proof supplies existence; subsequent tests need not
// re-establish strict inclusion. All known roots stay in each retained box.
// start<end must lie inside the proved free interval; use Evaluate for a singleton.
// Arithmetic/trim work shares one cumulative budget. Cancellation is cooperative
// between bounded exact steps and preserves any established enclosure.
func (t VerifiedSurfaceTube) EncloseInterval(ctx context.Context, start, end float64, o SurfaceTubeIntervalOptions) (r SurfaceTubeIntervalEnclosure, err error) {
	if ctx == nil || t.a == nil || t.b == nil || t.axis < 0 || t.axis > 3 || t.tangentSign == 0 || !finite(start) || !finite(end) || start >= end {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxIterations == 0 {
		o.MaxIterations = 24
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.ParameterTolerance) || !finite(o.BoxTolerance) || o.ParameterTolerance < 0 || o.BoxTolerance < 0 || o.MaxIterations < 1 || o.MaxIterations > 256 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	da, db := t.a.Domain(), t.b.Domain()
	current := [4][2]float64{{da.UMin, da.UMax}, {da.VMin, da.VMax}, {db.UMin, db.UMax}, {db.VMin, db.VMax}}
	if start < current[t.axis][0] || end > current[t.axis][1] {
		return r, ErrDomain
	}
	current[t.axis] = [2]float64{start, end}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	enclose := func(parameters [4][2]float64) (*SurfacePatch, *SurfacePatch, error) {
		a, e := trimPatchBudget(t.a, ParameterRectangle{UMin: parameters[0][0], UMax: parameters[0][1], VMin: parameters[1][0], VMax: parameters[1][1]}, budget)
		if e != nil {
			return nil, nil, e
		}
		b, e := trimPatchBudget(t.b, ParameterRectangle{UMin: parameters[2][0], UMax: parameters[2][1], VMin: parameters[3][0], VMax: parameters[3][1]}, budget)
		if e != nil {
			return nil, nil, e
		}
		bounds, ok := a.bounds.Intersect(b.bounds)
		if !ok {
			return nil, nil, ErrConvergence
		}
		r.ParametersA = UVBox{Min: Vec2{parameters[0][0], parameters[1][0]}, Max: Vec2{parameters[0][1], parameters[1][1]}}
		r.ParametersB = UVBox{Min: Vec2{parameters[2][0], parameters[3][0]}, Max: Vec2{parameters[2][1], parameters[3][1]}}
		r.Bounds = bounds
		r.HasBounds = true
		return a, b, nil
	}
	a, b, e := enclose(current)
	if e != nil {
		return r, e
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	hasGoals := o.ParameterTolerance > 0 || o.BoxTolerance > 0
	within := func(lo, hi, tolerance float64) bool {
		if !finite(lo) || !finite(hi) {
			return false
		}
		return new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo)).Cmp(new(big.Rat).SetFloat64(tolerance)) <= 0
	}
	meetsGoals := func() bool {
		if o.ParameterTolerance > 0 {
			for axis, pair := range current {
				if axis != t.axis && !within(pair[0], pair[1], o.ParameterTolerance) {
					return false
				}
			}
		}
		if o.BoxTolerance > 0 {
			for axis := 0; axis < 3; axis++ {
				if !within(coordinate(r.Bounds.Min, axis), coordinate(r.Bounds.Max, axis), o.BoxTolerance) {
					return false
				}
			}
		}
		return true
	}
	if hasGoals && meetsGoals() {
		r.Complete = true
		return r, nil
	}
	for iteration := 0; iteration < o.MaxIterations; iteration++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		remaining := o.MaxOperations - budget.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		proof, e := CertifySurfacePairTube(a, b, SurfacePairTubeOptions{FreeAxis: t.axis, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += proof.Test.ExactOperations
		r.Iterations = iteration + 1
		if e != nil {
			return r, e
		}
		if !proof.Test.CenterInvertible || proof.ExcludesContact {
			return r, ErrConvergence
		}
		next := current
		changed := false
		for j, axis := range proof.Test.ActiveAxes {
			bound := proof.Test.KrawczykBounds[j]
			lo, hi := max(current[axis][0], bound.Min), min(current[axis][1], bound.Max)
			if lo > hi {
				return r, ErrConvergence
			}
			if lo == hi {
				lo = max(current[axis][0], math.Nextafter(lo, math.Inf(-1)))
				hi = min(current[axis][1], math.Nextafter(hi, math.Inf(1)))
				if lo == hi {
					return r, ErrConvergence
				}
			}
			next[axis] = [2]float64{lo, hi}
			changed = changed || next[axis] != current[axis]
		}
		if !changed {
			if !hasGoals || meetsGoals() {
				r.Complete = true
				return r, nil
			}
			return r, ErrConvergence
		}
		a, b, e = enclose(next)
		if e != nil {
			return r, e
		}
		current = next
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if hasGoals && meetsGoals() {
			r.Complete = true
			return r, nil
		}
	}
	return r, ErrConvergence
}
