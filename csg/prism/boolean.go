// Package prism performs planar profile Booleans on prismatic solids of a
// common height. Its arrangement uses exact rational arithmetic on represented
// float64 inputs; only output coordinates are rounded.
package prism

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"

	"plainsolid/brep"
	"plainsolid/geom"
)

// Profile contains an outer polygon and strictly contained disjoint holes.
// Both input windings are accepted. Points are in the XY plane.
type Profile struct {
	Outer []geom.Vec2
	Holes [][]geom.Vec2
}
type Operation uint8

const (
	Union Operation = iota
	Intersection
	Difference
)

type Options struct {
	// MaxSegments bounds total input sides (default 256, maximum 2048).
	MaxSegments int
	// MaxWork bounds pair intersection and fragment classification work.
	// Zero selects two million operations.
	MaxWork int
}

var ErrLimit = errors.New("prism: arrangement work limit exceeded")
var ErrNonmanifold = errors.New("prism: nonmanifold result at a point contact")

type q = *big.Rat
type pt struct{ x, y q }

func number(v float64) q  { return new(big.Rat).SetFloat64(v) }
func add(a, b q) q        { return new(big.Rat).Add(a, b) }
func sub(a, b q) q        { return new(big.Rat).Sub(a, b) }
func mul(a, b q) q        { return new(big.Rat).Mul(a, b) }
func div(a, b q) q        { return new(big.Rat).Quo(a, b) }
func plus(a, b pt) pt     { return pt{add(a.x, b.x), add(a.y, b.y)} }
func minus(a, b pt) pt    { return pt{sub(a.x, b.x), sub(a.y, b.y)} }
func scale(a pt, b q) pt  { return pt{mul(a.x, b), mul(a.y, b)} }
func cross(a, b pt) q     { return sub(mul(a.x, b.y), mul(a.y, b.x)) }
func turn(a, b, c pt) int { return cross(minus(b, a), minus(c, a)).Sign() }
func less(a, b pt) bool {
	if x := a.x.Cmp(b.x); x != 0 {
		return x < 0
	}
	return a.y.Cmp(b.y) < 0
}
func key(p pt) string { return p.x.RatString() + "," + p.y.RatString() }
func area(r []pt) q {
	v := new(big.Rat)
	for i, p := range r {
		v.Add(v, cross(p, r[(i+1)%len(r)]))
	}
	return v
}
func contains(r []pt, p pt) bool {
	w := 0
	for i, a := range r {
		b := r[(i+1)%len(r)]
		x, y := a.y.Cmp(p.y), b.y.Cmp(p.y)
		if x <= 0 && y > 0 && turn(a, b, p) > 0 {
			w++
		}
		if x > 0 && y <= 0 && turn(a, b, p) < 0 {
			w--
		}
	}
	return w != 0
}
func on(a, b, p pt) bool {
	in := func(a, b, x q) bool { return x.Cmp(a) >= 0 && x.Cmp(b) <= 0 || x.Cmp(b) >= 0 && x.Cmp(a) <= 0 }
	return turn(a, b, p) == 0 && in(a.x, b.x, p.x) && in(a.y, b.y, p.y)
}

type side struct {
	a, b pt
	op   int
	cuts []q
}
type fragment struct {
	a, b   pt
	orient [2]int
}
type connection struct{ a, b pt }

// Boolean returns closed planar B-reps for a regularized Boolean on two
// profiles extruded from z=0 to height. Each connected output profile produces
// a solid with planar caps. Point contacts between components are rejected.
// Concave profiles and holes are supported. Unresolvable float64 output and
// resource exhaustion are explicit errors.
func Boolean(ctx context.Context, a, b Profile, height float64, op Operation, o Options) ([]brep.Solid, error) {
	if ctx == nil || !(height > 0) || math.IsInf(height, 0) || op > Difference || o.MaxSegments < 0 || o.MaxSegments > 2048 || o.MaxWork < 0 {
		return nil, fmt.Errorf("prism: invalid operation options")
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 256
	}
	if o.MaxWork == 0 {
		o.MaxWork = 2000000
	}
	var operands [2][][]pt
	var sides []side
	for i, p := range [2]Profile{a, b} {
		for j, r := range append([][]geom.Vec2{p.Outer}, p.Holes...) {
			if len(r) < 3 {
				return nil, fmt.Errorf("prism: invalid ring")
			}
			if len(r) > o.MaxSegments-len(sides) {
				return nil, ErrLimit
			}
			s := make([]pt, len(r))
			for k, v := range r {
				if !finite(v.X) || !finite(v.Y) {
					return nil, fmt.Errorf("prism: nonfinite vertex")
				}
				s[k] = pt{number(v.X), number(v.Y)}
			}
			w := area(s).Sign()
			if w == 0 {
				return nil, fmt.Errorf("prism: zero area ring")
			}
			if j == 0 && w < 0 || j > 0 && w > 0 {
				reverse(s)
			}
			for k, v := range s {
				n := s[(k+1)%len(s)]
				if key(v) == key(n) {
					return nil, fmt.Errorf("prism: zero-length edge")
				}
				sides = append(sides, side{v, n, i, []q{number(0), number(1)}})
			}
			operands[i] = append(operands[i], s)
		}
		// Validate embedded simple rings, hole nesting and coplanarity before
		// relying on their directed left/right interior convention.
		if _, e := extrude(operands[i], height); e != nil {
			return nil, fmt.Errorf("prism: operand %d: %w", i, e)
		}
	}
	work := o.MaxWork
	for i := range sides {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		for j := 0; j < i; j++ {
			if work == 0 {
				return nil, ErrLimit
			}
			work--
			cut(&sides[i], &sides[j])
		}
	}
	byKey := map[[2]string]*fragment{}
	var fragments []*fragment
	for _, s := range sides {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		sort.Slice(s.cuts, func(i, j int) bool { return s.cuts[i].Cmp(s.cuts[j]) < 0 })
		p := s.a
		d := minus(s.b, s.a)
		for j := 1; j < len(s.cuts); j++ {
			if s.cuts[j].Cmp(s.cuts[j-1]) == 0 {
				continue
			}
			if work == 0 {
				return nil, ErrLimit
			}
			work--
			r := plus(s.a, scale(d, s.cuts[j]))
			u, v := p, r
			orient := 1
			if less(v, u) {
				u, v = v, u
				orient = -1
			}
			k := [2]string{key(u), key(v)}
			f := byKey[k]
			if f == nil {
				f = &fragment{a: u, b: v}
				byKey[k] = f
				fragments = append(fragments, f)
			}
			if f.orient[s.op] != 0 {
				return nil, fmt.Errorf("prism: overlapping edges in one operand")
			}
			f.orient[s.op] = orient
			p = r
		}
	}
	keep := func(a, b bool) bool {
		switch op {
		case Union:
			return a || b
		case Intersection:
			return a && b
		default:
			return a && !b
		}
	}
	graph := map[string]connection{}
	in := map[string]bool{}
	for _, f := range fragments {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		var left, right [2]bool
		for i, d := range f.orient {
			switch d {
			case 1:
				left[i] = true
			case -1:
				right[i] = true
			default:
				p := scale(plus(f.a, f.b), number(.5))
				v := false
				for _, r := range operands[i] {
					if len(r) > work {
						return nil, ErrLimit
					}
					work -= len(r)
					if contains(r, p) {
						v = !v
					}
				}
				left[i], right[i] = v, v
			}
		}
		l, r := keep(left[0], left[1]), keep(right[0], right[1])
		if l == r {
			continue
		}
		a, b := f.a, f.b
		if !l {
			a, b = b, a
		}
		ak, bk := key(a), key(b)
		if _, ok := graph[ak]; ok || in[bk] {
			return nil, ErrNonmanifold
		}
		graph[ak], in[bk] = connection{a, b}, true
	}
	var out, holes [][]pt
	starts := make([]string, 0, len(graph))
	for k := range graph {
		starts = append(starts, k)
	}
	sort.Strings(starts)
	for _, start := range starts {
		if _, ok := graph[start]; !ok {
			continue
		}
		var ring []pt
		for k := start; ; {
			f, ok := graph[k]
			if !ok {
				return nil, fmt.Errorf("prism: open arrangement boundary")
			}
			delete(graph, k)
			ring = append(ring, f.a)
			k = key(f.b)
			if k == start {
				break
			}
		}
		ring = removeCollinear(ring)
		if len(ring) < 3 {
			return nil, fmt.Errorf("prism: degenerate output ring")
		}
		switch area(ring).Sign() {
		case 1:
			out = append(out, ring)
		case -1:
			holes = append(holes, ring)
		default:
			return nil, fmt.Errorf("prism: degenerate output ring")
		}
	}
	group := make([][][]pt, len(out))
	for i, r := range out {
		group[i] = [][]pt{r}
	}
	for _, h := range holes {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		id := -1
		for i, o := range out {
			if len(o) > work {
				return nil, ErrLimit
			}
			work -= len(o)
			if contains(o, h[0]) && (id < 0 || area(o).Cmp(area(out[id])) < 0) {
				id = i
			}
		}
		if id < 0 {
			return nil, fmt.Errorf("prism: uncontained output hole")
		}
		group[id] = append(group[id], h)
	}
	result := make([]brep.Solid, 0, len(group))
	for _, rings := range group {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		s, e := extrude(rings, height)
		if e != nil {
			return nil, fmt.Errorf("prism: output topology: %w", e)
		}
		result = append(result, s)
	}
	return result, nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func reverse(r []pt) {
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
}
func removeCollinear(r []pt) []pt {
	if len(r) < 3 {
		return r
	}
	s := make([]pt, 0, len(r))
	for i, p := range r {
		if turn(r[(i+len(r)-1)%len(r)], p, r[(i+1)%len(r)]) != 0 {
			s = append(s, p)
		}
	}
	return s
}
func extrude(rings [][]pt, height float64) (brep.Solid, error) {
	p := make([][]geom.Vec3, len(rings))
	for i, r := range rings {
		p[i] = make([]geom.Vec3, len(r))
		for j, a := range r {
			x, _ := a.x.Float64()
			y, _ := a.y.Float64()
			if !finite(x) || !finite(y) {
				return brep.Solid{}, fmt.Errorf("coordinate overflow")
			}
			p[i][j] = geom.Vec3{X: x, Y: y}
		}
	}
	return brep.PrismWithHoles(p[0], p[1:], geom.Vec3{Z: height})
}

func cut(a, b *side) {
	p, r := a.a, minus(a.b, a.a)
	q, s := b.a, minus(b.b, b.a)
	d := cross(r, s)
	v := minus(q, p)
	if d.Sign() != 0 {
		t, u := div(cross(v, s), d), div(cross(v, r), d)
		one := number(1)
		if t.Sign() >= 0 && u.Sign() >= 0 && t.Cmp(one) <= 0 && u.Cmp(one) <= 0 {
			a.cuts = append(a.cuts, t)
			b.cuts = append(b.cuts, u)
		}
		return
	}
	if cross(v, r).Sign() != 0 {
		return
	}
	appendAt := func(a *side, p pt) {
		if !on(a.a, a.b, p) {
			return
		}
		d := minus(a.b, a.a)
		w := minus(p, a.a)
		if d.x.Sign() != 0 {
			a.cuts = append(a.cuts, div(w.x, d.x))
		} else {
			a.cuts = append(a.cuts, div(w.y, d.y))
		}
	}
	appendAt(a, b.a)
	appendAt(a, b.b)
	appendAt(b, a.a)
	appendAt(b, a.b)
}
