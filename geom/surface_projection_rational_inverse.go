package geom

import (
	"context"
	"math/big"
)

// RationalSurfaceInverseOptions chooses two distinct world coordinates; [0,0]
// defaults to X,Y. Candidates are exact original UV proposals, not approximate
// root assertions. Limits are cumulative: defaults depth12,4096 cells,8M exact
// operations,16384 bits. A corner-affine inverse proposal is always verified;
// empty Candidates additionally enable exact cell-center proposals during hull
// subdivision. All proposals must pass exact evaluation before Found is set.
type RationalSurfaceInverseOptions struct {
	ProjectionAxes                                        [2]int
	Candidates                                            [][2]*big.Rat
	MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int
}

// RationalSurfaceInverse certifies a unique preimage when Found, or absence
// when Excluded. Pending retains all nonexcluded parameter cells after bounded
// subdivision. Failed candidates never imply absence. Rational outputs are
// caller-owned; PointBounds and UV are outward representatives.
type RationalSurfaceInverse struct {
	Found, Excluded, Complete                       bool
	Parameters                                      [2]*big.Rat
	Point                                           [3]*big.Rat
	UV                                              UVBox
	PointBounds                                     Box
	Injectivity                                     SurfaceInjectivity
	Pending                                         []ParameterRectangle
	CandidatesTested, VisitedCells, ExactOperations int
}

// InvertSurfaceProjectionRational first proves projection injectivity, then
// verifies rational preimage proposals by exact evaluation. This proves both
// existence and uniqueness even on original boundaries and interior knots.
// Without a matching proposal, exact positive-weight homogeneous hull tests
// conservatively subdivide the domain. Algebraic/unproposed roots remain pending.
func InvertSurfaceProjectionRational(ctx context.Context, p *SurfacePatch, query [2]*big.Rat, o RationalSurfaceInverseOptions) (r RationalSurfaceInverse, err error) {
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
	for _, candidate := range o.Candidates {
		for _, x := range candidate {
			if x == nil {
				return r, ErrInvalid
			}
			if e := b.check(x); e != nil {
				return r, e
			}
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
	d := p.Domain()
	limits := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	try := func(candidate [2]*big.Rat) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		r.CandidatesTested++
		inside := true
		for axis, x := range candidate {
			inside = inside && x.Cmp(new(big.Rat).SetFloat64(limits[axis][0])) >= 0 && x.Cmp(new(big.Rat).SetFloat64(limits[axis][1])) <= 0
		}
		if !inside {
			return false, nil
		}
		remaining = o.MaxOperations - b.operations
		if remaining < 1 {
			return false, ErrConvergence
		}
		jet, e := p.EvaluateRational(ctx, candidate[0], candidate[1], SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += jet.ExactOperations
		if e != nil {
			return false, e
		}
		if jet.Point[o.ProjectionAxes[0]].Cmp(query[0]) != 0 || jet.Point[o.ProjectionAxes[1]].Cmp(query[1]) != 0 {
			return false, nil
		}
		r.Parameters = [2]*big.Rat{new(big.Rat).Set(candidate[0]), new(big.Rat).Set(candidate[1])}
		r.Point = jet.Point
		r.PointBounds = jet.PointBounds
		r.UV = UVBox{Min: Vec2{rationalFloatBound(candidate[0], true), rationalFloatBound(candidate[1], true)}, Max: Vec2{rationalFloatBound(candidate[0], false), rationalFloatBound(candidate[1], false)}}
		r.Found = true
		r.Complete = true
		return true, nil
	}
	for _, candidate := range o.Candidates {
		if found, e := try(candidate); e != nil || found {
			return r, e
		}
	}
	// A corner-affine solve is only a proposal for a nonlinear patch. Exact
	// evaluation plus global injectivity remains the sole acceptance criterion.
	proposal, e := rationalProjectionCornerProposal(ctx, p, query, o.ProjectionAxes, b)
	if e != nil {
		return r, e
	}
	if proposal[0] != nil {
		if found, e := try(proposal); e != nil || found {
			return r, e
		}
	}
	type cell struct {
		p     *SurfacePatch
		depth int
	}
	stack := []cell{{p, 0}}
	pendingStack := func() {
		for _, item := range stack {
			r.Pending = append(r.Pending, item.p.Domain())
		}
	}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			pendingStack()
			return r, e
		}
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		r.VisitedCells++
		if r.VisitedCells > o.MaxCells {
			r.Pending = append(r.Pending, item.p.Domain())
			pendingStack()
			return r, ErrConvergence
		}
		excluded := false
		for axis, coordinate := range o.ProjectionAxes {
			positive, negative := true, true
			for _, h := range item.p.net {
				residual := new(big.Rat).Sub(h[coordinate], new(big.Rat).Mul(query[axis], h[3]))
				if e := b.check(residual); e != nil {
					r.Pending = append(r.Pending, item.p.Domain())
					pendingStack()
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
		if len(o.Candidates) == 0 {
			cd := item.p.Domain()
			center := [2]*big.Rat{new(big.Rat).Mul(big.NewRat(1, 2), new(big.Rat).Add(new(big.Rat).SetFloat64(cd.UMin), new(big.Rat).SetFloat64(cd.UMax))), new(big.Rat).Mul(big.NewRat(1, 2), new(big.Rat).Add(new(big.Rat).SetFloat64(cd.VMin), new(big.Rat).SetFloat64(cd.VMax)))}
			if found, e := try(center); e != nil || found {
				if e != nil {
					r.Pending = append(r.Pending, item.p.Domain())
					pendingStack()
				}
				return r, e
			}
		}
		if item.depth >= o.MaxDepth {
			r.Pending = append(r.Pending, item.p.Domain())
			continue
		}
		knot, axis, internal := patchSplitChoice(item.p)
		if !internal {
			domain := item.p.Domain()
			axis = domain.UMax-domain.UMin >= domain.VMax-domain.VMin
			lo, hi := domain.UMin, domain.UMax
			if !axis {
				lo, hi = domain.VMin, domain.VMax
			}
			knot = lo + (hi-lo)/2
			if knot <= lo || knot >= hi {
				r.Pending = append(r.Pending, domain)
				continue
			}
		}
		left, right, e := item.p.split(knot, axis, b)
		if e != nil {
			r.Pending = append(r.Pending, item.p.Domain())
			pendingStack()
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

func rationalProjectionCornerProposal(ctx context.Context, p *SurfacePatch, query [2]*big.Rat, axes [2]int, b *polynomialBudget) (uv [2]*big.Rat, err error) {
	d := p.Domain()
	parameters := [][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMin, d.VMax}}
	var points [3][3]*big.Rat
	for i, parameter := range parameters {
		remaining := b.maxOperations - b.operations
		if remaining < 1 {
			return uv, ErrConvergence
		}
		jet, e := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(parameter[0]), new(big.Rat).SetFloat64(parameter[1]), SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
		b.operations += jet.ExactOperations
		if e != nil {
			return uv, e
		}
		points[i] = jet.Point
	}
	u0, u1 := new(big.Rat).Sub(points[1][axes[0]], points[0][axes[0]]), new(big.Rat).Sub(points[1][axes[1]], points[0][axes[1]])
	v0, v1 := new(big.Rat).Sub(points[2][axes[0]], points[0][axes[0]]), new(big.Rat).Sub(points[2][axes[1]], points[0][axes[1]])
	q0, q1 := new(big.Rat).Sub(query[0], points[0][axes[0]]), new(big.Rat).Sub(query[1], points[0][axes[1]])
	determinant := new(big.Rat).Sub(new(big.Rat).Mul(u0, v1), new(big.Rat).Mul(u1, v0))
	if e := b.check(determinant); e != nil {
		return uv, e
	}
	if determinant.Sign() == 0 {
		return uv, nil
	}
	u := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(q0, v1), new(big.Rat).Mul(q1, v0)), determinant)
	v := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(u0, q1), new(big.Rat).Mul(u1, q0)), determinant)
	uv = [2]*big.Rat{parameterMap(ParameterSpan{Start: d.UMin, End: d.UMax}, u), parameterMap(ParameterSpan{Start: d.VMin, End: d.VMax}, v)}
	for _, x := range uv {
		if e := b.check(x); e != nil {
			return [2]*big.Rat{}, e
		}
	}
	return uv, nil
}
