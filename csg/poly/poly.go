// Package poly implements bounded Boolean operations on closed, oriented planar
// solids, including nonconvex boundaries. BSP classification and splitting use
// exact rational arithmetic on the input tessellation. Output remains float64:
// rounding, nonmanifold contacts and exhausted budgets return explicit errors.
package poly

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sort"
)

type Operation uint8

const (
	Union Operation = iota
	Intersection
	Difference
)

var ErrLimit = errors.New("poly: computation limit exceeded")
var ErrInvalid = errors.New("poly: invalid boundary or options")
var ErrPrecision = errors.New("poly: output cannot be represented reliably")

// Options controls cumulative predicate work, fragment creation, recursion,
// rational numerator/denominator size, and output growth. Zero uses defaults.
// MaxConversionError bounds each vertex displacement in model units (default
// 1e-9); it never merges vertices or changes exact classification. Collision work
// is separately bounded per validation pass. Cancellation is checked between
// passes; collision validation itself is bounded but not interruptible.
type Options struct {
	MaxInputTriangles, MaxFragments, MaxDepth, MaxWork, MaxVertices, MaxOutputTriangles, MaxBits, MaxCollisionPairs int
	MaxConversionError                                                                                              float64
	// RequireExactConversion rejects any exact output coordinate that
	// cannot be represented as float64, regardless of MaxConversionError.
	RequireExactConversion bool
}

func defaults(o Options) (Options, error) {
	p := []*int{&o.MaxInputTriangles, &o.MaxFragments, &o.MaxDepth, &o.MaxWork, &o.MaxVertices, &o.MaxOutputTriangles, &o.MaxBits, &o.MaxCollisionPairs}
	d := []int{2048, 20000, 128, 2000000, 16384, 8192, 4096, 200000}
	for i, v := range p {
		if *v < 0 {
			return o, ErrInvalid
		}
		if *v == 0 {
			*v = d[i]
		}
	}
	if o.MaxConversionError == 0 {
		o.MaxConversionError = 1e-9
	}
	if o.MaxConversionError < 0 || math.IsNaN(o.MaxConversionError) || math.IsInf(o.MaxConversionError, 0) {
		return o, ErrInvalid
	}
	return o, nil
}

type budget struct {
	ctx             context.Context
	o               Options
	work, fragments int
	err             error
	sources         []Source
	exactVertices   []point
}

func (b *budget) tick(n int) bool {
	if b.err != nil {
		return false
	}
	if n > b.o.MaxWork-b.work {
		b.err = ErrLimit
		return false
	}
	b.work += n
	if e := b.ctx.Err(); e != nil {
		b.err = e
		return false
	}
	return true
}
func (b *budget) check(v *big.Rat) {
	if v.Num().BitLen() > b.o.MaxBits || v.Denom().BitLen() > b.o.MaxBits {
		b.err = ErrLimit
	}
}
func (b *budget) point(v point) {
	for _, x := range v {
		b.check(x)
	}
}

type point [3]*big.Rat
type plane struct {
	n point
	d *big.Rat
}
type polygon struct {
	v      []point
	p      plane
	source Source
}

func add(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
func sub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func mul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
func div(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(a, b) }
func neg(a *big.Rat) *big.Rat    { return new(big.Rat).Neg(a) }
func vec(v geom.Vec3) point {
	return point{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
}
func diff(a, b point) point { return point{sub(a[0], b[0]), sub(a[1], b[1]), sub(a[2], b[2])} }
func cross(a, b point) point {
	return point{sub(mul(a[1], b[2]), mul(a[2], b[1])), sub(mul(a[2], b[0]), mul(a[0], b[2])), sub(mul(a[0], b[1]), mul(a[1], b[0]))}
}
func dot(a, b point) *big.Rat { return add(add(mul(a[0], b[0]), mul(a[1], b[1])), mul(a[2], b[2])) }
func equal(a, b point) bool   { return a[0].Cmp(b[0]) == 0 && a[1].Cmp(b[1]) == 0 && a[2].Cmp(b[2]) == 0 }
func zero(a point) bool       { return a[0].Sign() == 0 && a[1].Sign() == 0 && a[2].Sign() == 0 }
func key(a point) string      { return a[0].RatString() + "," + a[1].RatString() + "," + a[2].RatString() }
func makePlane(a, b, c point) plane {
	n := cross(diff(b, a), diff(c, a))
	q := new(big.Rat)
	for _, x := range n {
		z := new(big.Rat).Abs(x)
		if z.Cmp(q) > 0 {
			q = z
		}
	}
	for i := range n {
		n[i] = div(n[i], q)
	}
	return plane{n, dot(n, a)}
}
func (p plane) distance(v point) *big.Rat { return sub(dot(p.n, v), p.d) }
func (p plane) flip() plane               { return plane{point{neg(p.n[0]), neg(p.n[1]), neg(p.n[2])}, neg(p.d)} }
func (p polygon) flip() polygon {
	v := make([]point, len(p.v))
	for i := range v {
		v[i] = p.v[len(v)-1-i]
	}
	source := p.source
	source.Reversed = !source.Reversed
	return polygon{v: v, p: p.p.flip(), source: source}
}
func (b *budget) fragment(v []point, p plane, source Source) (polygon, bool) {
	clean := make([]point, 0, len(v))
	for _, x := range v {
		if !b.tick(1) {
			return polygon{}, false
		}
		if len(clean) == 0 || !equal(x, clean[len(clean)-1]) {
			clean = append(clean, x)
		}
	}
	if len(clean) > 1 && equal(clean[0], clean[len(clean)-1]) {
		clean = clean[:len(clean)-1]
	}
	for change := true; change && len(clean) >= 3; {
		change = false
		for i := range clean {
			if !b.tick(1) {
				return polygon{}, false
			}
			if zero(cross(diff(clean[i], clean[(i+len(clean)-1)%len(clean)]), diff(clean[(i+1)%len(clean)], clean[i]))) {
				clean = append(clean[:i], clean[i+1:]...)
				change = true
				break
			}
		}
	}
	if len(clean) < 3 {
		return polygon{}, false
	}
	b.fragments++
	if b.fragments > b.o.MaxFragments {
		b.err = ErrLimit
		return polygon{}, false
	}
	for _, x := range clean {
		b.point(x)
	}
	return polygon{v: clean, p: p, source: source}, b.err == nil
}
func (b *budget) split(p plane, q polygon, cf, cb, f, back *[]polygon) {
	if !b.tick(len(q.v)) {
		return
	}
	ds := make([]*big.Rat, len(q.v))
	front, behind := false, false
	for i, v := range q.v {
		ds[i] = p.distance(v)
		b.check(ds[i])
		front = front || ds[i].Sign() > 0
		behind = behind || ds[i].Sign() < 0
	}
	if !front && !behind {
		if dot(p.n, q.p.n).Sign() > 0 {
			*cf = append(*cf, q)
		} else {
			*cb = append(*cb, q)
		}
		return
	}
	if !behind {
		*f = append(*f, q)
		return
	}
	if !front {
		*back = append(*back, q)
		return
	}
	var fv, bv []point
	for i, v := range q.v {
		j := (i + 1) % len(q.v)
		s, t := ds[i].Sign(), ds[j].Sign()
		if s >= 0 {
			fv = append(fv, v)
		}
		if s <= 0 {
			bv = append(bv, v)
		}
		if s*t < 0 {
			r := div(ds[i], sub(ds[i], ds[j]))
			var x point
			for k := range x {
				x[k] = add(v[k], mul(r, sub(q.v[j][k], v[k])))
			}
			fv = append(fv, x)
			bv = append(bv, x)
		}
	}
	if x, ok := b.fragment(fv, q.p, q.source); ok {
		*f = append(*f, x)
	}
	if x, ok := b.fragment(bv, q.p, q.source); ok {
		*back = append(*back, x)
	}
}

type node struct {
	p           *plane
	polys       []polygon
	front, back *node
	b           *budget
}

func (n *node) build(ps []polygon, depth int) {
	if len(ps) == 0 || !n.b.tick(1) {
		return
	}
	if depth > n.b.o.MaxDepth {
		n.b.err = ErrLimit
		return
	}
	if n.p == nil {
		p := ps[0].p
		n.p = &p
	}
	var f, b []polygon
	for _, p := range ps {
		n.b.split(*n.p, p, &n.polys, &n.polys, &f, &b)
		if n.b.err != nil {
			return
		}
	}
	if len(f) > 0 {
		if n.front == nil {
			n.front = &node{b: n.b}
		}
		n.front.build(f, depth+1)
	}
	if len(b) > 0 {
		if n.back == nil {
			n.back = &node{b: n.b}
		}
		n.back.build(b, depth+1)
	}
}
func (n *node) invert() {
	if n == nil || !n.b.tick(1) {
		return
	}
	for i := range n.polys {
		n.polys[i] = n.polys[i].flip()
	}
	if n.p != nil {
		p := n.p.flip()
		n.p = &p
	}
	n.front.invert()
	n.back.invert()
	n.front, n.back = n.back, n.front
}
func (n *node) clip(ps []polygon) []polygon {
	if n == nil || n.p == nil {
		return ps
	}
	if !n.b.tick(1) {
		return nil
	}
	var f, b []polygon
	for _, p := range ps {
		n.b.split(*n.p, p, &f, &b, &f, &b)
		if n.b.err != nil {
			return nil
		}
	}
	if n.front != nil {
		f = n.front.clip(f)
	}
	if n.back != nil {
		b = n.back.clip(b)
	} else {
		b = nil
	}
	return append(f, b...)
}
func (n *node) clipTo(other *node) {
	if n == nil || !n.b.tick(1) {
		return
	}
	n.polys = other.clip(n.polys)
	n.front.clipTo(other)
	n.back.clipTo(other)
}
func (n *node) all() []polygon {
	if n == nil || !n.b.tick(1) {
		return nil
	}
	p := append([]polygon(nil), n.polys...)
	p = append(p, n.front.all()...)
	return append(p, n.back.all()...)
}

// Combine applies a regularized Boolean to the exact oriented triangle surfaces
// represented by the inputs. Inputs must be embedded manifolds with outward
// material boundaries (cavity shells inward). Edge/vertex-only contacts that
// cannot form an embedded manifold are rejected. No convex decomposition is used.
func combine(ctx context.Context, a, b brep.Solid, op Operation, o Options, history *[]Source) (brep.Solid, error) {
	return combineDetailed(ctx, a, b, op, o, history, nil)
}
func combineDetailed(ctx context.Context, a, b brep.Solid, op Operation, o Options, history *[]Source, exactVertices *[]point) (result brep.Solid, retErr error) {
	if ctx == nil || op > Difference {
		return brep.Solid{}, ErrInvalid
	}
	var err error
	o, err = defaults(o)
	if err != nil {
		return brep.Solid{}, err
	}
	bu := &budget{ctx: ctx, o: o}
	defer func() {
		if retErr == nil {
			*history = bu.sources
			if exactVertices != nil {
				*exactVertices = bu.exactVertices
			}
		}
	}()
	if !bu.tick(1) {
		return brep.Solid{}, bu.err
	}
	ap, err := input(a, bu)
	if err != nil {
		return brep.Solid{}, fmt.Errorf("poly operand A: %w", err)
	}
	bp, err := input(b, bu)
	if err != nil {
		return brep.Solid{}, fmt.Errorf("poly operand B: %w", err)
	}
	for i := range ap {
		ap[i].source = Source{Operand: OperandA, Triangle: i}
	}
	for i := range bp {
		bp[i].source = Source{Operand: OperandB, Triangle: i}
	}
	if len(ap) == 0 || len(bp) == 0 {
		switch op {
		case Union:
			if len(ap) == 0 {
				return assemble(bp, bu)
			}
			return assemble(ap, bu)
		case Difference:
			if len(bp) == 0 {
				return assemble(ap, bu)
			}
		}
		return brep.Solid{}, nil
	}
	ps := booleanPolygons(ap, bp, op, bu)
	if bu.err != nil {
		return brep.Solid{}, bu.err
	}
	return assemble(ps, bu)
}

func booleanPolygons(ap, bp []polygon, op Operation, bu *budget) []polygon {
	an, bn := &node{b: bu}, &node{b: bu}
	an.build(ap, 0)
	bn.build(bp, 0)
	switch op {
	case Union:
		an.clipTo(bn)
		bn.clipTo(an)
		bn.invert()
		bn.clipTo(an)
		bn.invert()
		an.build(bn.all(), 0)
	case Difference:
		an.invert()
		an.clipTo(bn)
		bn.clipTo(an)
		bn.invert()
		bn.clipTo(an)
		bn.invert()
		an.build(bn.all(), 0)
		an.invert()
	case Intersection:
		an.invert()
		bn.clipTo(an)
		bn.invert()
		an.clipTo(bn)
		bn.clipTo(an)
		an.build(bn.all(), 0)
		an.invert()
	}
	return an.all()
}

func geometricCheck(m mesh.Mesh, b *budget) error {
	if !b.tick(1) {
		return b.err
	}
	if err := m.ValidateSolid(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	r, err := collision.SelfIntersections(m, collision.Options{MaxPairs: b.o.MaxCollisionPairs, MaxContacts: 1})
	if len(r.Contacts) > 0 {
		return fmt.Errorf("%w: intersecting boundaries", ErrInvalid)
	}
	if err != nil {
		if errors.Is(err, collision.ErrLimit) {
			return fmt.Errorf("%w: collision validation", ErrLimit)
		}
		return fmt.Errorf("poly collision validation: %w", err)
	}
	if !r.Complete {
		return fmt.Errorf("%w: intersecting boundaries", ErrInvalid)
	}
	if !b.tick(1) {
		return b.err
	}
	return nil
}
func input(s brep.Solid, b *budget) ([]polygon, error) {
	m := s.Tessellate()
	if len(m.Triangles) > b.o.MaxInputTriangles {
		return nil, ErrLimit
	}
	if len(m.Triangles) == 0 {
		return nil, nil
	}
	if err := geometricCheck(m, b); err != nil {
		return nil, err
	}
	ps := make([]polygon, len(m.Triangles))
	for i, t := range m.Triangles {
		v := []point{vec(m.Vertices[t[0]]), vec(m.Vertices[t[1]]), vec(m.Vertices[t[2]])}
		for _, x := range v {
			b.point(x)
		}
		p := makePlane(v[0], v[1], v[2])
		b.point(p.n)
		b.check(p.d)
		ps[i] = polygon{v: v, p: p}
	}
	if b.err != nil {
		return nil, b.err
	}
	if err := orientations(m, ps, b); err != nil {
		return nil, err
	}
	return ps, nil
}

func assemble(ps []polygon, b *budget) (brep.Solid, error) {
	if len(ps) == 0 {
		return brep.Solid{}, nil
	}
	var pts []point
	ids := map[string]int{}
	intern := func(p point) int {
		k := key(p)
		if i, ok := ids[k]; ok {
			return i
		}
		i := len(pts)
		ids[k] = i
		pts = append(pts, p)
		if len(pts) > b.o.MaxVertices {
			b.err = ErrLimit
		}
		return i
	}
	for _, p := range ps {
		for _, v := range p.v {
			intern(v)
		}
	}
	boundary := append([]point(nil), pts...)
	var tris [][3]int
	var sources []Source
	for _, p := range ps {
		var loop []int
		for i, v := range p.v {
			w := p.v[(i+1)%len(p.v)]
			d := diff(w, v)
			axis := 0
			for d[axis].Sign() == 0 {
				axis++
			}
			type cut struct {
				t *big.Rat
				i int
			}
			cuts := []cut{{new(big.Rat), intern(v)}}
			for j, x := range boundary {
				if !b.tick(1) {
					return brep.Solid{}, b.err
				}
				if equal(x, v) || equal(x, w) {
					continue
				}
				dx := diff(x, v)
				if !zero(cross(d, dx)) {
					continue
				}
				t := div(dx[axis], d[axis])
				if t.Sign() > 0 && t.Cmp(big.NewRat(1, 1)) < 0 {
					cuts = append(cuts, cut{t, j})
				}
			}
			sort.Slice(cuts, func(i, j int) bool { return cuts[i].t.Cmp(cuts[j].t) < 0 })
			for _, c := range cuts {
				loop = append(loop, c.i)
			}
		}
		if len(loop) == 3 {
			tris = append(tris, [3]int{loop[0], loop[1], loop[2]})
			sources = append(sources, p.source)
		} else if b.o.RequireExactConversion {
			// Do not introduce a centroid whose rational denominator can
			// prevent exact conversion even when every boundary vertex fits.
			// The fragment is convex; clip strict ears while preserving
			// every conforming subdivision on the boundary.
			ears := exactConvexEars(loop, pts, p.p.n, b)
			if b.err != nil {
				return brep.Solid{}, b.err
			}
			for _, t := range ears {
				tris = append(tris, t)
				sources = append(sources, p.source)
			}
		} else {
			center := point{new(big.Rat), new(big.Rat), new(big.Rat)}
			for _, v := range p.v {
				for k := range center {
					center[k] = add(center[k], v[k])
				}
			}
			for k := range center {
				center[k] = div(center[k], big.NewRat(int64(len(p.v)), 1))
			}
			b.point(center)
			ci := intern(center)
			for i, j := range loop {
				tris = append(tris, [3]int{ci, j, loop[(i+1)%len(loop)]})
				sources = append(sources, p.source)
			}
		}
		if len(tris) > b.o.MaxOutputTriangles {
			return brep.Solid{}, ErrLimit
		}
		if b.err != nil {
			return brep.Solid{}, b.err
		}
	}
	vs := make([]geom.Vec3, len(pts))
	rounded := map[geom.Vec3]int{}
	tol := new(big.Rat).SetFloat64(b.o.MaxConversionError)
	tol = mul(tol, tol)
	for i, p := range pts {
		var xyz [3]float64
		for j := range xyz {
			if b.o.RequireExactConversion {
				var e error
				xyz[j], e = geom.RepresentedExactScalar(p[j])
				if e != nil {
					return brep.Solid{}, fmt.Errorf("%w: exact output coordinate is not representable", ErrPrecision)
				}
			} else {
				xyz[j], _ = p[j].Float64()
			}
		}
		v := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
		if !v.IsFinite() {
			return brep.Solid{}, ErrPrecision
		}
		if _, ok := rounded[v]; ok {
			return brep.Solid{}, fmt.Errorf("%w: distinct vertices collapse", ErrPrecision)
		}
		rounded[v] = i
		dv := diff(vec(v), p)
		if dot(dv, dv).Cmp(tol) > 0 {
			return brep.Solid{}, ErrPrecision
		}
		vs[i] = v
	}
	m := mesh.Mesh{Vertices: vs, Triangles: tris}
	if err := geometricCheck(m, b); err != nil {
		return brep.Solid{}, fmt.Errorf("poly result: %w", err)
	}
	ops := make([]polygon, len(tris))
	for i, t := range tris {
		v := []point{vec(vs[t[0]]), vec(vs[t[1]]), vec(vs[t[2]])}
		p := makePlane(v[0], v[1], v[2])
		b.point(p.n)
		b.check(p.d)
		ops[i] = polygon{v: v, p: p}
	}
	if err := orientations(m, ops, b); err != nil {
		return brep.Solid{}, fmt.Errorf("poly result: %w", err)
	}
	edges := []brep.Edge{}
	em := map[[2]int]int{}
	faces := make([]brep.Face, len(tris))
	for i, t := range tris {
		for j, v := range t {
			w := t[(j+1)%3]
			pair := [2]int{v, w}
			rev := v > w
			if rev {
				pair = [2]int{w, v}
			}
			ei, ok := em[pair]
			if !ok {
				ei = len(edges)
				em[pair] = ei
				edges = append(edges, brep.Edge{Start: pair[0], End: pair[1]})
			}
			faces[i].Boundary = append(faces[i].Boundary, brep.EdgeUse{Edge: ei, Reversed: rev})
		}
	}
	s, err := brep.NewSolid(vs, edges, faces)
	if err != nil {
		return brep.Solid{}, fmt.Errorf("%w: %v", ErrPrecision, err)
	}
	b.sources = sources
	b.exactVertices = pts
	return s, nil
}

func exactConvexEars(loop []int, pts []point, normal point, b *budget) [][3]int {
	var ears [][3]int
	origin := pts[loop[0]]
	area := new(big.Rat)
	for i := 1; i+1 < len(loop); i++ {
		if !b.tick(1) {
			return nil
		}
		area.Add(area, dot(cross(diff(pts[loop[i]], origin), diff(pts[loop[i+1]], origin)), normal))
	}
	b.check(area)
	if b.err != nil {
		return nil
	}
	for len(loop) > 3 {
		n := len(loop)
		found := false
		for i, id := range loop {
			if !b.tick(1) {
				return nil
			}
			left, right := loop[(i+n-1)%n], loop[(i+1)%n]
			a := dot(cross(diff(pts[id], pts[left]), diff(pts[right], pts[left])), normal)
			b.check(a)
			if b.err != nil {
				return nil
			}
			if a.Sign() <= 0 || a.Cmp(area) >= 0 {
				continue
			}
			ears = append(ears, [3]int{left, id, right})
			area.Sub(area, a)
			b.check(area)
			if b.err != nil {
				return nil
			}
			loop = append(loop[:i], loop[i+1:]...)
			found = true
			break
		}
		if !found {
			b.err = ErrInvalid
			return nil
		}
	}
	return append(ears, [3]int{loop[0], loop[1], loop[2]})
}
