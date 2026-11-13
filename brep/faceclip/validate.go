package faceclip

import (
	"context"
	"math/big"
)

// ValidateBoundary independently certifies a rational open planar region:
// coplanarity, simple disjoint loops, nonoverlapping adjacent segments, correct
// hole parent/nesting and winding, and exact projected area. Crossings without
// shared vertices and all inter-loop touching are rejected. Consecutive forward
// collinear edges are allowed. MaxWork counts pair and containment predicates;
// all segment pairs are examined, so the worst case is quadratic.
func ValidateBoundary(ctx context.Context, b Boundary, o LoopOptions) error {
	if ctx == nil || b.Axis < 0 || b.Axis > 2 || b.ProjectedArea == nil || b.ProjectedArea.Sign() < 0 || o.MaxVertices < 0 || o.MaxWork < 0 || o.MaxBits < 0 {
		return ErrInvalid
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 8192
	}
	if o.MaxWork == 0 {
		o.MaxWork = 2000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	work, count := 0, 0
	tick := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		work++
		if work > o.MaxWork {
			return ErrLimit
		}
		return nil
	}
	check := func(x *big.Rat) bool {
		return x != nil && x.Num().BitLen() <= o.MaxBits && x.Denom().BitLen() <= o.MaxBits
	}
	if e := tick(); e != nil {
		return e
	}
	if !check(b.ProjectedArea) {
		return ErrLimit
	}
	if len(b.Loops) == 0 {
		if b.ProjectedArea.Sign() != 0 {
			return ErrInvalid
		}
		return nil
	}
	for _, l := range b.Loops {
		if len(l.Vertices) < 3 {
			return ErrInvalid
		}
		count += len(l.Vertices)
		if count > o.MaxVertices {
			return ErrLimit
		}
		for _, p := range l.Vertices {
			for _, x := range p {
				if x == nil {
					return ErrInvalid
				}
				if !check(x) {
					return ErrLimit
				}
			}
		}
	}
	origin := b.Loops[0].Vertices[0]
	var normal Point
	found := false
	for i := 1; i+1 < len(b.Loops[0].Vertices); i++ {
		normal = cross(diff(b.Loops[0].Vertices[i], origin), diff(b.Loops[0].Vertices[i+1], origin))
		if normal[b.Axis].Sign() != 0 {
			found = true
			break
		}
	}
	if !found {
		return ErrInvalid
	}
	planeD := dot(normal, origin)
	type edge struct {
		a, c        Point
		loop, index int
	}
	var edges []edge
	u, v := (b.Axis+1)%3, (b.Axis+2)%3
	orient := func(a, c, p Point) *big.Rat {
		return sub(mul(sub(c[u], a[u]), sub(p[v], a[v])), mul(sub(c[v], a[v]), sub(p[u], a[u])))
	}
	on := func(a, c, p Point) bool {
		return orient(a, c, p).Sign() == 0 && p[u].Cmp(minRat(a[u], c[u])) >= 0 && p[u].Cmp(maxRat(a[u], c[u])) <= 0 && p[v].Cmp(minRat(a[v], c[v])) >= 0 && p[v].Cmp(maxRat(a[v], c[v])) <= 0
	}
	areas := make([]*big.Rat, len(b.Loops))
	total := new(big.Rat)
	for i, l := range b.Loops {
		area := new(big.Rat)
		for j, a := range l.Vertices {
			if e := tick(); e != nil {
				return e
			}
			c := l.Vertices[(j+1)%len(l.Vertices)]
			if same(a, c) || dot(normal, a).Cmp(planeD) != 0 {
				return ErrInvalid
			}
			area.Add(area, sub(mul(a[u], c[v]), mul(a[v], c[u])))
			if !check(area) {
				return ErrLimit
			}
			edges = append(edges, edge{a, c, i, j})
		}
		area.Quo(area, big.NewRat(2, 1))
		if area.Sign() == 0 {
			return ErrNonmanifold
		}
		areas[i] = area
		total.Add(total, area)
		if !check(total) {
			return ErrLimit
		}
	}
	for i, a := range edges {
		for _, c := range edges[:i] {
			if e := tick(); e != nil {
				return e
			}
			adjacent := a.loop == c.loop && (a.index == (c.index+1)%len(b.Loops[a.loop].Vertices) || c.index == (a.index+1)%len(b.Loops[a.loop].Vertices))
			if adjacent {
				var shared, pa, pc Point
				if same(a.a, c.c) {
					shared, pa, pc = a.a, a.c, c.a
				} else if same(a.c, c.a) {
					shared, pa, pc = a.c, a.a, c.c
				} else {
					return ErrNonmanifold
				}
				if on(shared, pa, pc) || on(shared, pc, pa) {
					return ErrNonmanifold
				}
				continue
			}
			oa, ob, oc, od := orient(a.a, a.c, c.a), orient(a.a, a.c, c.c), orient(c.a, c.c, a.a), orient(c.a, c.c, a.c)
			if oa.Sign()*ob.Sign() < 0 && oc.Sign()*od.Sign() < 0 || on(a.a, a.c, c.a) || on(a.a, a.c, c.c) || on(c.a, c.c, a.a) || on(c.a, c.c, a.c) {
				return ErrNonmanifold
			}
		}
	}
	contains := func(l Loop, p Point) (bool, error) {
		inside := false
		for i, a := range l.Vertices {
			if e := tick(); e != nil {
				return false, e
			}
			c := l.Vertices[(i+1)%len(l.Vertices)]
			if (a[v].Cmp(p[v]) > 0) != (c[v].Cmp(p[v]) > 0) {
				x := add(a[u], div(mul(sub(p[v], a[v]), sub(c[u], a[u])), sub(c[v], a[v])))
				if !check(x) {
					return false, ErrLimit
				}
				if x.Cmp(p[u]) > 0 {
					inside = !inside
				}
			}
		}
		return inside, nil
	}
	outerSign := 0
	for i, l := range b.Loops {
		depth, parent := 0, -1
		for j, other := range b.Loops {
			if i == j {
				continue
			}
			inside, e := contains(other, l.Vertices[0])
			if e != nil {
				return e
			}
			if inside {
				depth++
				if parent < 0 || new(big.Rat).Abs(areas[j]).Cmp(new(big.Rat).Abs(areas[parent])) < 0 {
					parent = j
				}
			}
		}
		hole := depth%2 == 1
		if l.Hole != hole {
			return ErrInvalid
		}
		if hole {
			if l.Parent != parent || parent < 0 || b.Loops[parent].Hole {
				return ErrInvalid
			}
		} else if l.Parent != -1 {
			return ErrInvalid
		}
		sign := areas[i].Sign()
		if hole {
			sign = -sign
		}
		if outerSign == 0 {
			outerSign = sign
		} else if outerSign != sign {
			return ErrInvalid
		}
	}
	if new(big.Rat).Abs(total).Cmp(b.ProjectedArea) != 0 {
		return ErrInvalid
	}
	return nil
}
