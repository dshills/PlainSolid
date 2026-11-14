package brep

import (
	"math"
	"plainsolid/geom"
	"sort"
)

// planarEdgeGrid indexes conservative endpoint AABBs. Cell assignment is
// monotone in each coordinate; any intersecting boxes share at least one cell.
// Exact intersection and collinearity predicates remain the final decision.
type planarEdgeGrid struct {
	lo, hi     geom.Vec2
	cells      [256][]int
	stamps     []uint32
	generation uint32
	found      []int
}

func newPlanarEdgeGrid(points []geom.Vec2) *planarEdgeGrid {
	g := &planarEdgeGrid{lo: points[0], hi: points[0]}
	for _, p := range points {
		g.lo.X = min(g.lo.X, p.X)
		g.lo.Y = min(g.lo.Y, p.Y)
		g.hi.X = max(g.hi.X, p.X)
		g.hi.Y = max(g.hi.Y, p.Y)
	}
	return g
}
func (g *planarEdgeGrid) cellsFor(a, b geom.Vec2) (int, int, int, int) {
	cell := func(v, lo, hi float64) int {
		if hi == lo {
			return 0
		}
		return max(0, min(15, int(math.Floor(16*((v-lo)/(hi-lo))))))
	}
	return cell(min(a.X, b.X), g.lo.X, g.hi.X), cell(max(a.X, b.X), g.lo.X, g.hi.X), cell(min(a.Y, b.Y), g.lo.Y, g.hi.Y), cell(max(a.Y, b.Y), g.lo.Y, g.hi.Y)
}
func (g *planarEdgeGrid) add(a, b geom.Vec2, id int) int {
	x0, x1, y0, y1 := g.cellsFor(a, b)
	for len(g.stamps) <= id {
		g.stamps = append(g.stamps, 0)
	}
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			i := y*16 + x
			g.cells[i] = append(g.cells[i], id)
		}
	}
	return (x1 - x0 + 1) * (y1 - y0 + 1)
}
func (g *planarEdgeGrid) query(a, b geom.Vec2) ([]int, int) {
	g.generation++
	if g.generation == 0 {
		clear(g.stamps)
		g.generation = 1
	}
	g.found = g.found[:0]
	x0, x1, y0, y1 := g.cellsFor(a, b)
	work := 0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			ids := g.cells[y*16+x]
			work += 1 + len(ids)
			for _, id := range ids {
				if g.stamps[id] != g.generation {
					g.stamps[id] = g.generation
					g.found = append(g.found, id)
				}
			}
		}
	}
	sort.Ints(g.found)
	return g.found, work
}
