package faceclip

import (
	"context"
	"errors"
	"math/big"
	"sort"
)

var ErrNonmanifold = errors.New("faceclip: clipped boundary has overlapping or branching contacts")

// Loop is an exact oriented boundary. Hole distinguishes inner loops; Parent is
// its containing outer loop index, or -1 for an outer loop. Winding follows the
// input face. Islands are separate outer loops, not artificially connected.
type Loop struct {
	Vertices []Point
	Hole     bool
	Parent   int
}
type LoopOptions struct{ MaxVertices, MaxWork, MaxBits int }

// Boundary holds exact merged open face-region loops; it is not a solid.
// Fragment ancestry remains available in Clip, rather than being collapsed to
// one arbitrary owner per reconstructed loop.
type Boundary struct {
	Loops         []Loop
	Axis          int
	ProjectedArea *big.Rat
}

func pointKey(p Point) string {
	return p[0].RatString() + "," + p[1].RatString() + "," + p[2].RatString()
}

// MergeLoops aligns fragment edges at every exact shared vertex, cancels
// opposing segments, and reconstructs oriented loops. MaxVertices defaults to
// 8192, MaxWork to 2000000 and MaxBits to 16384. Fragment edges are compared with
// all vertices; bounds are explicit. Input must be the unmodified result of Clip.
// Collinear boundary vertices are retained to preserve exact segmentation.
func MergeLoops(ctx context.Context, r Result, o LoopOptions) (Boundary, error) {
	empty := Boundary{}
	if ctx == nil || r.Axis < 0 || r.Axis > 2 || r.ProjectedArea == nil || r.ProjectedArea.Sign() < 0 || o.MaxVertices < 0 || o.MaxWork < 0 || o.MaxBits < 0 {
		return empty, ErrInvalid
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
	check := func(x *big.Rat) bool { return x.Num().BitLen() <= o.MaxBits && x.Denom().BitLen() <= o.MaxBits }
	work := 0
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
	var points []Point
	lookup := map[string]int{}
	var rings [][]int
	for _, f := range r.Fragments {
		if len(f.Vertices) < 3 {
			return empty, ErrInvalid
		}
		var ids []int
		for _, p := range f.Vertices {
			if e := tick(); e != nil {
				return empty, e
			}
			for _, x := range p {
				if x == nil {
					return empty, ErrInvalid
				}
				if x.Num().BitLen() > o.MaxBits || x.Denom().BitLen() > o.MaxBits {
					return empty, ErrLimit
				}
			}
			k := pointKey(p)
			id, ok := lookup[k]
			if !ok {
				id = len(points)
				if id >= o.MaxVertices {
					return empty, ErrLimit
				}
				lookup[k] = id
				points = append(points, clone(p))
			}
			ids = append(ids, id)
		}
		rings = append(rings, ids)
	}
	if len(points) == 0 {
		if r.ProjectedArea.Sign() != 0 {
			return empty, ErrInvalid
		}
		return Boundary{Axis: r.Axis, ProjectedArea: new(big.Rat)}, nil
	}
	counts := map[[2]int]int{}
	for _, ring := range rings {
		for i, a := range ring {
			b := ring[(i+1)%len(ring)]
			if a == b {
				return empty, ErrInvalid
			}
			delta := diff(points[b], points[a])
			axis := 0
			for delta[axis].Sign() == 0 {
				axis++
			}
			type cut struct {
				t  *big.Rat
				id int
			}
			cuts := []cut{{new(big.Rat), a}, {big.NewRat(1, 1), b}}
			for j, p := range points {
				if e := tick(); e != nil {
					return empty, e
				}
				if j == a || j == b {
					continue
				}
				d := diff(p, points[a])
				c := cross(delta, d)
				if dot(c, c).Sign() != 0 {
					continue
				}
				t := div(d[axis], delta[axis])
				if !check(t) {
					return empty, ErrLimit
				}
				if t.Sign() > 0 && t.Cmp(big.NewRat(1, 1)) < 0 {
					cuts = append(cuts, cut{t, j})
				}
			}
			sort.Slice(cuts, func(i, j int) bool { return cuts[i].t.Cmp(cuts[j].t) < 0 })
			for j := 0; j+1 < len(cuts); j++ {
				key := [2]int{cuts[j].id, cuts[j+1].id}
				counts[key]++
				if counts[key] > 1 {
					return empty, ErrNonmanifold
				}
			}
		}
	}
	next := map[int]int{}
	incoming := map[int]int{}
	for edge := range counts {
		if counts[[2]int{edge[1], edge[0]}] != 0 {
			continue
		}
		if _, ok := next[edge[0]]; ok {
			return empty, ErrNonmanifold
		}
		if _, ok := incoming[edge[1]]; ok {
			return empty, ErrNonmanifold
		}
		next[edge[0]] = edge[1]
		incoming[edge[1]] = edge[0]
	}
	for a := range next {
		if _, ok := incoming[a]; !ok {
			return empty, ErrNonmanifold
		}
	}
	var loops []Loop
	used := map[int]bool{}
	for start := 0; start < len(points); start++ {
		if _, ok := next[start]; !ok || used[start] {
			continue
		}
		loop := Loop{Parent: -1}
		cur := start
		for {
			if e := tick(); e != nil {
				return empty, e
			}
			if used[cur] {
				if cur != start {
					return empty, ErrNonmanifold
				}
				break
			}
			used[cur] = true
			loop.Vertices = append(loop.Vertices, clone(points[cur]))
			var ok bool
			cur, ok = next[cur]
			if !ok {
				return empty, ErrNonmanifold
			}
		}
		if len(loop.Vertices) < 3 {
			return empty, ErrNonmanifold
		}
		loops = append(loops, loop)
	}
	u, v := (r.Axis+1)%3, (r.Axis+2)%3
	areas := make([]*big.Rat, len(loops))
	sum := new(big.Rat)
	largest := -1
	for i, l := range loops {
		a := new(big.Rat)
		for j, p := range l.Vertices {
			q := l.Vertices[(j+1)%len(l.Vertices)]
			a.Add(a, sub(mul(p[u], q[v]), mul(p[v], q[u])))
		}
		a.Quo(a, big.NewRat(2, 1))
		if a.Sign() == 0 {
			return empty, ErrNonmanifold
		}
		if !check(a) {
			return empty, ErrLimit
		}
		areas[i] = a
		sum.Add(sum, a)
		if !check(sum) {
			return empty, ErrLimit
		}
		if largest < 0 || new(big.Rat).Abs(a).Cmp(new(big.Rat).Abs(areas[largest])) > 0 {
			largest = i
		}
	}
	if new(big.Rat).Abs(sum).Cmp(r.ProjectedArea) != 0 {
		return empty, ErrInvalid
	}
	// Determine nesting through an exact half-open ray crossing. Loop vertices
	// cannot touch another boundary in a manifold result; return explicit error.
	contains := func(poly []Point, p Point) (bool, error) {
		inside := false
		for i, a := range poly {
			if e := tick(); e != nil {
				return false, e
			}
			b := poly[(i+1)%len(poly)]
			orient := sub(mul(sub(b[u], a[u]), sub(p[v], a[v])), mul(sub(b[v], a[v]), sub(p[u], a[u])))
			if orient.Sign() == 0 && p[u].Cmp(minRat(a[u], b[u])) >= 0 && p[u].Cmp(maxRat(a[u], b[u])) <= 0 && p[v].Cmp(minRat(a[v], b[v])) >= 0 && p[v].Cmp(maxRat(a[v], b[v])) <= 0 {
				return false, ErrNonmanifold
			}
			if (a[v].Cmp(p[v]) > 0) != (b[v].Cmp(p[v]) > 0) {
				x := add(a[u], div(mul(sub(p[v], a[v]), sub(b[u], a[u])), sub(b[v], a[v])))
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
	for i := range loops {
		depth := 0
		parent := -1
		for j := range loops {
			if i == j {
				continue
			}
			inside, e := contains(loops[j].Vertices, loops[i].Vertices[0])
			if e != nil {
				return empty, e
			}
			if inside {
				depth++
				if parent < 0 || new(big.Rat).Abs(areas[j]).Cmp(new(big.Rat).Abs(areas[parent])) < 0 {
					parent = j
				}
			}
		}
		loops[i].Hole = depth%2 == 1
		if loops[i].Hole {
			loops[i].Parent = parent
		}
		expected := areas[largest].Sign()
		if loops[i].Hole {
			expected = -expected
		}
		if areas[i].Sign() != expected {
			return empty, ErrNonmanifold
		}
	}
	return Boundary{loops, r.Axis, new(big.Rat).Set(r.ProjectedArea)}, nil
}
func minRat(a, b *big.Rat) *big.Rat {
	if a.Cmp(b) < 0 {
		return a
	}
	return b
}
func maxRat(a, b *big.Rat) *big.Rat {
	if a.Cmp(b) > 0 {
		return a
	}
	return b
}
