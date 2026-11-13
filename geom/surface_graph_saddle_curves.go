package geom

import (
	"context"
	"math/big"
)

// QuadraticSaddleRayCurve carries a represented straight UV pcurve (XY of its
// Vec3 poles, Z=0) and exact spatial restrictions on both original supports.
// All three curves use t in [0,1], from the saddle to the identified terminal.
// Model curves may be curved, rational, and not exportable as float poles.
type QuadraticSaddleRayCurve struct {
	Ray            int
	ParameterCurve *BSpline
	ModelA, ModelB *ExactSplineCurve
	Agreement      ExactCurveComparison
}

// QuadraticSaddleCurves retains the complete clipped section even when some
// endpoints cannot be represented. Pending contains those original terminal
// identities/brackets. Complete means every ray has paired exact model evidence.
type QuadraticSaddleCurves struct {
	Complete        bool
	Section         QuadraticGraphSection
	Curves          []QuadraticSaddleRayCurve
	Pending         []GraphSaddleTerminal
	ExactOperations int
}

// BuildQuadraticSaddleRayCurves creates usable pcurves only from exactly
// represented UV endpoints. Every pair of spatial restrictions is independently
// compared as exact rational curves; no sampled model agreement is accepted.
func BuildQuadraticSaddleRayCurves(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, rectangle [2][2]*big.Rat, o GraphPointContactOptions) (r QuadraticSaddleCurves, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	section, e := ClipQuadraticGraphSaddleSection(ctx, a, c, uv, rectangle, o)
	r.Section = section
	r.ExactOperations = section.ExactOperations
	if e != nil {
		return r, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations - section.ExactOperations, maxBits: o.MaxCoefficientBits}
	if budget.maxOperations < 1 {
		return r, ErrConvergence
	}
	defer func() { r.ExactOperations = section.ExactOperations + budget.operations }()
	var center [2]float64
	represented := true
	for axis := 0; axis < 2; axis++ {
		center[axis], e = RepresentedExactScalar(uv[axis])
		represented = represented && e == nil
	}
	pa, e := trimPatchBudget(a, section.Certificate.Critical.Cell, budget)
	if e != nil {
		return r, e
	}
	pc, e := trimPatchBudget(c, section.Certificate.Critical.Cell, budget)
	if e != nil {
		return r, e
	}
	for index, terminal := range section.Terminals {
		if e := ctx.Err(); e != nil {
			r.Pending = append(r.Pending, section.Terminals[index:]...)
			return r, e
		}
		var endpoint [2]float64
		ready := represented && terminal.Exact
		if ready {
			for axis := 0; axis < 2; axis++ {
				endpoint[axis], e = RepresentedExactScalar(terminal.Lower[axis])
				ready = ready && e == nil
			}
		}
		if !ready {
			r.Pending = append(r.Pending, terminal)
			continue
		}
		curve, e := NewBezier([]Vec3{{X: center[0], Y: center[1]}, {X: endpoint[0], Y: endpoint[1]}}, nil)
		if e != nil {
			return r, e
		}
		q := QuadraticSaddleRayCurve{Ray: terminal.Ray, ParameterCurve: curve}
		for i, p := range []*SurfacePatch{pa, pc} {
			remaining := budget.maxOperations - budget.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			model, e := p.AffineParameterCurve(ctx, uv, terminal.Lower, TubeAffineLineOptions{MaxPoles: 4096, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return r, e
			}
			budget.operations += model.ExactOperations()
			if i == 0 {
				q.ModelA = model
			} else {
				q.ModelB = model
			}
		}
		remaining := budget.maxOperations - budget.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		q.Agreement, e = CompareExactCurvesContext(ctx, q.ModelA, q.ModelB, ExactCurveComparisonOptions{MaxSpans: 1, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += q.Agreement.ExactOperations
		if e != nil {
			return r, e
		}
		if !q.Agreement.Complete || !q.Agreement.Equal {
			return r, ErrConvergence
		}
		r.Curves = append(r.Curves, q)
	}
	if len(r.Pending) > 0 {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
