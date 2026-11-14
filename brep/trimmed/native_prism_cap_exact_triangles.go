package trimmed

import (
	"math/big"
	"plainsolid/geom"
)

type nativePrismCapTriangle struct {
	ids   [3]int
	cycle int
}

// nativePrismHoledCapTriangles constructs a maximal noncrossing constrained
// graph using exact rational predicates. No boundary vertex or edge is removed.
// Candidate triangles must contain no other vertex, lie in the signed region,
// reproduce every directed boundary edge and pair every internal edge. Their
// exact total area must equal the signed ring area. The 96-vertex cap supplements
// the cumulative arithmetic budget; this is deliberately not a generic mesher.
func nativePrismHoledCapTriangles(rings [][]int, points [][3]*big.Rat, xy [2]int, b *nativePrismAffineBudget) ([]nativePrismCapTriangle, error) {
	return nativePrismCapTrianglesMode(rings, points, xy, b, false)
}

// Nested mode is for independently certified disjoint arrangement cycles.
func nativePrismCapTrianglesMode(rings [][]int, points [][3]*big.Rat, xy [2]int, b *nativePrismAffineBudget, nested bool) ([]nativePrismCapTriangle, error) {
	type pt [2]*big.Rat
	p := []pt{}
	global := []int{}
	local := map[int]int{}
	loops := [][]int{}
	for _, ring := range rings {
		if len(ring) < 3 {
			return nil, geom.ErrInvalid
		}
		loop := []int{}
		for _, id := range ring {
			if _, ok := local[id]; ok {
				return nil, geom.ErrDomain
			}
			local[id] = len(p)
			loop = append(loop, len(p))
			global = append(global, id)
			p = append(p, pt{points[id][xy[0]], points[id][xy[1]]})
		}
		loops = append(loops, loop)
	}
	if len(p) > 96 {
		return nil, geom.ErrConvergence
	}
	cross := func(a, c, d pt) *big.Rat {
		return b.op('-', b.op('*', b.op('-', c[0], a[0]), b.op('-', d[1], a[1])), b.op('*', b.op('-', c[1], a[1]), b.op('-', d[0], a[0])))
	}
	between := func(x, a, c *big.Rat) bool {
		if a.Cmp(c) > 0 {
			a, c = c, a
		}
		return x.Cmp(a) >= 0 && x.Cmp(c) <= 0
	}
	on := func(q, a, c pt) bool {
		return cross(a, c, q).Sign() == 0 && between(q[0], a[0], c[0]) && between(q[1], a[1], c[1])
	}
	location := func(q pt, loop []int) int {
		w := 0
		for i, a := range loop {
			c := loop[(i+1)%len(loop)]
			if on(q, p[a], p[c]) {
				return 2
			}
			sign := cross(p[a], p[c], q).Sign()
			if p[a][1].Cmp(q[1]) <= 0 {
				if p[c][1].Cmp(q[1]) > 0 && sign > 0 {
					w++
				}
			} else if p[c][1].Cmp(q[1]) <= 0 && sign < 0 {
				w--
			}
		}
		return w
	}
	inside := func(q pt) bool {
		w := 0
		for _, loop := range loops {
			k := location(q, loop)
			if k == 2 {
				return false
			}
			w += k
		}
		return w == 1
	}
	type edge [2]int
	key := func(a, c int) edge {
		if a > c {
			a, c = c, a
		}
		return edge{a, c}
	}
	graph := map[edge]bool{}
	boundary := map[edge]int{}
	ordered := []edge{}
	want := new(big.Rat)
	areas := []*big.Rat{}
	for _, loop := range loops {
		area := new(big.Rat)
		for i, a := range loop {
			c := loop[(i+1)%len(loop)]
			e := key(a, c)
			if graph[e] {
				return nil, geom.ErrDomain
			}
			graph[e] = true
			ordered = append(ordered, e)
			sign := 1
			if a > c {
				sign = -1
			}
			boundary[e] = sign
			area = b.op('+', area, b.op('-', b.op('*', p[a][0], p[c][1]), b.op('*', p[a][1], p[c][0])))
		}
		areas = append(areas, area)
		want = b.op('+', want, area)
	}
	if want.Sign() <= 0 {
		return nil, geom.ErrDomain
	}
	intersects := func(a, c, d, e pt) bool {
		s1, s2, s3, s4 := cross(a, c, d).Sign(), cross(a, c, e).Sign(), cross(d, e, a).Sign(), cross(d, e, c).Sign()
		return s1*s2 < 0 && s3*s4 < 0 || s1 == 0 && on(d, a, c) || s2 == 0 && on(e, a, c) || s3 == 0 && on(a, d, e) || s4 == 0 && on(c, d, e)
	}
	for a := range p {
		for c := a + 1; c < len(p); c++ {
			e := edge{a, c}
			if graph[e] {
				continue
			}
			clear := true
			for d := range p {
				if d != a && d != c && on(p[d], p[a], p[c]) {
					clear = false
					break
				}
			}
			if !clear {
				continue
			}
			for _, f := range ordered {
				if f[0] == a || f[1] == a || f[0] == c || f[1] == c {
					continue
				}
				if intersects(p[a], p[c], p[f[0]], p[f[1]]) {
					clear = false
					break
				}
			}
			if b.err != nil {
				return nil, b.err
			}
			if !clear {
				continue
			}
			mid := pt{b.op('/', b.op('+', p[a][0], p[c][0]), big.NewRat(2, 1)), b.op('/', b.op('+', p[a][1], p[c][1]), big.NewRat(2, 1))}
			if !inside(mid) {
				continue
			}
			graph[e] = true
			ordered = append(ordered, e)
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	count := map[edge]int{}
	orientation := map[edge]int{}
	sum := new(big.Rat)
	out := []nativePrismCapTriangle{}
	for a := range p {
		for c := a + 1; c < len(p); c++ {
			if !graph[edge{a, c}] {
				continue
			}
			for d := c + 1; d < len(p); d++ {
				if !graph[edge{a, d}] || !graph[edge{c, d}] {
					continue
				}
				ids := [3]int{a, c, d}
				area := cross(p[a], p[c], p[d])
				if area.Sign() == 0 {
					continue
				}
				if area.Sign() < 0 {
					ids[1], ids[2] = ids[2], ids[1]
					area = b.op('-', new(big.Rat), area)
				}
				clear := true
				for q := range p {
					if q == a || q == c || q == d {
						continue
					}
					if cross(p[ids[0]], p[ids[1]], p[q]).Sign() >= 0 && cross(p[ids[1]], p[ids[2]], p[q]).Sign() >= 0 && cross(p[ids[2]], p[ids[0]], p[q]).Sign() >= 0 {
						clear = false
						break
					}
				}
				if b.err != nil {
					return nil, b.err
				}
				if !clear {
					continue
				}
				center := pt{}
				for k := 0; k < 2; k++ {
					center[k] = b.op('/', b.op('+', b.op('+', p[a][k], p[c][k]), p[d][k]), big.NewRat(3, 1))
				}
				if !inside(center) {
					continue
				}
				owner := -1
				for i, loop := range loops {
					if areas[i].Sign() > 0 && location(center, loop) == 1 {
						if owner >= 0 {
							if !nested || areas[i].Cmp(areas[owner]) == 0 {
								return nil, geom.ErrInvalid
							}
							if areas[i].Cmp(areas[owner]) > 0 {
								continue
							}
						}
						owner = i
					}
				}
				if owner < 0 {
					return nil, geom.ErrInvalid
				}
				out = append(out, nativePrismCapTriangle{[3]int{global[ids[0]], global[ids[1]], global[ids[2]]}, owner})
				sum = b.op('+', sum, area)
				for i, u := range ids {
					v := ids[(i+1)%3]
					e := key(u, v)
					count[e]++
					if u < v {
						orientation[e]++
					} else {
						orientation[e]--
					}
				}
			}
		}
	}
	if b.err != nil {
		return nil, b.err
	}
	if sum.Cmp(want) != 0 {
		return nil, geom.ErrInvalid
	}
	for e, sign := range boundary {
		if count[e] != 1 || orientation[e] != sign {
			return nil, geom.ErrInvalid
		}
	}
	for e, n := range count {
		if _, ok := boundary[e]; !ok && (n != 2 || orientation[e] != 0) {
			return nil, geom.ErrInvalid
		}
	}
	return out, nil
}
