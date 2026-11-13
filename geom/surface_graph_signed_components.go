package geom

import (
	"context"
	"math/big"
	"sort"
)

// SignedPerimeterEdge has selected signed cells to its left in UV. ZeroSide
// identifies an exact contact segment in Partition, or -1 if its relative
// interior is strictly signed. Exterior marks the original chart boundary.
// StartCorner/EndCorner reference Corners, including nonzero model locations.
type SignedPerimeterEdge struct {
	Cell                             int
	Edge                             SurfaceEdge
	ZeroSide, StartCorner, EndCorner int
	Exterior                         bool
	Start, End                       [2]*big.Rat
}
type SignedPerimeterCycle struct {
	Edges      []SignedPerimeterEdge
	SignedArea *big.Rat
	Hole       bool
}

// SignedGraphComponent is a union of closed same-side cells. InternalZeroSides
// preserves contact loci between its cells: strict side is only asserted in
// cell interiors and on the relative interiors of edges whose ZeroSide is -1.
type SignedGraphComponent struct {
	Cells             []int
	Cycles            []SignedPerimeterCycle
	InternalZeroSides []int
	// ContactCorners references Partition.ZeroCorners, including isolated
	// internal zero vertices that must survive consolidation.
	ContactCorners []int
}
type SplineGraphSignedComponents struct {
	Complete         bool
	Side             int
	Partition        SplineGraphContactPartition
	Components       []SignedGraphComponent
	Corners          [][2]*big.Rat
	AmbiguousCorners []int
	ExactOperations  int
}

// FindSplineGraphSignedComponents extracts exact cell-union perimeter cycles
// for Side +1 or -1. Boundary contact edges and exterior noncontact edges are
// distinguished without dropping internal zero loci. Maximal components use
// shared full sides; corner-only touching or pinched perimeter fans remain
// unresolved. Outer loops are UV CCW, holes CW; source cell ancestry is retained.
func FindSplineGraphSignedComponents(ctx context.Context, a, c *SurfacePatch, side int, o GraphPointContactOptions) (r SplineGraphSignedComponents, err error) {
	if side != -1 && side != 1 {
		return r, ErrInvalid
	}
	r.Side = side
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
	cornerIDs := map[string]int{}
	cornerID := func(point [2]float64) int {
		u, v := new(big.Rat).SetFloat64(point[0]), new(big.Rat).SetFloat64(point[1])
		key := u.RatString() + "," + v.RatString()
		if id, ok := cornerIDs[key]; ok {
			return id
		}
		id := len(r.Corners)
		cornerIDs[key] = id
		r.Corners = append(r.Corners, [2]*big.Rat{u, v})
		return id
	}
	ambiguous := map[int]bool{}
	// Local corner order LL,HL,HH,LH and CCW-oriented sides.
	endpoints := [4][2]int{{3, 0}, {1, 2}, {0, 1}, {2, 3}}
	for root, cell := range partition.Cells {
		if cell.Side != side || visited[root] {
			continue
		}
		if e := ctx.Err(); e != nil {
			return r, e
		}
		component := SignedGraphComponent{}
		queue := []int{root}
		visited[root] = true
		for len(queue) > 0 {
			index := queue[0]
			queue = queue[1:]
			component.Cells = append(component.Cells, index)
			for _, other := range neighbors[index] {
				if other >= 0 && partition.Cells[other].Side == side && !visited[other] {
					visited[other] = true
					queue = append(queue, other)
				}
			}
		}
		sort.Ints(component.Cells)
		contactCorners := map[int]bool{}
		for _, index := range component.Cells {
			for _, id := range partition.Cells[index].ZeroCorners {
				if id >= 0 {
					contactCorners[id] = true
				}
			}
		}
		for id := range contactCorners {
			component.ContactCorners = append(component.ContactCorners, id)
		}
		sort.Ints(component.ContactCorners)

		var edges []SignedPerimeterEdge
		for _, index := range component.Cells {
			item := partition.Cells[index]
			for edgeSide := SurfaceEdge(0); edgeSide < 4; edgeSide++ {
				other := neighbors[index][edgeSide]
				if other >= 0 && partition.Cells[other].Side == side {
					if index < other && item.ZeroSides[edgeSide] >= 0 {
						component.InternalZeroSides = append(component.InternalZeroSides, item.ZeroSides[edgeSide])
					}
					continue
				}
				ids := endpoints[edgeSide]
				d := item.Domain
				points := [4][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMax, d.VMax}, {d.UMin, d.VMax}}
				first, last := cornerID(points[ids[0]]), cornerID(points[ids[1]])
				edge := SignedPerimeterEdge{Cell: index, Edge: edgeSide, ZeroSide: item.ZeroSides[edgeSide], StartCorner: first, EndCorner: last, Exterior: other < 0}
				for axis := 0; axis < 2; axis++ {
					edge.Start[axis] = new(big.Rat).Set(r.Corners[first][axis])
					edge.End[axis] = new(big.Rat).Set(r.Corners[last][axis])
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
				cycle := SignedPerimeterCycle{SignedArea: new(big.Rat)}
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
