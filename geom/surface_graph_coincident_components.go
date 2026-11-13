package geom

import (
	"context"
	"math/big"
	"sort"
)

// CoincidentPerimeterEdge retains the original partition cell/side ancestry.
// Its direction places coincident cells to its left in UV. Corner/ZeroSide IDs
// reference Partition; all rational endpoint coordinates belong to the caller.
type CoincidentPerimeterEdge struct {
	Cell                             int
	Edge                             SurfaceEdge
	ZeroSide, StartCorner, EndCorner int
	Start, End                       [2]*big.Rat
}

// CoincidentPerimeterCycle is CCW for an outer cycle and CW for a hole in UV.
// SignedArea is exact. Model XY orientation additionally depends on the chart's
// ProjectionOrientation in Partition.Graph.
type CoincidentPerimeterCycle struct {
	Edges      []CoincidentPerimeterEdge
	SignedArea *big.Rat
	Hole       bool
}
type CoincidentGraphComponent struct {
	Cells  []int
	Cycles []CoincidentPerimeterCycle
}

// SplineGraphCoincidentComponents preserves complete partition evidence and
// maximal edge-connected coincident regions. Complete also certifies manifold
// perimeter cycles. Corner-only component contacts or branched cycle vertices
// are reported as AmbiguousCorners and force ErrConvergence.
type SplineGraphCoincidentComponents struct {
	Complete         bool
	Partition        SplineGraphContactPartition
	Components       []CoincidentGraphComponent
	AmbiguousCorners []int
	ExactOperations  int
}

// FindSplineGraphCoincidentComponents revalidates the exact common-chart
// partition, removes interior shared sides, and links directed boundary edges
// by canonical exact corner identity. It neither rounds UV nor merges unrelated
// model points; no Boolean shell assembly is performed.
func FindSplineGraphCoincidentComponents(ctx context.Context, a, c *SurfacePatch, o GraphPointContactOptions) (r SplineGraphCoincidentComponents, err error) {
	partition, e := PartitionSplineGraphContact(ctx, a, c, o)
	r.Partition = partition
	r.ExactOperations = partition.ExactOperations
	if e != nil {
		return r, e
	}
	if !partition.Complete {
		return r, ErrConvergence
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations - partition.ExactOperations, maxBits: o.MaxCoefficientBits}
	if b.maxOperations < 1 {
		return r, ErrConvergence
	}
	defer func() { r.ExactOperations = partition.ExactOperations + b.operations }()
	neighbors := make([][4]int, len(partition.Cells))
	for i := range neighbors {
		neighbors[i] = [4]int{-1, -1, -1, -1}
	}
	for _, adj := range partition.Adjacencies {
		neighbors[adj.Cells[0]][adj.Edges[0]] = adj.Cells[1]
		neighbors[adj.Cells[1]][adj.Edges[1]] = adj.Cells[0]
	}
	visited := make([]bool, len(partition.Cells))
	cornerOwners := map[int]int{}
	ambiguous := map[int]bool{}
	// Local corner order LL,HL,HH,LH and CCW-oriented sides.
	endpoints := [4][2]int{{3, 0}, {1, 2}, {0, 1}, {2, 3}}
	for root, cell := range partition.Cells {
		if !cell.Coincident || visited[root] {
			continue
		}
		if e := ctx.Err(); e != nil {
			return r, e
		}
		component := CoincidentGraphComponent{}
		queue := []int{root}
		visited[root] = true
		for len(queue) > 0 {
			index := queue[0]
			queue = queue[1:]
			component.Cells = append(component.Cells, index)
			for _, other := range neighbors[index] {
				if other >= 0 && partition.Cells[other].Coincident && !visited[other] {
					visited[other] = true
					queue = append(queue, other)
				}
			}
		}
		sort.Ints(component.Cells)
		var edges []CoincidentPerimeterEdge
		for _, index := range component.Cells {
			item := partition.Cells[index]
			for side := SurfaceEdge(0); side < 4; side++ {
				other := neighbors[index][side]
				if other >= 0 && partition.Cells[other].Coincident {
					continue
				}
				ids := endpoints[side]
				first, last := item.ZeroCorners[ids[0]], item.ZeroCorners[ids[1]]
				if first < 0 || last < 0 || item.ZeroSides[side] < 0 {
					return r, ErrConvergence
				}
				edge := CoincidentPerimeterEdge{Cell: index, Edge: side, ZeroSide: item.ZeroSides[side], StartCorner: first, EndCorner: last}
				for axis := 0; axis < 2; axis++ {
					edge.Start[axis] = new(big.Rat).Set(partition.ZeroCorners[first].Parameters[axis])
					edge.End[axis] = new(big.Rat).Set(partition.ZeroCorners[last].Parameters[axis])
				}
				edges = append(edges, edge)
			}
		}
		outgoing, incoming := map[int][]int{}, map[int][]int{}
		for index, edge := range edges {
			outgoing[edge.StartCorner] = append(outgoing[edge.StartCorner], index)
			incoming[edge.EndCorner] = append(incoming[edge.EndCorner], index)
		}
		valid := true
		for corner, list := range outgoing {
			if len(list) != 1 || len(incoming[corner]) != 1 {
				ambiguous[corner] = true
				valid = false
			}
			if owner, ok := cornerOwners[corner]; ok && owner != len(r.Components) {
				ambiguous[corner] = true
			} else {
				cornerOwners[corner] = len(r.Components)
			}
		}
		for corner, list := range incoming {
			if len(list) != 1 || len(outgoing[corner]) != 1 {
				ambiguous[corner] = true
				valid = false
			}
		}
		if valid {
			used := make([]bool, len(edges))
			for first := range edges {
				if used[first] {
					continue
				}
				cycle := CoincidentPerimeterCycle{SignedArea: new(big.Rat)}
				current := first
				for {
					if used[current] {
						if current != first {
							return r, ErrConvergence
						}
						break
					}
					used[current] = true
					edge := edges[current]
					cycle.Edges = append(cycle.Edges, edge)
					cross := new(big.Rat).Sub(new(big.Rat).Mul(edge.Start[0], edge.End[1]), new(big.Rat).Mul(edge.Start[1], edge.End[0]))
					cycle.SignedArea.Add(cycle.SignedArea, cross)
					if e := b.check(cycle.SignedArea); e != nil {
						return r, e
					}
					current = outgoing[edge.EndCorner][0]
				}
				cycle.SignedArea.Quo(cycle.SignedArea, big.NewRat(2, 1))
				if cycle.SignedArea.Sign() == 0 {
					return r, ErrConvergence
				}
				cycle.Hole = cycle.SignedArea.Sign() < 0
				component.Cycles = append(component.Cycles, cycle)
			}
			sort.SliceStable(component.Cycles, func(i, j int) bool {
				if component.Cycles[i].Hole != component.Cycles[j].Hole {
					return !component.Cycles[i].Hole
				}
				x, y := component.Cycles[i].Edges[0], component.Cycles[j].Edges[0]
				if x.Cell != y.Cell {
					return x.Cell < y.Cell
				}
				return x.Edge < y.Edge
			})
		}
		r.Components = append(r.Components, component)
	}
	for corner := range ambiguous {
		r.AmbiguousCorners = append(r.AmbiguousCorners, corner)
	}
	sort.Ints(r.AmbiguousCorners)
	if len(r.AmbiguousCorners) > 0 {
		return r, ErrConvergence
	}
	r.Complete = true
	return r, nil
}
