package geom

import (
	"context"
	"math/big"
)

// GraphCriticalContactKind is the local Morse type of A.Z-B.Z.
type GraphCriticalContactKind int

const (
	GraphCriticalUnknown GraphCriticalContactKind = iota
	GraphCriticalMinimum
	GraphCriticalMaximum
	GraphCriticalSaddle
)

// SplineGraphCriticalContact proves a nondegenerate stationary zero of the
// height difference in a regular common-XY chart. The critical point is locally
// isolated; only minima/maxima are locally isolated intersection points.
// A saddle has locally crossing zero branches. No global ordering, exhaustive
// contact set, or certified neighborhood radius is implied.
type SplineGraphCriticalContact struct {
	Proven                             bool
	Kind                               GraphCriticalContactKind
	Parameters                         [2]*big.Rat
	Point                              [3]*big.Rat
	Hessian                            [2][2]*big.Rat
	Determinant, ProjectionDeterminant *big.Rat
	Cell                               ParameterRectangle
	ExactOperations                    int
}

// ClassifySplineGraphCriticalContact checks an exact rational witness strictly
// inside one common knot cell. At a stationary zero F=0,dF=0 with positive W,
// the height Hessian is exactly Hess(F)/W. Its inertia and the Morse lemma give
// the local classification. Degenerate Hessians and knot/boundary witnesses
// remain unresolved rather than receiving a one-sided smoothness assumption.
func ClassifySplineGraphCriticalContact(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, o GraphPointContactOptions) (r SplineGraphCriticalContact, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || uv[0] == nil || uv[1] == nil {
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
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	for _, x := range uv {
		if e := budget.check(x); e != nil {
			return r, e
		}
	}
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	var p *SurfacePatch
	for _, cell := range cells {
		d := cell.Domain()
		if uv[0].Cmp(new(big.Rat).SetFloat64(d.UMin)) > 0 && uv[0].Cmp(new(big.Rat).SetFloat64(d.UMax)) < 0 && uv[1].Cmp(new(big.Rat).SetFloat64(d.VMin)) > 0 && uv[1].Cmp(new(big.Rat).SetFloat64(d.VMax)) < 0 {
			p = cell
			break
		}
	}
	if p == nil {
		return r, ErrConvergence
	}
	d := p.Domain()
	r.Cell = d
	normalized := [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Sub(uv[0], new(big.Rat).SetFloat64(d.UMin)), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.UMax), new(big.Rat).SetFloat64(d.UMin))), new(big.Rat).Quo(new(big.Rat).Sub(uv[1], new(big.Rat).SetFloat64(d.VMin)), new(big.Rat).Sub(new(big.Rat).SetFloat64(d.VMax), new(big.Rat).SetFloat64(d.VMin)))}
	value := func(field, du, dv int) (*big.Rat, error) {
		values := make([]*big.Rat, len(p.net))
		for i, h := range p.net {
			values[i] = h[field]
		}
		nu, nv := p.countU, p.countV
		for axis, n := range []int{du, dv} {
			for k := 0; k < n; k++ {
				if axis == 0 && nu == 1 || axis == 1 && nv == 1 {
					return new(big.Rat), nil
				}
				var e error
				values, e = scalarPartial(values, nu, nv, axis == 0, d, budget)
				if e != nil {
					return nil, e
				}
				if axis == 0 {
					nu--
				} else {
					nv--
				}
			}
		}
		return tensorScalarValue(values, nu, nv, normalized[0], normalized[1], budget)
	}
	for _, derivative := range [][2]int{{0, 0}, {1, 0}, {0, 1}} {
		q, e := value(2, derivative[0], derivative[1])
		if e != nil {
			return r, e
		}
		if q.Sign() != 0 {
			return r, ErrConvergence
		}
	}
	weight, e := value(3, 0, 0)
	if e != nil {
		return r, e
	}
	if weight.Sign() <= 0 {
		return r, ErrConvergence
	}
	for _, entry := range [][4]int{{0, 0, 2, 0}, {0, 1, 1, 1}, {1, 0, 1, 1}, {1, 1, 0, 2}} {
		q, e := value(2, entry[2], entry[3])
		if e != nil {
			return r, e
		}
		q.Quo(q, weight)
		if e := budget.check(q); e != nil {
			return r, e
		}
		r.Hessian[entry[0]][entry[1]] = q
	}
	r.Determinant = new(big.Rat).Sub(new(big.Rat).Mul(r.Hessian[0][0], r.Hessian[1][1]), new(big.Rat).Mul(r.Hessian[0][1], r.Hessian[1][0]))
	if e := budget.check(r.Determinant); e != nil {
		return r, e
	}
	if r.Determinant.Sign() == 0 {
		return r, ErrConvergence
	}
	remaining := o.MaxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	jet, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += jet.ExactOperations
	if e != nil {
		return r, e
	}
	r.ProjectionDeterminant = new(big.Rat).Sub(new(big.Rat).Mul(jet.Du[0], jet.Dv[1]), new(big.Rat).Mul(jet.Du[1], jet.Dv[0]))
	if e := budget.check(r.ProjectionDeterminant); e != nil {
		return r, e
	}
	if r.ProjectionDeterminant.Sign() == 0 {
		return r, ErrConvergence
	}
	r.Kind = GraphCriticalSaddle
	if r.Determinant.Sign() > 0 {
		r.Kind = GraphCriticalMinimum
		if r.Hessian[0][0].Sign() < 0 {
			r.Kind = GraphCriticalMaximum
		}
	}
	r.Parameters = [2]*big.Rat{new(big.Rat).Set(uv[0]), new(big.Rat).Set(uv[1])}
	r.Point = jet.Point
	r.Proven = true
	return r, nil
}
