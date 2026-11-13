package geom

import (
	"context"
	"math/big"
)

// SurfaceProjectionIntervalInverse retains a proved unique projected inverse
// in exact rational UV bounds. Found proves existence (not just hull overlap).
// ModelBounds encloses its spatial point. Unresolved boundary contacts and work
// limits retain Pending rectangles; they never become false exclusion.
type SurfaceProjectionIntervalInverse struct {
	Found, Excluded, Complete     bool
	Lower, Upper                  [2]*big.Rat
	UV                            UVBox
	ModelBounds                   Box
	Injectivity                   SurfaceInjectivity
	Pending                       []ParameterRectangle
	VisitedCells, ExactOperations int
	KnotAwareTests                int
}

// InvertSurfaceProjectionInterval uses exact rational 2D Krawczyk tests on
// knot-aware rectangles. The hull of exact one-sided Bézier Jacobians bounds
// parameter-line integrals across continuous spline knots; strict self-mapping
// and contraction therefore prove existence even on interior knot lines.
// separately certified global projection injectivity proves global uniqueness.
// Query components and residual arithmetic remain exact. Candidate hints from
// RationalSurfaceInverseOptions are unused. Boundary roots can remain pending.
func InvertSurfaceProjectionInterval(ctx context.Context, p *SurfacePatch, query [2]*big.Rat, o RationalSurfaceInverseOptions) (r SurfaceProjectionIntervalInverse, err error) {
	if ctx == nil || p == nil || len(p.net) == 0 || query[0] == nil || query[1] == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.ProjectionAxes == ([2]int{}) {
		o.ProjectionAxes = [2]int{0, 1}
	}
	if o.ProjectionAxes[0] < 0 || o.ProjectionAxes[0] > 2 || o.ProjectionAxes[1] < 0 || o.ProjectionAxes[1] > 2 || o.ProjectionAxes[0] == o.ProjectionAxes[1] {
		return r, ErrInvalid
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 12
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxDepth < 0 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	for _, x := range query {
		if e := b.check(x); e != nil {
			return r, e
		}
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Injectivity, err = CertifySurfaceInjectivity(p, SurfaceInjectivityOptions{ProjectionAxes: o.ProjectionAxes, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += r.Injectivity.ExactOperations
	if err != nil {
		return r, err
	}
	if !r.Injectivity.ProvenInjective {
		return r, ErrConvergence
	}
	type cell struct {
		p     *SurfacePatch
		depth int
	}
	stack := []cell{{p, 0}}
	pending := func(item *cell) {
		if item != nil {
			r.Pending = append(r.Pending, item.p.Domain())
		}
		for _, x := range stack {
			r.Pending = append(r.Pending, x.p.Domain())
		}
	}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			pending(nil)
			return r, e
		}
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		r.VisitedCells++
		if r.VisitedCells > o.MaxCells {
			pending(&item)
			return r, ErrConvergence
		}
		excluded := false
		for axis, coordinate := range o.ProjectionAxes {
			positive, negative := true, true
			for _, h := range item.p.net {
				residual := new(big.Rat).Sub(h[coordinate], new(big.Rat).Mul(query[axis], h[3]))
				if e := b.check(residual); e != nil {
					pending(&item)
					return r, e
				}
				positive = positive && residual.Sign() > 0
				negative = negative && residual.Sign() < 0
			}
			excluded = excluded || positive || negative
		}
		if excluded {
			continue
		}
		knot, axis, internal := patchSplitChoice(item.p)
		{
			if internal {
				r.KnotAwareTests++
			}
			box, found, outside, e := projectionKrawczykContext(ctx, item.p, query, o.ProjectionAxes, o.MaxCells, b)
			if e != nil {
				pending(&item)
				return r, e
			}
			if outside {
				continue
			}
			if found {
				for j := 0; j < 2; j++ {
					r.Lower[j] = new(big.Rat).Set(box[j].lo)
					r.Upper[j] = new(big.Rat).Set(box[j].hi)
				}
				r.UV = UVBox{Min: Vec2{rationalFloatBound(r.Lower[0], true), rationalFloatBound(r.Lower[1], true)}, Max: Vec2{rationalFloatBound(r.Upper[0], false), rationalFloatBound(r.Upper[1], false)}}
				r.ModelBounds = item.p.bounds
				r.Found = true
				r.Complete = true
				r.Pending = nil
				return r, nil
			}
		}
		if item.depth >= o.MaxDepth {
			r.Pending = append(r.Pending, item.p.Domain())
			continue
		}
		if !internal {
			d := item.p.Domain()
			axis = d.UMax-d.UMin >= d.VMax-d.VMin
			lo, hi := d.UMin, d.UMax
			if !axis {
				lo, hi = d.VMin, d.VMax
			}
			knot = lo + (hi-lo)/2
			if knot <= lo || knot >= hi {
				r.Pending = append(r.Pending, d)
				continue
			}
		}
		left, right, e := item.p.split(knot, axis, b)
		if e != nil {
			pending(&item)
			return r, e
		}
		stack = append(stack, cell{right, item.depth + 1}, cell{left, item.depth + 1})
	}
	if len(r.Pending) > 0 {
		return r, ErrConvergence
	}
	r.Excluded = true
	r.Complete = true
	return r, nil
}

func projectionKrawczykContext(ctx context.Context, p *SurfacePatch, query [2]*big.Rat, axes [2]int, maxCells int, b *polynomialBudget) (image [2]exactInterval, found, excluded bool, err error) {
	d := p.Domain()
	domain := [2]exactInterval{{new(big.Rat).SetFloat64(d.UMin), new(big.Rat).SetFloat64(d.UMax)}, {new(big.Rat).SetFloat64(d.VMin), new(big.Rat).SetFloat64(d.VMax)}}
	var center [2]*big.Rat
	for i := 0; i < 2; i++ {
		center[i] = new(big.Rat).Mul(big.NewRat(1, 2), new(big.Rat).Add(domain[i].lo, domain[i].hi))
	}
	var point [3]*big.Rat
	var derivative [3][2]*big.Rat
	var jacobian [3][2]exactInterval
	if _, _, multi := patchSplitChoice(p); multi {
		remaining := b.maxOperations - b.operations
		if remaining < 1 {
			return image, false, false, ErrConvergence
		}
		jet, e := p.EvaluateRational(ctx, center[0], center[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		b.operations += jet.ExactOperations
		if e != nil {
			return image, false, false, e
		}
		point = jet.Point
		for axis := 0; axis < 3; axis++ {
			derivative[axis] = [2]*big.Rat{jet.Du[axis], jet.Dv[axis]}
		}
		cells, e := graphContactBezierCells(ctx, p, maxCells, b)
		if e != nil {
			return image, false, false, e
		}
		for index, cell := range cells {
			if e := ctx.Err(); e != nil {
				return image, false, false, e
			}
			_, bounds, e := surfaceDifferentialIntervals(cell, -1, 0, b)
			if e != nil {
				return image, false, false, e
			}
			for axis := 0; axis < 3; axis++ {
				for coordinate := 0; coordinate < 2; coordinate++ {
					bound := bounds[axis][coordinate]
					if index == 0 {
						jacobian[axis][coordinate] = exactInterval{new(big.Rat).Set(bound.lo), new(big.Rat).Set(bound.hi)}
					} else {
						if bound.lo.Cmp(jacobian[axis][coordinate].lo) < 0 {
							jacobian[axis][coordinate].lo.Set(bound.lo)
						}
						if bound.hi.Cmp(jacobian[axis][coordinate].hi) > 0 {
							jacobian[axis][coordinate].hi.Set(bound.hi)
						}
					}
				}
			}
		}
	} else {
		var e error
		point, derivative, e = exactSurfaceJet(p, center[0], center[1], b)
		if e != nil {
			return image, false, false, e
		}
		_, jacobian, e = surfaceDifferentialIntervals(p, -1, 0, b)
		if e != nil {
			return image, false, false, e
		}
	}
	matrix := [2][2]*big.Rat{derivative[axes[0]], derivative[axes[1]]}
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(matrix[0][0], matrix[1][1]), new(big.Rat).Mul(matrix[0][1], matrix[1][0]))
	if e := b.check(determinant); e != nil {
		return image, false, false, e
	}
	if determinant.Sign() == 0 {
		return image, false, false, nil
	}
	inverse := [2][2]*big.Rat{{new(big.Rat).Quo(matrix[1][1], determinant), new(big.Rat).Quo(new(big.Rat).Neg(matrix[0][1]), determinant)}, {new(big.Rat).Quo(new(big.Rat).Neg(matrix[1][0]), determinant), new(big.Rat).Quo(matrix[0][0], determinant)}}

	contraction := true
	inclusion := true
	for i := 0; i < 2; i++ {
		base := new(big.Rat).Set(center[i])
		for k := 0; k < 2; k++ {
			base.Sub(base, new(big.Rat).Mul(inverse[i][k], new(big.Rat).Sub(point[axes[k]], query[k])))
		}
		if e := b.check(base); e != nil {
			return image, false, false, e
		}
		value := exactInterval{new(big.Rat).Set(base), new(big.Rat).Set(base)}
		rowNorm := new(big.Rat)
		for j := 0; j < 2; j++ {
			m := exactInterval{new(big.Rat), new(big.Rat)}
			if i == j {
				m.lo.SetInt64(1)
				m.hi.SetInt64(1)
			}
			for k := 0; k < 2; k++ {
				product, e := intervalMultiply(exactInterval{inverse[i][k], inverse[i][k]}, jacobian[axes[k]][j], b)
				if e != nil {
					return image, false, false, e
				}
				m, e = intervalSubtract(m, product, b)
				if e != nil {
					return image, false, false, e
				}
			}
			absolute := new(big.Rat).Abs(m.lo)
			if x := new(big.Rat).Abs(m.hi); x.Cmp(absolute) > 0 {
				absolute = x
			}
			rowNorm.Add(rowNorm, absolute)
			displacement := exactInterval{new(big.Rat).Sub(domain[j].lo, center[j]), new(big.Rat).Sub(domain[j].hi, center[j])}
			term, e := intervalMultiply(m, displacement, b)
			if e != nil {
				return image, false, false, e
			}
			value, e = boundedExactInterval(new(big.Rat).Add(value.lo, term.lo), new(big.Rat).Add(value.hi, term.hi), b)
			if e != nil {
				return image, false, false, e
			}
		}
		if e := b.check(rowNorm); e != nil {
			return image, false, false, e
		}
		contraction = contraction && rowNorm.Cmp(big.NewRat(1, 1)) < 0
		if value.hi.Cmp(domain[i].lo) < 0 || value.lo.Cmp(domain[i].hi) > 0 {
			return image, false, true, nil
		}
		inclusion = inclusion && value.lo.Cmp(domain[i].lo) > 0 && value.hi.Cmp(domain[i].hi) < 0
		image[i] = value
	}
	return image, inclusion && contraction, false, nil
}
