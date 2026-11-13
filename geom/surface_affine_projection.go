package geom

import (
	"context"
	"math/big"
)

// SurfaceAffineProjection certifies B's projected original coordinates as
// B[i]=Offset[i]+Scale[i]*A[Axis[i]]. The clipped rectangle is the exact preimage
// of B's closed domain intersected with A's domain. Rational fields are owned by
// the caller; they are diagnostics, never accepted as later proof input.
type SurfaceAffineProjection struct {
	// Empty means no positive-area projected overlap; boundary touching may remain.
	Domain          ParameterRectangle
	Empty           bool
	Axis            [2]int
	Offset, Scale   [2]*big.Rat
	ExactOperations int
}

// ClipSurfaceAffineProjection proves an axis-aligned affine projection onto a
// proven affine plane chart. This is not a surface/plane intersection test.
// A nonaffine or skewed projection remains ErrConvergence; exact clipping bounds
// that cannot fit float64 return ErrUnrepresentable without rounded trimming.
func ClipSurfaceAffineProjection(ctx context.Context, p *SurfacePatch, chart *AffineSurfaceChart, o SurfacePatchOptions) (r SurfaceAffineProjection, err error) {
	if ctx == nil || p == nil || chart == nil || chart.determinant == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 {
		return r, ErrInvalid
	}
	if len(p.net) > o.MaxPoles {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	q := *p
	q.net = make([]exactHomogeneous, len(p.net))
	q.options = o
	for i, h := range p.net {
		var point [3]*big.Rat
		for axis := 0; axis < 3; axis++ {
			point[axis] = new(big.Rat).Quo(h[axis], h[3])
			if e := budget.check(point[axis]); e != nil {
				return r, e
			}
		}
		left := o.MaxOperations - budget.operations
		if left < 1 {
			return r, ErrConvergence
		}
		inv, e := chart.InvertExactContext(ctx, point, AffineChartInverseOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += inv.ExactOperations
		if e != nil {
			return r, e
		}
		q.net[i] = exactHomogeneous{new(big.Rat).Mul(inv.U, h[3]), new(big.Rat).Mul(inv.V, h[3]), new(big.Rat), new(big.Rat).Set(h[3])}
		for _, x := range q.net[i] {
			if e := budget.check(x); e != nil {
				return r, e
			}
		}
	}
	if e := q.computeBounds(budget); e != nil {
		return r, e
	}
	left := o.MaxOperations - budget.operations
	if left < 1 {
		return r, ErrConvergence
	}
	affine, e := NewAffineSurfaceChart(ctx, &q, AffineSurfaceChartOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
	if affine != nil {
		budget.operations += affine.operations
	}
	if e != nil {
		return r, e
	}
	d := p.Domain()
	mins := [2]float64{d.UMin, d.VMin}
	maxs := [2]float64{d.UMax, d.VMax}
	plane := chart.Domain()
	targetMin := [2]float64{plane.UMin, plane.VMin}
	targetMax := [2]float64{plane.UMax, plane.VMax}
	for i := 0; i < 2; i++ {
		axis := -1
		coefficient := affine.u[i]
		if affine.u[i].Sign() != 0 && affine.v[i].Sign() == 0 {
			axis = 0
		} else if affine.v[i].Sign() != 0 && affine.u[i].Sign() == 0 {
			axis = 1
			coefficient = affine.v[i]
		}
		if axis < 0 {
			return r, ErrConvergence
		}
		r.Axis[i] = axis
		mn := new(big.Rat).SetFloat64(mins[axis])
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(maxs[axis]), mn)
		r.Scale[i] = new(big.Rat).Quo(coefficient, width)
		r.Offset[i] = new(big.Rat).Sub(affine.origin[i], new(big.Rat).Mul(r.Scale[i], mn))
		if e := budget.check(r.Scale[i]); e != nil {
			return r, e
		}
		if e := budget.check(r.Offset[i]); e != nil {
			return r, e
		}
	}
	if r.Axis[0] == r.Axis[1] {
		return r, ErrConvergence
	}
	for i, axis := range r.Axis {
		lo := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(targetMin[i]), r.Offset[i]), r.Scale[i])
		hi := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(targetMax[i]), r.Offset[i]), r.Scale[i])
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		lower, upper := new(big.Rat).SetFloat64(mins[axis]), new(big.Rat).SetFloat64(maxs[axis])
		if lo.Cmp(lower) > 0 {
			lower = lo
		}
		if hi.Cmp(upper) < 0 {
			upper = hi
		}
		if lower.Cmp(upper) >= 0 {
			r.Empty = true
			return r, nil
		}
		if e := budget.check(lower); e != nil {
			return r, e
		}
		if e := budget.check(upper); e != nil {
			return r, e
		}
		mins[axis], e = exactRepresentableFloat(lower)
		if e != nil {
			return r, e
		}
		maxs[axis], e = exactRepresentableFloat(upper)
		if e != nil {
			return r, e
		}
	}
	r.Domain = ParameterRectangle{UMin: mins[0], UMax: maxs[0], VMin: mins[1], VMax: maxs[1]}
	return r, nil
}
