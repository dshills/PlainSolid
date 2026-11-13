package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"plainsolid/geom"
)

// exactHoledFootprintTriangulation consumes independently certified simple
// oriented rings. Visible bridges are exact, remain in material, and never
// cross earlier bridges. Positive ear triangles must reproduce the original
// oriented boundary chain and rational area; bridge duplication is temporary.
func exactHoledFootprintTriangulation(ctx context.Context, points [][2]*big.Rat, loops [][]int, limit int) ([][3]int, int, error) {
	p := partitionFillProof{ctx: ctx, points: points, limit: limit}
	if len(loops) == 0 {
		return nil, 0, geom.ErrInvalid
	}
	joined := append([]int(nil), loops[0]...)
	between := func(a, b, c int) bool {
		for axis := 0; axis < 2; axis++ {
			lo, hi := points[a][axis], points[b][axis]
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			if points[c][axis].Cmp(lo) < 0 || points[c][axis].Cmp(hi) > 0 {
				return false
			}
		}
		return true
	}
	crosses := func(a, b, c, d int) (bool, error) {
		x, e := p.orient(a, b, c)
		if e != nil {
			return false, e
		}
		y, e := p.orient(a, b, d)
		if e != nil {
			return false, e
		}
		u, e := p.orient(c, d, a)
		if e != nil {
			return false, e
		}
		v, e := p.orient(c, d, b)
		if e != nil {
			return false, e
		}
		return (x == 0 && between(a, b, c)) || (y == 0 && between(a, b, d)) || (u == 0 && between(c, d, a)) || (v == 0 && between(c, d, b)) || (x*y < 0 && u*v < 0), nil
	}
	var bridges [][2]int
	for hi := 1; hi < len(loops); hi++ {
		hole := loops[hi]
		found := false
		for oi, a := range joined {
			if found {
				break
			}
			for hj, b := range hole {
				// A repeated bridge endpoint has several polygon occurrences.
				// Use only the occurrence whose local material cone contains
				// the new bridge, preserving the cyclic boundary walk.
				prev, successor := joined[(oi+len(joined)-1)%len(joined)], joined[(oi+1)%len(joined)]
				turn, e := p.orient(prev, a, successor)
				if e != nil {
					return nil, p.work, e
				}
				left, e := p.orient(prev, a, b)
				if e != nil {
					return nil, p.work, e
				}
				right, e := p.orient(a, successor, b)
				if e != nil {
					return nil, p.work, e
				}
				if (turn >= 0 && (left <= 0 || right <= 0)) || (turn < 0 && left <= 0 && right <= 0) {
					continue
				}
				clear := true
				barriers := append([][]int(nil), loops...)
				for _, bridge := range bridges {
					barriers = append(barriers, []int{bridge[0], bridge[1]})
				}
				for _, ring := range barriers {
					for j, c := range ring {
						d := ring[(j+1)%len(ring)]
						if c == a || c == b || d == a || d == b {
							other := c
							if other == a || other == b {
								other = d
							}
							turn, e := p.orient(a, b, other)
							if e != nil {
								return nil, p.work, e
							}
							if turn == 0 {
								clear = false
							}
							continue
						}
						hit, e := crosses(a, b, c, d)
						if e != nil {
							return nil, p.work, e
						}
						if hit {
							clear = false
						}
					}
				}
				if !clear {
					continue
				}
				midpoint := [2]*big.Rat{}
				for axis := 0; axis < 2; axis++ {
					midpoint[axis] = new(big.Rat).Quo(new(big.Rat).Add(points[a][axis], points[b][axis]), big.NewRat(2, 1))
				}
				p.points = append(points, midpoint)
				mid := len(points)
				inside := func(ring []int) (bool, error) {
					w := 0
					for j, c := range ring {
						d := ring[(j+1)%len(ring)]
						turn, e := p.orient(c, d, mid)
						if e != nil {
							return false, e
						}
						if points[c][1].Cmp(midpoint[1]) <= 0 && points[d][1].Cmp(midpoint[1]) > 0 && turn > 0 {
							w++
						}
						if points[c][1].Cmp(midpoint[1]) > 0 && points[d][1].Cmp(midpoint[1]) <= 0 && turn < 0 {
							w--
						}
					}
					return w != 0, nil
				}
				material, e := inside(loops[0])
				if e != nil {
					return nil, p.work, e
				}
				for _, h := range loops[1:] {
					in, e := inside(h)
					if e != nil {
						return nil, p.work, e
					}
					if in {
						material = false
					}
				}
				p.points = points
				if !material {
					continue
				}
				next := append([]int(nil), joined[:oi+1]...)
				next = append(next, hole[hj:]...)
				next = append(next, hole[:hj+1]...)
				next = append(next, a)
				next = append(next, joined[oi+1:]...)
				joined = next
				bridges = append(bridges, [2]int{a, b})
				found = true
				break
			}
		}
		if !found {
			return nil, p.work, fmt.Errorf("hole %d bridge: %w", hi, geom.ErrConvergence)
		}
	}
	tris, e := p.triangulate(joined)
	if e != nil {
		return nil, p.work, fmt.Errorf("bridged polygon triangulation: %w", e)
	}
	chain := map[[2]int]int{}
	area, sum := new(big.Rat), new(big.Rat)
	for _, loop := range loops {
		area.Add(area, p.area(loop))
		for i, a := range loop {
			partitionChainAdd(chain, a, loop[(i+1)%len(loop)], -1)
		}
	}
	for _, tri := range tris {
		sum.Add(sum, p.area(tri[:]))
		for i, a := range tri {
			partitionChainAdd(chain, a, tri[(i+1)%3], 1)
		}
	}
	if len(chain) != 0 || sum.Cmp(area) != 0 || area.Sign() <= 0 {
		return nil, p.work, geom.ErrConvergence
	}
	return tris, p.work, nil
}
