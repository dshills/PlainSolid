package geom

import (
	"context"
	"math/big"
)

// WrappedRectangleEmbeddingOptions shares one exact arithmetic quota across
// arrangement, support restriction and projected injectivity. ProjectionAxes
// and subdivision limits have SurfaceInjectivityOptions semantics.
type WrappedRectangleEmbeddingOptions struct {
	Arrangement        WrappedRectangleOverlapOptions
	ProjectionAxes     [2]int
	MaxDepth, MaxCells int
}

// EmbeddedWrappedRectangleArrangement proves the union and intersection chart
// cycles are embedded in model space. SourceHull is a restriction of the
// immutable fundamental support. Original A parameters equal restricted chart
// parameters plus SourcePeriodShift*source period. Original owner intervals in
// Arrangement are unchanged. No surrounding shell or trim topology is asserted.
type EmbeddedWrappedRectangleArrangement struct {
	Arrangement       WrappedRectangleArrangement
	SourceHull        ParameterRectangle
	SourcePeriodShift [2]*big.Int
	Injectivity       SurfaceInjectivity
	ExactOperations   int
}

// CertifyEmbeddedRectangleArrangement is an opt-in physical embedding guard.
// It requires the entire convex arrangement hull to fit one fundamental chart
// after one whole-period translation per wrapped axis. Restriction cuts must
// be exactly represented, and a bounded projected injectivity certificate must
// hold over the full hull. Unsupported seams, projections and limits publish
// no partial embedding. This never changes the raw quotient arrangement API.
func (r *WrappedSurfaceChartRelation) CertifyEmbeddedRectangleArrangement(ctx context.Context, a, b [2][2]*big.Rat, o WrappedRectangleEmbeddingOptions) (EmbeddedWrappedRectangleArrangement, error) {
	var zero EmbeddedWrappedRectangleArrangement
	arrangement, e := r.ArrangeRectanglesRational(ctx, a, b, o.Arrangement)
	if e != nil {
		return zero, e
	}
	maxOps, maxBits := o.Arrangement.MaxOperations, o.Arrangement.MaxCoefficientBits
	if maxOps == 0 {
		maxOps = 1000000
	}
	if maxBits == 0 {
		maxBits = 16384
	}
	budget := &polynomialBudget{maxOperations: maxOps, maxBits: maxBits, operations: arrangement.ExactOperations}
	var hull [2][2]*big.Rat
	for _, cell := range arrangement.Cells {
		for axis := 0; axis < 2; axis++ {
			for end := 0; end < 2; end++ {
				q := cell.A[axis][end]
				if hull[axis][end] == nil || (end == 0 && q.Cmp(hull[axis][end]) < 0) || (end == 1 && q.Cmp(hull[axis][end]) > 0) {
					hull[axis][end] = new(big.Rat).Set(q)
				}
			}
		}
	}
	d := r.source.Domain()
	domain := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
	shifts := [2]*big.Int{new(big.Int), new(big.Int)}
	var represented [2][2]float64
	for axis := 0; axis < 2; axis++ {
		if period := r.source.period[axis]; period != nil {
			turns := new(big.Rat).Quo(new(big.Rat).Sub(hull[axis][0], new(big.Rat).SetFloat64(domain[axis][0])), period)
			if e = budget.check(turns); e != nil {
				return zero, e
			}
			rem := new(big.Int)
			shifts[axis].QuoRem(turns.Num(), turns.Denom(), rem)
			if rem.Sign() < 0 {
				shifts[axis].Sub(shifts[axis], big.NewInt(1))
			}
			shift := new(big.Rat).Mul(new(big.Rat).SetInt(shifts[axis]), period)
			if e = budget.check(shift); e != nil {
				return zero, e
			}
			for end := 0; end < 2; end++ {
				hull[axis][end].Sub(hull[axis][end], shift)
				if e = budget.check(hull[axis][end]); e != nil {
					return zero, e
				}
			}
		}
		if hull[axis][0].Cmp(new(big.Rat).SetFloat64(domain[axis][0])) < 0 || hull[axis][1].Cmp(new(big.Rat).SetFloat64(domain[axis][1])) > 0 {
			return zero, ErrConvergence
		}
		for end := 0; end < 2; end++ {
			represented[axis][end], e = exactRepresentableFloat(hull[axis][end])
			if e != nil {
				return zero, e
			}
		}
	}
	rectangle := ParameterRectangle{UMin: represented[0][0], UMax: represented[0][1], VMin: represented[1][0], VMax: represented[1][1]}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	patch, e := trimPatchBudget(r.source.patch, rectangle, budget)
	if e != nil {
		return zero, e
	}
	remaining := maxOps - budget.operations
	if remaining < 1 {
		return zero, ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	proof, e := CertifySurfaceInjectivity(patch, SurfaceInjectivityOptions{ProjectionAxes: o.ProjectionAxes, MaxDepth: o.MaxDepth, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: maxBits})
	if e != nil {
		return zero, e
	}
	if !proof.ProvenInjective {
		return zero, ErrConvergence
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return EmbeddedWrappedRectangleArrangement{Arrangement: arrangement, SourceHull: rectangle, SourcePeriodShift: shifts, Injectivity: proof, ExactOperations: budget.operations + proof.ExactOperations}, nil
}
