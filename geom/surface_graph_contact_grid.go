package geom

import (
	"context"
	"math/big"
	"sort"
)

// GraphContactGridCell describes one open rectangular complement of the contact
// grid. Bounds are exact original UV coordinates; Side is strict A.Z-B.Z.
type GraphContactGridCell struct {
	Lower, Upper [2]*big.Rat
	Side         int
}

// SplineGraphContactGrid is exhaustive only when Proven. Lines contains sorted
// interior contact coordinates by fixed axis. Vertices includes all grid nodes,
// including the original domain boundary, in U-major order. Each rational is
// caller-owned. Simple factors change sign at every line; no differentiability
// across source knots is claimed.
type SplineGraphContactGrid struct {
	Proven                        bool
	Domain                        ParameterRectangle
	Lines                         [2][]*big.Rat
	Cells                         []GraphContactGridCell
	Vertices                      [][2]*big.Rat
	Graph                         SurfaceInjectivity
	ExactOperations, Subdivisions int
}

// DiscoverSplineGraphContactGrid proves a common product of distinct rational
// U and V factors and a uniformly strict Bernstein quotient on every union cell.
// MaxCandidates bounds recovered factors per axis and MaxCells additionally
// bounds output rectangles. Empty Lines on an axis is supported. Unsupported
// roots or hidden additional factors cannot produce a false completeness claim:
// strict exact quotient positivity is required after all divisions. Candidate
// hints are unused. Failed discovery remains unresolved, not disjointness.
func DiscoverSplineGraphContactGrid(ctx context.Context, a, c *SurfacePatch, o GraphCrossingLinesOptions) (r SplineGraphContactGrid, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
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
	if o.MaxRootSubdivisions == 0 {
		o.MaxRootSubdivisions = 8192
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 128
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxRootSubdivisions < 1 || o.MaxCandidates < 1 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	d, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, d, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	if len(cells) == 0 {
		return r, ErrConvergence
	}
	r.Domain = d.Domain()
	bounds := [2][2]float64{{r.Domain.UMin, r.Domain.UMax}, {r.Domain.VMin, r.Domain.VMax}}
	var candidates [2][]*big.Rat
	for axis := 0; axis < 2; axis++ {
		p := cells[0]
		n, m, lo, hi := p.countU, p.countV, p.Domain().UMin, p.Domain().UMax
		if axis == 1 {
			n, m, lo, hi = p.countV, p.countU, p.Domain().VMin, p.Domain().VMax
		}
		var poly rationalPolynomial
		for j := 0; j < m && len(poly) == 0; j++ {
			values := make([]*big.Rat, n)
			for i := range values {
				index := i*p.countV + j
				if axis == 1 {
					index = j*p.countV + i
				}
				values[i] = p.net[index][2]
			}
			poly, e = bernsteinToPower(values, budget)
			if e != nil {
				return r, e
			}
		}
		if len(poly) == 0 {
			return r, ErrConvergence
		}
		origin := new(big.Rat).SetFloat64(lo)
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), origin)
		lower := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[axis][0]), origin), width)
		upper := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[axis][1]), origin), width)
		remaining := o.MaxRootSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		steps, e := isolatePolynomialRoots(poly, lower, upper, budget, remaining, func(lo, hi *big.Rat, multiplicity int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			root, e := recoverRationalPolynomialRoot(poly, lo, hi, budget)
			if e != nil {
				return false, e
			}
			if root == nil {
				return new(big.Rat).Mul(new(big.Rat).Sub(hi, lo), width).Cmp(big.NewRat(1, 1000000000000)) <= 0, nil
			}
			physical := new(big.Rat).Add(origin, new(big.Rat).Mul(width, root))
			if multiplicity != 1 {
				return true, nil
			}
			if physical.Cmp(new(big.Rat).SetFloat64(bounds[axis][0])) > 0 && physical.Cmp(new(big.Rat).SetFloat64(bounds[axis][1])) < 0 {
				if len(candidates[axis]) >= o.MaxCandidates {
					return false, ErrConvergence
				}
				candidates[axis] = append(candidates[axis], physical)
			}
			return true, nil
		})
		r.Subdivisions += steps
		if e != nil {
			return r, e
		}
	}

	for axis := 0; axis < 2; axis++ {
		sort.Slice(candidates[axis], func(i, j int) bool { return candidates[axis][i].Cmp(candidates[axis][j]) < 0 })
		for i := 1; i < len(candidates[axis]); i++ {
			if candidates[axis][i-1].Cmp(candidates[axis][i]) >= 0 {
				return r, ErrConvergence
			}
		}
	}
	if len(candidates[0])+1 > o.MaxCells/(len(candidates[1])+1) {
		return r, ErrConvergence
	}
	sign := 0
	for _, p := range cells {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		q := p
		for axis := 0; axis < 2; axis++ {
			for _, root := range candidates[axis] {
				q, e = divideGraphAxisBernstein(q, axis, root, budget)
				if e != nil {
					return r, e
				}
				if q == nil {
					return r, ErrConvergence
				}
			}
		}
		for _, h := range q.net {
			current := h[2].Sign()
			if current == 0 || (sign != 0 && current != sign) {
				return r, ErrConvergence
			}
			sign = current
		}
	}
	remaining := o.MaxOperations - budget.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	budget.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	var cuts [2][]*big.Rat
	for axis := 0; axis < 2; axis++ {
		cuts[axis] = append(cuts[axis], new(big.Rat).SetFloat64(bounds[axis][0]))
		for _, root := range candidates[axis] {
			r.Lines[axis] = append(r.Lines[axis], new(big.Rat).Set(root))
			cuts[axis] = append(cuts[axis], root)
		}
		cuts[axis] = append(cuts[axis], new(big.Rat).SetFloat64(bounds[axis][1]))
	}
	for i, u := range cuts[0] {
		for j, v := range cuts[1] {
			r.Vertices = append(r.Vertices, [2]*big.Rat{new(big.Rat).Set(u), new(big.Rat).Set(v)})
			if i == len(cuts[0])-1 || j == len(cuts[1])-1 {
				continue
			}
			side := sign
			if (len(candidates[0])+len(candidates[1])-i-j)%2 != 0 {
				side = -side
			}
			r.Cells = append(r.Cells, GraphContactGridCell{Lower: [2]*big.Rat{new(big.Rat).Set(u), new(big.Rat).Set(v)}, Upper: [2]*big.Rat{new(big.Rat).Set(cuts[0][i+1]), new(big.Rat).Set(cuts[1][j+1])}, Side: side})
		}
	}
	r.Proven = true
	return r, nil
}
