package geom

import (
	"context"
	"math/big"
	"sort"
)

// WrappedRectangleCell is a positive-area cell in original A coordinates.
// Owners bit0/bit1 denotes membership in the original A/B rectangle.
type WrappedRectangleCell struct {
	A      [2][2]*big.Rat
	Owners uint8
}

// WrappedRectangleEdge is directed with retained material to its left in A's
// chart. Owner UV endpoint pairs are populated only for that boundary owner.
// They retain original coordinates and traversal, including reversed B charts.
type WrappedRectangleEdge struct {
	Start, End [2]*big.Rat
	Owners     uint8
	AUV, BUV   [2][2]*big.Rat
}

// WrappedRectangleArrangement proves quotient-chart topology only. Intrinsic
// self-intersection of the model surface remains possible and requires an
// independent embedding proof before interpreting these cycles as solid faces.
type WrappedRectangleArrangement struct {
	Complete                       bool
	Cells                          []WrappedRectangleCell
	Union, Intersection            []WrappedRectangleEdge
	TargetPeriodShift              [2]*big.Int
	CanceledEdges, ExactOperations int
}

// ArrangeRectanglesRational requires one unique positive-area lifted overlap.
// The union hull must be strictly shorter than each wrapped source period, so
// no hidden second quotient occurrence is treated as a separate physical branch.
// Adjacent-only, corner-only, disjoint and multi-occurrence inputs reject. The
// exact union grid has at most nine cells. Shared full grid edges are canceled;
// each retained cycle preserves original boundary ancestry and owner UV units.
func (r *WrappedSurfaceChartRelation) ArrangeRectanglesRational(ctx context.Context, a, b [2][2]*big.Rat, o WrappedRectangleOverlapOptions) (WrappedRectangleArrangement, error) {
	var zero WrappedRectangleArrangement
	overlap, e := r.OverlapRectanglesRational(ctx, a, b, o)
	if e != nil {
		return zero, e
	}
	if len(overlap.Components) != 1 || overlap.Components[0].Dimension != 2 {
		return zero, ErrDomain
	}
	maxOps, maxBits := o.MaxOperations, o.MaxCoefficientBits
	if maxOps == 0 {
		maxOps = 1000000
	}
	if maxBits == 0 {
		maxBits = 16384
	}
	budget := &polynomialBudget{maxOperations: maxOps, maxBits: maxBits, operations: overlap.ExactOperations}
	shifts := overlap.Components[0].TargetPeriodShift
	var mappedB [2][2]*big.Rat
	for i, j := range r.axis {
		shift := new(big.Rat)
		if p := r.target.period[j]; p != nil {
			shift.Mul(new(big.Rat).SetInt(shifts[j]), p)
		}
		for end := 0; end < 2; end++ {
			mappedB[i][end] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Add(b[j][end], shift), r.offset[i]), r.scale[i])
			if e = budget.check(mappedB[i][end]); e != nil {
				return zero, e
			}
		}
		if mappedB[i][0].Cmp(mappedB[i][1]) > 0 {
			mappedB[i][0], mappedB[i][1] = mappedB[i][1], mappedB[i][0]
		}
	}
	var grid [2][]*big.Rat
	for i := 0; i < 2; i++ {
		values := []*big.Rat{a[i][0], a[i][1], mappedB[i][0], mappedB[i][1]}
		sort.Slice(values, func(j, k int) bool { return values[j].Cmp(values[k]) < 0 })
		for _, v := range values {
			if len(grid[i]) == 0 || v.Cmp(grid[i][len(grid[i])-1]) != 0 {
				grid[i] = append(grid[i], new(big.Rat).Set(v))
			}
		}
		if p := r.source.period[i]; p != nil {
			width := new(big.Rat).Sub(grid[i][len(grid[i])-1], grid[i][0])
			if e = budget.check(width); e != nil {
				return zero, e
			}
			if width.Cmp(p) >= 0 {
				return zero, ErrDomain
			}
		}
	}
	out := WrappedRectangleArrangement{TargetPeriodShift: [2]*big.Int{new(big.Int).Set(shifts[0]), new(big.Int).Set(shifts[1])}}
	for u := 0; u+1 < len(grid[0]); u++ {
		for v := 0; v+1 < len(grid[1]); v++ {
			cell := WrappedRectangleCell{A: [2][2]*big.Rat{{grid[0][u], grid[0][u+1]}, {grid[1][v], grid[1][v+1]}}}
			for owner, rect := range [2][2][2]*big.Rat{a, mappedB} {
				inside := true
				for axis := 0; axis < 2; axis++ {
					inside = inside && cell.A[axis][0].Cmp(rect[axis][0]) >= 0 && cell.A[axis][1].Cmp(rect[axis][1]) <= 0
				}
				if inside {
					cell.Owners |= 1 << owner
				}
			}
			if cell.Owners != 0 {
				out.Cells = append(out.Cells, cell)
			}
		}
	}
	key := func(p [2]*big.Rat) string { return p[0].RatString() + "," + p[1].RatString() }
	boundary := func(intersection bool) ([]WrappedRectangleEdge, error) {
		edges := map[string]WrappedRectangleEdge{}
		for _, cell := range out.Cells {
			if intersection && cell.Owners != 3 {
				continue
			}
			points := [4][2]*big.Rat{{cell.A[0][0], cell.A[1][0]}, {cell.A[0][1], cell.A[1][0]}, {cell.A[0][1], cell.A[1][1]}, {cell.A[0][0], cell.A[1][1]}}
			for k, start := range points {
				end := points[(k+1)%4]
				forward, reverse := key(start)+"|"+key(end), key(end)+"|"+key(start)
				if _, ok := edges[reverse]; ok {
					delete(edges, reverse)
					out.CanceledEdges++
					continue
				}
				if _, ok := edges[forward]; ok {
					return nil, ErrConvergence
				}
				edges[forward] = WrappedRectangleEdge{Start: start, End: end}
			}
		}
		var list []WrappedRectangleEdge
		for _, edge := range edges {
			for owner, rect := range [2][2][2]*big.Rat{a, mappedB} {
				on := false
				for axis := 0; axis < 2; axis++ {
					other := 1 - axis
					for side := 0; side < 2; side++ {
						if edge.Start[axis].Cmp(rect[axis][side]) == 0 && edge.End[axis].Cmp(rect[axis][side]) == 0 && edge.Start[other].Cmp(rect[other][0]) >= 0 && edge.Start[other].Cmp(rect[other][1]) <= 0 && edge.End[other].Cmp(rect[other][0]) >= 0 && edge.End[other].Cmp(rect[other][1]) <= 0 {
							on = true
						}
					}
				}
				if on {
					edge.Owners |= 1 << owner
				}
			}
			if edge.Owners == 0 {
				return nil, ErrConvergence
			}
			for end, p := range [2][2]*big.Rat{edge.Start, edge.End} {
				if edge.Owners&1 != 0 {
					edge.AUV[end] = [2]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1])}
				}
				if edge.Owners&2 != 0 {
					for i, j := range r.axis {
						q := new(big.Rat).Add(r.offset[i], new(big.Rat).Mul(r.scale[i], p[i]))
						if period := r.target.period[j]; period != nil {
							q.Sub(q, new(big.Rat).Mul(new(big.Rat).SetInt(shifts[j]), period))
						}
						if e := budget.check(q); e != nil {
							return nil, e
						}
						edge.BUV[end][j] = q
					}
				}
			}
			list = append(list, edge)
		}
		if len(list) == 0 {
			return nil, ErrConvergence
		}
		sort.Slice(list, func(i, j int) bool {
			if c := list[i].Start[0].Cmp(list[j].Start[0]); c != 0 {
				return c < 0
			}
			return list[i].Start[1].Cmp(list[j].Start[1]) < 0
		})
		next := map[string]int{}
		for i, edge := range list {
			k := key(edge.Start)
			if _, exists := next[k]; exists {
				return nil, ErrConvergence
			}
			next[k] = i
		}
		ordered := make([]WrappedRectangleEdge, 0, len(list))
		used := make([]bool, len(list))
		at := 0
		for len(ordered) < len(list) {
			if used[at] {
				return nil, ErrConvergence
			}
			used[at] = true
			edge := list[at]
			ordered = append(ordered, edge)
			var ok bool
			at, ok = next[key(edge.End)]
			if !ok {
				return nil, ErrConvergence
			}
		}
		if at != 0 {
			return nil, ErrConvergence
		}
		return ordered, nil
	}
	out.Union, e = boundary(false)
	if e != nil {
		return zero, e
	}
	out.Intersection, e = boundary(true)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	out.Complete = true
	out.ExactOperations = budget.operations
	return out, nil
}
