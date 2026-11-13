package geom

import (
	"context"
	"math/big"
	"sort"
)

// GraphSaddleRay is a proved branch parameterized by positive r<=Extent:
// UV[Axis]=center[Axis]+Direction*r and the other coordinate changes by
// Direction*slope*r. Slope is the unique quadratic-factor root in its interval.
// Opposite directions of the same Line meet only at the critical point.
type GraphSaddleRay struct {
	Line, Axis, Direction          int
	SlopeLower, SlopeUpper, Extent *big.Rat
}

// QuadraticGraphSaddleBranches proves exactly four contact germs at Critical.
// All zero sets in Cell are portions of the same two distinct straight UV lines;
// their model images need not be straight. Extents are certified inside Cell.
// ChartOrientation is the sign of the common XY chart Jacobian at the point.
// Embedding proves projected injectivity on Cell, so distinct UV rays cannot
// meet again there. No statement is made about unrelated knot cells.
type QuadraticGraphSaddleBranches struct {
	Embedding                                       SurfaceInjectivity
	Proven                                          bool
	Critical                                        SplineGraphCriticalContact
	Rays                                            []GraphSaddleRay
	ChartOrientation, ExactOperations, Subdivisions int
}

// CertifyQuadraticGraphSaddleBranches supports a genuinely total-degree-two
// homogeneous height numerator, allowing positive nonconstant rational weights.
// Distinct linear factors follow from its negative Hessian determinant. Exact
// root isolation (at most 8192 subdivisions) separates factor slopes; no
// sampled branch tracing is used.
func CertifyQuadraticGraphSaddleBranches(ctx context.Context, a, c *SurfacePatch, uv [2]*big.Rat, o GraphPointContactOptions) (r QuadraticGraphSaddleBranches, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	critical, e := ClassifySplineGraphCriticalContact(ctx, a, c, uv, o)
	r.Critical = critical
	r.ExactOperations = critical.ExactOperations
	if e != nil {
		return r, e
	}
	if critical.Kind != GraphCriticalSaddle {
		return r, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations - critical.ExactOperations, maxBits: o.MaxCoefficientBits}
	if budget.maxOperations < 1 {
		return r, ErrConvergence
	}
	defer func() { r.ExactOperations = critical.ExactOperations + budget.operations }()
	p, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	p, e = trimPatchBudget(p, critical.Cell, budget)
	if e != nil {
		return r, e
	}
	candidates, e := quadraticGraphWitnesses(p, budget)
	if e != nil {
		return r, e
	}
	quadratic := false
	for _, candidate := range candidates {
		quadratic = quadratic || (candidate[0].Cmp(uv[0]) == 0 && candidate[1].Cmp(uv[1]) == 0)
	}
	if !quadratic {
		return r, ErrConvergence
	}
	source, e := trimPatchBudget(a, critical.Cell, budget)
	if e != nil {
		return r, e
	}
	remaining := budget.maxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Embedding, e = CertifySurfaceInjectivity(source, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += r.Embedding.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Embedding.ProvenInjective {
		return r, ErrConvergence
	}
	h := critical.Hessian
	type line struct {
		axis   int
		lo, hi *big.Rat
	}
	var lines []line
	if h[1][1].Sign() == 0 {
		if h[0][1].Sign() == 0 {
			return r, ErrConvergence
		}
		slope := new(big.Rat).Quo(new(big.Rat).Neg(h[0][0]), new(big.Rat).Mul(big.NewRat(2, 1), h[0][1]))
		lines = []line{{1, new(big.Rat), new(big.Rat)}, {0, new(big.Rat).Set(slope), new(big.Rat).Set(slope)}}
	} else {
		poly := rationalPolynomial{h[0][0], new(big.Rat).Mul(big.NewRat(2, 1), h[0][1]), h[1][1]}
		bound, e := polynomialRootBound(poly, budget)
		if e != nil {
			return r, e
		}
		vertex := new(big.Rat).Quo(new(big.Rat).Neg(h[0][1]), h[1][1])
		steps, e := isolatePolynomialRoots(poly, new(big.Rat).Neg(bound), bound, budget, 8192, func(lo, hi *big.Rat, _ int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if lo.Cmp(vertex) <= 0 && hi.Cmp(vertex) >= 0 {
				return false, nil
			}
			lines = append(lines, line{0, new(big.Rat).Set(lo), new(big.Rat).Set(hi)})
			return true, nil
		})
		r.Subdivisions = steps
		if e != nil {
			return r, e
		}
		if len(lines) != 2 {
			return r, ErrConvergence
		}
		sort.Slice(lines, func(i, j int) bool { return lines[i].lo.Cmp(lines[j].lo) < 0 })
	}
	d := critical.Cell
	mins, maxs := [2]float64{d.UMin, d.VMin}, [2]float64{d.UMax, d.VMax}
	var margins [2]*big.Rat
	for axis := 0; axis < 2; axis++ {
		left := new(big.Rat).Sub(uv[axis], new(big.Rat).SetFloat64(mins[axis]))
		right := new(big.Rat).Sub(new(big.Rat).SetFloat64(maxs[axis]), uv[axis])
		margins[axis] = left
		if right.Cmp(left) < 0 {
			margins[axis] = right
		}
	}
	for i, l := range lines {
		maxSlope := new(big.Rat).Abs(l.lo)
		other := new(big.Rat).Abs(l.hi)
		if other.Cmp(maxSlope) > 0 {
			maxSlope = other
		}
		extent := new(big.Rat).Set(margins[l.axis])
		limited := new(big.Rat).Quo(margins[1-l.axis], new(big.Rat).Add(big.NewRat(1, 1), maxSlope))
		if limited.Cmp(extent) < 0 {
			extent = limited
		}
		extent.Quo(extent, big.NewRat(2, 1))
		if e := budget.check(extent); e != nil {
			return r, e
		}
		if extent.Sign() <= 0 {
			return r, ErrConvergence
		}
		for _, sign := range []int{-1, 1} {
			r.Rays = append(r.Rays, GraphSaddleRay{Line: i, Axis: l.axis, Direction: sign, SlopeLower: new(big.Rat).Set(l.lo), SlopeUpper: new(big.Rat).Set(l.hi), Extent: new(big.Rat).Set(extent)})
		}
	}
	r.ChartOrientation = critical.ProjectionDeterminant.Sign()
	r.Proven = true
	return r, nil
}
