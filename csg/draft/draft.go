// Package draft edits supporting planes of convex planar BReps using exact
// affine images, then reconstructs their bounded halfspace intersection.
package draft

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/collision"
	"plainsolid/geom"
	"sort"
)

var ErrInvalid = errors.New("draft: invalid convex solid, transform or options")
var ErrUnsupported = errors.New("draft: unsupported face selection")
var ErrUnbounded = errors.New("draft: supporting normals admit an unbounded recession direction")
var ErrLimit = errors.New("draft: resource limit exceeded")
var ErrPrecision = errors.New("draft: output conversion cannot preserve boundary")

// FaceTransform applies the given affine image to one original supporting
// halfspace. Its determinant must be nonzero. Other supports remain fixed.
type FaceTransform struct {
	Face      int
	Transform geom.Affine
}

// FaceShear moves a point x to x+Displacement*(neutral.Normal dot
// (x-neutral.Origin)). This fixes the neutral plane exactly. The coefficient is
// displacement per unit of the represented plane coordinate, not a draft angle.
type FaceShear struct {
	Face         int
	Displacement geom.Vec3
}

// Options separately bounds exact reconstruction and each geometry validation
// pass. Defaults: 256 source faces, 32 unique supports, 4096 output vertices,
// 8192 validation triangles, 2000000 work units, 16384 rational bits, 200000
// collision pairs, and 1e-9 maximum Euclidean coordinate conversion error.
type Options struct {
	MaxFaces, MaxPlanes, MaxVertices, MaxTriangles, MaxWork, MaxBits, MaxCollisionPairs int
	MaxCoordinateError                                                                  float64
}

// Halfspace is Normal dot x <= Offset, with exact caller-owned coefficients.
// Normal is normalized by its largest absolute component, not Euclidean length.
type Halfspace struct {
	Normal [3]*big.Rat
	Offset *big.Rat
}

// Result preserves exact vertices and volume alongside the rounded planar BRep.
// FaceSources and FacePlanes align with output faces. OldToNew is -1 for a
// vanished supporting face. Coplanar source faces can share one output face.
// Empty regularized results have zero volume and no boundary.
type Result struct {
	Solid                               brep.Solid
	ExactVertices                       [][3]*big.Rat
	ExactVolume, MaxSquaredDisplacement *big.Rat
	FaceSources                         [][]int
	FacePlanes                          []Halfspace
	OldToNew                            []int
}
type q = *big.Rat
type point [3]q
type matrix [3][4]q
type plane struct {
	n       point
	d       q
	key     string
	sources []int
}

func num(x float64) q          { return new(big.Rat).SetFloat64(x) }
func add(a, b q) q             { return new(big.Rat).Add(a, b) }
func sub(a, b q) q             { return new(big.Rat).Sub(a, b) }
func mul(a, b q) q             { return new(big.Rat).Mul(a, b) }
func div(a, b q) q             { return new(big.Rat).Quo(a, b) }
func exact(v geom.Vec3) point  { return point{num(v.X), num(v.Y), num(v.Z)} }
func plus(a, b point) point    { return point{add(a[0], b[0]), add(a[1], b[1]), add(a[2], b[2])} }
func diff(a, b point) point    { return point{sub(a[0], b[0]), sub(a[1], b[1]), sub(a[2], b[2])} }
func scale(a point, s q) point { return point{mul(a[0], s), mul(a[1], s), mul(a[2], s)} }
func cross(a, b point) point {
	return point{sub(mul(a[1], b[2]), mul(a[2], b[1])), sub(mul(a[2], b[0]), mul(a[0], b[2])), sub(mul(a[0], b[1]), mul(a[1], b[0]))}
}
func dot(a, b point) q   { return add(add(mul(a[0], b[0]), mul(a[1], b[1])), mul(a[2], b[2])) }
func zero(a point) bool  { return a[0].Sign() == 0 && a[1].Sign() == 0 && a[2].Sign() == 0 }
func key(a point) string { return a[0].RatString() + "," + a[1].RatString() + "," + a[2].RatString() }
func normalize(n point, d q) (plane, bool) {
	m := new(big.Rat)
	for _, x := range n {
		a := new(big.Rat).Abs(x)
		if a.Cmp(m) > 0 {
			m = a
		}
	}
	if m.Sign() == 0 {
		return plane{}, false
	}
	for i := range n {
		n[i] = div(n[i], m)
	}
	d = div(d, m)
	return plane{n: n, d: d, key: key(n) + "," + d.RatString()}, true
}
func through(a, b, c point) (plane, bool) {
	n := cross(diff(b, a), diff(c, a))
	return normalize(n, dot(n, a))
}
func (m matrix) det() q {
	return dot(point{m[0][0], m[0][1], m[0][2]}, cross(point{m[1][0], m[1][1], m[1][2]}, point{m[2][0], m[2][1], m[2][2]}))
}
func (m matrix) apply(p point) point {
	var r point
	for i := 0; i < 3; i++ {
		r[i] = add(dot(point{m[i][0], m[i][1], m[i][2]}, p), m[i][3])
	}
	return r
}

type budget struct {
	ctx  context.Context
	o    Options
	work int
	err  error
}

func (b *budget) tick() bool {
	if b.err != nil {
		return false
	}
	if e := b.ctx.Err(); e != nil {
		b.err = e
		return false
	}
	if b.work >= b.o.MaxWork {
		b.err = ErrLimit
		return false
	}
	b.work++
	return true
}
func (b *budget) check(x q) {
	if x.Num().BitLen() > b.o.MaxBits || x.Denom().BitLen() > b.o.MaxBits {
		b.err = ErrLimit
	}
}
func (b *budget) point(p point) {
	for _, x := range p {
		b.check(x)
	}
}
func options(o Options) (Options, error) {
	if math.IsNaN(o.MaxCoordinateError) || math.IsInf(o.MaxCoordinateError, 0) || o.MaxCoordinateError < 0 {
		return o, ErrInvalid
	}
	ps := []*int{&o.MaxFaces, &o.MaxPlanes, &o.MaxVertices, &o.MaxTriangles, &o.MaxWork, &o.MaxBits, &o.MaxCollisionPairs}
	ds := []int{256, 32, 4096, 8192, 2000000, 16384, 200000}
	for i, p := range ps {
		if *p < 0 {
			return o, ErrInvalid
		}
		if *p == 0 {
			*p = ds[i]
		}
	}
	if o.MaxPlanes > 128 || o.MaxTriangles > collision.MaxTriangles {
		return o, ErrInvalid
	}
	if o.MaxCoordinateError == 0 {
		o.MaxCoordinateError = 1e-9
	}
	return o, nil
}

// Apply edits selected exact supporting halfspaces. All faces sharing an original
// support must receive the same resulting support, or selection is ambiguous.
// Normals admitting a nonzero recession direction fail explicitly, even when
// their offsets might separately make the intersection empty. Faces may vanish.
func Apply(ctx context.Context, s brep.Solid, changes []FaceTransform, o Options) (Result, error) {
	if ctx == nil {
		return Result{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Result{}, e
	}
	var e error
	o, e = options(o)
	if e != nil {
		return Result{}, e
	}
	if len(changes) > o.MaxFaces {
		return Result{}, ErrLimit
	}
	transforms := map[int]matrix{}
	for _, c := range changes {
		if _, ok := transforms[c.Face]; ok {
			return Result{}, ErrInvalid
		}
		var m matrix
		for i := range m {
			for j := range m[i] {
				x := c.Transform[i][j]
				if math.IsNaN(x) || math.IsInf(x, 0) {
					return Result{}, ErrInvalid
				}
				m[i][j] = num(x)
			}
		}
		if m.det().Sign() == 0 {
			return Result{}, ErrInvalid
		}
		transforms[c.Face] = m
	}
	return apply(ctx, s, transforms, o)
}

// Shear constructs neutral-plane fixing affine maps exactly, without first
// rounding a floating matrix. Different selected faces can have different pulls.
func Shear(ctx context.Context, s brep.Solid, neutral geom.Plane, changes []FaceShear, o Options) (Result, error) {
	if ctx == nil {
		return Result{}, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Result{}, e
	}
	var e error
	o, e = options(o)
	if e != nil {
		return Result{}, e
	}
	if len(changes) > o.MaxFaces {
		return Result{}, ErrLimit
	}
	if !neutral.Origin().IsFinite() || !neutral.Normal().IsFinite() || neutral.Normal() == (geom.Vec3{}) {
		return Result{}, ErrInvalid
	}
	n := exact(neutral.Normal())
	offset := dot(n, exact(neutral.Origin()))
	transforms := map[int]matrix{}
	for _, c := range changes {
		if !c.Displacement.IsFinite() {
			return Result{}, ErrInvalid
		}
		if _, ok := transforms[c.Face]; ok {
			return Result{}, ErrInvalid
		}
		d := exact(c.Displacement)
		var m matrix
		for i := range m {
			for j := 0; j < 3; j++ {
				m[i][j] = mul(d[i], n[j])
				if i == j {
					m[i][j] = add(m[i][j], big.NewRat(1, 1))
				}
			}
			m[i][3] = new(big.Rat).Neg(mul(d[i], offset))
		}
		if m.det().Sign() == 0 {
			return Result{}, ErrInvalid
		}
		transforms[c.Face] = m
	}
	return apply(ctx, s, transforms, o)
}
func validate(ctx context.Context, s brep.Solid, o Options) error {
	_, e := collision.NewPointLocator(ctx, s, collision.LocationOptions{MaxTriangles: o.MaxTriangles, MaxWork: o.MaxWork, MaxCollisionPairs: o.MaxCollisionPairs})
	if errors.Is(e, collision.ErrLimit) {
		return fmt.Errorf("%w: validation", ErrLimit)
	}
	return e
}
func apply(ctx context.Context, s brep.Solid, transforms map[int]matrix, o Options) (Result, error) {
	return applyExtra(ctx, s, transforms, nil, o)
}
func applyExtra(ctx context.Context, s brep.Solid, transforms map[int]matrix, extra []plane, o Options) (Result, error) {
	return applyFiltered(ctx, s, transforms, extra, nil, o)
}
func applyFiltered(ctx context.Context, s brep.Solid, transforms map[int]matrix, extra []plane, removed map[int]bool, o Options) (Result, error) {
	empty := Result{}
	if ctx == nil {
		return empty, ErrInvalid
	}
	var e error
	o, e = options(o)
	if e != nil {
		return empty, e
	}
	b := &budget{ctx: ctx, o: o}
	if !b.tick() {
		return empty, b.err
	}
	faces, edges, vertices := s.Faces(), s.Edges(), s.Vertices()
	if len(faces) > o.MaxFaces {
		return empty, ErrLimit
	}
	for i, m := range transforms {
		if i < 0 || i >= len(faces) {
			return empty, ErrInvalid
		}
		for _, row := range m {
			for _, x := range row {
				b.check(x)
			}
		}
	}
	if b.err != nil {
		return empty, b.err
	}
	result := Result{ExactVolume: new(big.Rat), MaxSquaredDisplacement: new(big.Rat), OldToNew: make([]int, len(faces))}
	for i := range result.OldToNew {
		result.OldToNew[i] = -1
	}
	if len(faces) == 0 {
		return result, nil
	}
	if s.Topology().Shells != 1 {
		return empty, ErrInvalid
	}
	if e = validate(ctx, s, o); e != nil {
		return empty, e
	}
	points := make([]point, len(vertices))
	for i, v := range vertices {
		points[i] = exact(v)
		b.point(points[i])
	}
	var planes []plane
	dedup := map[string]int{}
	originalImages := map[string]string{}
	for i, f := range faces {
		if !b.tick() {
			return empty, b.err
		}
		var ids []int
		for _, u := range f.Boundary {
			x := edges[u.Edge].Start
			if u.Reversed {
				x = edges[u.Edge].End
			}
			ids = append(ids, x)
		}
		p, ok := through(points[ids[0]], points[ids[1]], points[ids[2]])
		if !ok {
			return empty, ErrInvalid
		}
		for _, loop := range append([][]brep.EdgeUse{f.Boundary}, f.Holes...) {
			for _, u := range loop {
				if !b.tick() {
					return empty, b.err
				}
				if dot(p.n, points[edges[u.Edge].Start]).Cmp(p.d) != 0 || dot(p.n, points[edges[u.Edge].End]).Cmp(p.d) != 0 {
					return empty, fmt.Errorf("%w: face not exactly planar", ErrInvalid)
				}
			}
		}
		for _, v := range points {
			if !b.tick() {
				return empty, b.err
			}
			if dot(p.n, v).Cmp(p.d) > 0 {
				return empty, fmt.Errorf("%w: input is not convex", ErrInvalid)
			}
		}
		original := p.key
		if m, selected := transforms[i]; selected {
			a, c, d := m.apply(points[ids[0]]), m.apply(points[ids[1]]), m.apply(points[ids[2]])
			b.point(a)
			b.point(c)
			b.point(d)
			n := cross(diff(c, a), diff(d, a))
			if m.det().Sign() < 0 {
				n = scale(n, big.NewRat(-1, 1))
			}
			p, ok = normalize(n, dot(n, a))
			if !ok {
				return empty, ErrInvalid
			}
		}
		b.point(p.n)
		b.check(p.d)
		if old, ok := originalImages[original]; ok && old != p.key {
			return empty, fmt.Errorf("%w: split selection on one coplanar support", ErrUnsupported)
		}
		originalImages[original] = p.key
		if removed[i] {
			continue
		}
		if index, ok := dedup[p.key]; ok {
			planes[index].sources = append(planes[index].sources, i)
		} else {
			dedup[p.key] = len(planes)
			p.sources = []int{i}
			planes = append(planes, p)
		}
		if len(planes) > o.MaxPlanes {
			return empty, ErrLimit
		}
	}
	if b.err != nil {
		return empty, b.err
	}
	for _, p := range extra {
		b.point(p.n)
		b.check(p.d)
		if i, ok := dedup[p.key]; ok {
			planes[i].sources = append(planes[i].sources, p.sources...)
		} else {
			dedup[p.key] = len(planes)
			planes = append(planes, p)
		}
	}
	if len(planes) > o.MaxPlanes || b.err != nil {
		return empty, ErrLimit
	}
	if e = bounded(planes, b); e != nil {
		return empty, e
	}
	var out []point
	known := map[string]bool{}
	for i := 0; i < len(planes); i++ {
		for j := i + 1; j < len(planes); j++ {
			for k := j + 1; k < len(planes); k++ {
				p, ok := intersection(planes[i], planes[j], planes[k], b)
				if b.err != nil {
					return empty, b.err
				}
				if !ok || !feasible(p, planes, b) {
					if b.err != nil {
						return empty, b.err
					}
					continue
				}
				id := key(p)
				if !known[id] {
					known[id] = true
					out = append(out, p)
					if len(out) > o.MaxVertices {
						return empty, ErrLimit
					}
				}
			}
		}
	}
	if !fullDimension(out, b) {
		if b.err != nil {
			return empty, b.err
		}
		return result, nil
	}
	var loops [][]int
	var supports []plane
	for _, p := range planes {
		var ids []int
		for i, v := range out {
			if !b.tick() {
				return empty, b.err
			}
			if dot(p.n, v).Cmp(p.d) == 0 {
				ids = append(ids, i)
			}
		}
		h := hull(ids, out, p.n, b)
		if b.err != nil {
			return empty, b.err
		}
		if len(h) >= 3 {
			loops = append(loops, h)
			supports = append(supports, p)
		}
	}
	return assemble(out, loops, supports, result, b)
}
func intersection(a, c, d plane, b *budget) (point, bool) {
	if !b.tick() {
		return point{}, false
	}
	u, v, w := cross(c.n, d.n), cross(d.n, a.n), cross(a.n, c.n)
	det := dot(a.n, u)
	if det.Sign() == 0 {
		return point{}, false
	}
	p := scale(plus(plus(scale(u, a.d), scale(v, c.d)), scale(w, d.d)), div(big.NewRat(1, 1), det))
	b.point(p)
	return p, b.err == nil
}
func feasible(p point, planes []plane, b *budget) bool {
	for _, h := range planes {
		if !b.tick() {
			return false
		}
		if dot(h.n, p).Cmp(h.d) > 0 {
			return false
		}
	}
	return true
}

// A nonzero recession direction exists iff the homogeneous constraints meet the
// boundary of [-1,1]^3. Such a bounded intersection has a vertex with at least
// one active cube face, so triples containing a cube face are exhaustive.
func bounded(ps []plane, b *budget) error {
	all := make([]plane, len(ps))
	for i, p := range ps {
		all[i] = plane{n: p.n, d: new(big.Rat)}
	}
	for k := 0; k < 3; k++ {
		for _, sign := range []int64{-1, 1} {
			n := point{new(big.Rat), new(big.Rat), new(big.Rat)}
			n[k] = big.NewRat(sign, 1)
			all = append(all, plane{n: n, d: big.NewRat(1, 1)})
		}
	}
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			for k := j + 1; k < len(all); k++ {
				if k < len(ps) {
					continue
				}
				p, ok := intersection(all[i], all[j], all[k], b)
				if b.err != nil {
					return b.err
				}
				if ok && !zero(p) && feasible(p, all, b) {
					return ErrUnbounded
				}
				if b.err != nil {
					return b.err
				}
			}
		}
	}
	return nil
}
func fullDimension(ps []point, b *budget) bool {
	if len(ps) < 4 {
		return false
	}
	a := ps[0]
	var direction, normal point
	haveDirection, haveNormal := false, false
	for _, p := range ps[1:] {
		if !b.tick() {
			return false
		}
		d := diff(p, a)
		if !haveDirection {
			if !zero(d) {
				direction = d
				haveDirection = true
			}
			continue
		}
		if !haveNormal {
			n := cross(direction, d)
			if !zero(n) {
				normal = n
				haveNormal = true
			}
			continue
		}
		if dot(normal, d).Sign() != 0 {
			return true
		}
	}
	return false
}
func hull(ids []int, points []point, n point, b *budget) []int {
	if len(ids) < 3 {
		return nil
	}
	axis := 0
	for n[axis].Sign() == 0 {
		axis++
	}
	u, v := (axis+1)%3, (axis+2)%3
	sort.Slice(ids, func(i, j int) bool {
		b.tick()
		a, c := points[ids[i]], points[ids[j]]
		if cmp := a[u].Cmp(c[u]); cmp != 0 {
			return cmp < 0
		}
		return a[v].Cmp(c[v]) < 0
	})
	turn := func(a, c, d int) int {
		x, y, z := points[a], points[c], points[d]
		return sub(mul(sub(y[u], x[u]), sub(z[v], x[v])), mul(sub(y[v], x[v]), sub(z[u], x[u]))).Sign()
	}
	chain := func(order []int) []int {
		var h []int
		for _, i := range order {
			if !b.tick() {
				return nil
			}
			for len(h) >= 2 && turn(h[len(h)-2], h[len(h)-1], i) <= 0 {
				h = h[:len(h)-1]
			}
			h = append(h, i)
		}
		return h
	}
	lo := chain(ids)
	if b.err != nil {
		return nil
	}
	rev := append([]int(nil), ids...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	hi := chain(rev)
	if b.err != nil {
		return nil
	}
	out := append(lo[:len(lo)-1], hi[:len(hi)-1]...)
	if n[axis].Sign() < 0 {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}
func assemble(points []point, loops [][]int, supports []plane, r Result, b *budget) (Result, error) {
	empty := Result{}
	vertices := make([]geom.Vec3, len(points))
	seen := map[geom.Vec3]bool{}
	tol2 := mul(num(b.o.MaxCoordinateError), num(b.o.MaxCoordinateError))
	for i, p := range points {
		if !b.tick() {
			return empty, b.err
		}
		var xyz [3]float64
		for j, x := range p {
			xyz[j], _ = x.Float64()
		}
		v := geom.Vec3{X: xyz[0], Y: xyz[1], Z: xyz[2]}
		if !v.IsFinite() || seen[v] {
			return empty, ErrPrecision
		}
		seen[v] = true
		d := diff(exact(v), p)
		error2 := dot(d, d)
		if error2.Cmp(tol2) > 0 {
			return empty, ErrPrecision
		}
		if error2.Cmp(r.MaxSquaredDisplacement) > 0 {
			r.MaxSquaredDisplacement = new(big.Rat).Set(error2)
		}
		vertices[i] = v
		var exactPoint [3]q
		for k, x := range p {
			exactPoint[k] = new(big.Rat).Set(x)
		}
		r.ExactVertices = append(r.ExactVertices, exactPoint)
	}
	var edges []brep.Edge
	var faces []brep.Face
	edgeMap := map[[2]int]int{}
	origin := points[0]
	for i, loop := range loops {
		f := brep.Face{}
		for j, a := range loop {
			if !b.tick() {
				return empty, b.err
			}
			c := loop[(j+1)%len(loop)]
			rev := a > c
			if rev {
				a, c = c, a
			}
			key := [2]int{a, c}
			id, ok := edgeMap[key]
			if !ok {
				id = len(edges)
				edgeMap[key] = id
				edges = append(edges, brep.Edge{Start: a, End: c})
			}
			f.Boundary = append(f.Boundary, brep.EdgeUse{Edge: id, Reversed: rev})
		}
		for j := 1; j+1 < len(loop); j++ {
			if !b.tick() {
				return empty, b.err
			}
			r.ExactVolume.Add(r.ExactVolume, div(dot(diff(points[loop[0]], origin), cross(diff(points[loop[j]], origin), diff(points[loop[j+1]], origin))), big.NewRat(6, 1)))
		}
		b.check(r.ExactVolume)
		if b.err != nil {
			return empty, b.err
		}
		faces = append(faces, f)
		p := supports[i]
		r.FaceSources = append(r.FaceSources, append([]int(nil), p.sources...))
		h := Halfspace{Offset: new(big.Rat).Set(p.d)}
		for k, x := range p.n {
			h.Normal[k] = new(big.Rat).Set(x)
		}
		r.FacePlanes = append(r.FacePlanes, h)
		for _, old := range p.sources {
			if old >= 0 {
				r.OldToNew[old] = i
			}
		}
	}
	if r.ExactVolume.Sign() <= 0 {
		return empty, ErrInvalid
	}
	s, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return empty, fmt.Errorf("%w: %v", ErrPrecision, e)
	}
	if e = validate(b.ctx, s, b.o); e != nil {
		return empty, fmt.Errorf("draft result: %w", e)
	}
	r.Solid = s
	return r, nil
}
