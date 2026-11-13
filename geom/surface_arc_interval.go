package geom

import (
	"context"
	"math"
	"math/big"
)

// SurfaceArcIntervalOptions controls scalar interval-Newton contraction.
// Zero limits select 24 iterations, 4M operations and 16384 coefficient bits.
// Positive tolerances request dependent parameter/model box widths; zero
// disables a goal. With both disabled Complete means representable stabilization.
type SurfaceArcIntervalOptions struct {
	ParameterTolerance, BoxTolerance                 float64
	MaxIterations, MaxOperations, MaxCoefficientBits int
}

// SurfaceArcIntervalEnclosure covers all arc points in the requested free slab.
// It makes no existence claim for each section, or even for the slab as a whole.
// Excluded proves there are no roots. Otherwise HasBounds certifies Parameters
// and Bounds even on incomplete refinement, cancellation or resource error.
// Complete describes only refinement/exclusion, not arc topology. A slab whose
// intersection with the cell is a singleton retains conservative global model
// bounds and may remain incomplete; use EvaluateExact for point refinement.
type SurfaceArcIntervalEnclosure struct {
	Parameters                                 UVBox
	Bounds                                     Box
	HasBounds, Excluded, Complete              bool
	DependentAxis, Iterations, ExactOperations int
}

// EncloseInterval encloses every plane root in the source cell whose independent
// coordinate lies in [start,end]. The free coordinate is v for dependent axis0,
// and u for dependent axis1. The requested positive-width slab may extend beyond
// the cell or actual arc projection: roots outside the cell are irrelevant.
// Strict dependent numerator derivative is rechecked from private exact source;
// no caller-modifiable public bounding box is trusted. Interval Newton discards
// only regions that cannot contain a root. All arithmetic shares one budget.
func (a SurfacePlaneArc) EncloseInterval(ctx context.Context, start, end float64, o SurfaceArcIntervalOptions) (r SurfaceArcIntervalEnclosure, err error) {
	if ctx == nil || a.source == nil || !finite(start) || !finite(end) || start >= end {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	fixed := a.Certificate.FixedParameterAxis
	if fixed < 0 || fixed > 1 {
		return r, ErrInvalid
	}
	r.DependentAxis = fixed
	if o.MaxIterations == 0 {
		o.MaxIterations = 24
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.ParameterTolerance) || !finite(o.BoxTolerance) || o.ParameterTolerance < 0 || o.BoxTolerance < 0 || o.MaxIterations < 1 || o.MaxIterations > 256 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	d := a.source.Domain()
	freeMin, freeMax := d.VMin, d.VMax
	if fixed == 1 {
		freeMin, freeMax = d.UMin, d.UMax
	}
	lower, upper := max(start, freeMin), min(end, freeMax)
	if lower > upper {
		r.Excluded, r.Complete = true, true
		return r, nil
	}
	rectangle := d
	if fixed == 0 {
		rectangle.VMin, rectangle.VMax = lower, upper
	} else {
		rectangle.UMin, rectangle.UMax = lower, upper
	}
	r.Parameters = UVBox{Min: Vec2{rectangle.UMin, rectangle.VMin}, Max: Vec2{rectangle.UMax, rectangle.VMax}}
	r.Bounds = a.source.bounds
	r.HasBounds = true
	if lower == upper {
		return r, ErrConvergence
	}
	p, e := trimPatchBudget(a.source, rectangle, budget)
	if e != nil {
		return r, e
	}
	r.Bounds = p.bounds
	hasGoals := o.ParameterTolerance > 0 || o.BoxTolerance > 0
	within := func(lo, hi, tolerance float64) bool {
		if !finite(lo) || !finite(hi) {
			return false
		}
		return new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo)).Cmp(new(big.Rat).SetFloat64(tolerance)) <= 0
	}
	meetsGoals := func() bool {
		lo, hi := rectangle.UMin, rectangle.UMax
		if fixed == 1 {
			lo, hi = rectangle.VMin, rectangle.VMax
		}
		if o.ParameterTolerance > 0 && !within(lo, hi, o.ParameterTolerance) {
			return false
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
	for iteration := 0; iteration < o.MaxIterations; iteration++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		r.Iterations = iteration + 1
		data, e := p.arcData(a.plane, budget)
		if e != nil {
			return r, e
		}
		if data.base[4].sign() != 0 {
			r.Excluded, r.Complete = true, true
			r.HasBounds = false
			return r, nil
		}
		derivative := data.du[4]
		if fixed == 1 {
			derivative = data.dv[4]
		}
		if derivative.sign() == 0 {
			return r, ErrConvergence
		}
		if hasGoals && meetsGoals() {
			r.Complete = true
			return r, nil
		}
		centerRange, e := restrictedScalarRange(data.fields[4], p.countU, p.countV, fixed, big.NewRat(1, 2), budget)
		if e != nil {
			return r, e
		}
		offset, e := intervalDivide(centerRange, derivative, budget)
		if e != nil {
			return r, e
		}
		lo, hi := rectangle.UMin, rectangle.UMax
		if fixed == 1 {
			lo, hi = rectangle.VMin, rectangle.VMax
		}
		center := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).SetFloat64(lo), new(big.Rat).SetFloat64(hi)), big.NewRat(2, 1))
		image, e := intervalSubtract(exactInterval{center, center}, offset, budget)
		if e != nil {
			return r, e
		}
		nextLo, nextHi := max(lo, rationalFloatBound(image.lo, true)), min(hi, rationalFloatBound(image.hi, false))
		if nextLo > nextHi {
			r.Excluded, r.Complete = true, true
			r.HasBounds = false
			return r, nil
		}
		if nextLo == nextHi {
			nextLo = max(lo, math.Nextafter(nextLo, math.Inf(-1)))
			nextHi = min(hi, math.Nextafter(nextHi, math.Inf(1)))
			if nextLo == nextHi {
				return r, ErrConvergence
			}
		}
		if nextLo == lo && nextHi == hi {
			if !hasGoals || meetsGoals() {
				r.Complete = true
				return r, nil
			}
			return r, ErrConvergence
		}
		next := rectangle
		if fixed == 0 {
			next.UMin, next.UMax = nextLo, nextHi
		} else {
			next.VMin, next.VMax = nextLo, nextHi
		}
		restricted, e := trimPatchBudget(p, next, budget)
		if e != nil {
			return r, e
		}
		p = restricted
		rectangle = next
		r.Parameters = UVBox{Min: Vec2{next.UMin, next.VMin}, Max: Vec2{next.UMax, next.VMax}}
		r.Bounds = p.bounds
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
