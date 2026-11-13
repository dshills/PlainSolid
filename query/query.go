// Package query provides spatial queries on immutable copies of triangle meshes.
// Classification assumes a closed, consistently outward-oriented mesh with no
// self-intersections and correctly reversed cavity shells.
package query

import (
	"errors"
	"math"
	"plainsolid/geom"
	"plainsolid/mesh"
	"sort"
)

// Index owns a copy of mesh geometry and an axis-aligned bounding-volume tree.
// It is safe for concurrent queries. Build validates geometry, not embedding.
type Index struct {
	m     mesh.Mesh
	nodes []node
	order []int
}
type node struct {
	box                     geom.Box
	start, end, left, right int
}

func New(m mesh.Mesh) (*Index, error) {
	if e := m.Validate(); e != nil {
		return nil, e
	}
	q := &Index{m: mesh.Mesh{Vertices: append([]geom.Vec3(nil), m.Vertices...), Triangles: append([][3]int(nil), m.Triangles...)}, order: make([]int, len(m.Triangles))}
	for i := range q.order {
		q.order[i] = i
	}
	if len(q.order) > 0 {
		q.build(0, len(q.order))
	}
	return q, nil
}
func (q *Index) triangleBox(f int) geom.Box {
	t := q.m.Triangles[f]
	b, _ := geom.Bounds([]geom.Vec3{q.m.Vertices[t[0]], q.m.Vertices[t[1]], q.m.Vertices[t[2]]})
	return b
}
func (q *Index) build(start, end int) int {
	b := q.triangleBox(q.order[start])
	for _, f := range q.order[start+1 : end] {
		b, _ = b.Union(q.triangleBox(f))
	}
	n := len(q.nodes)
	q.nodes = append(q.nodes, node{b, start, end, -1, -1})
	if end-start <= 8 {
		return n
	}
	extent := b.Size()
	axis := 0
	if extent.Y > extent.X {
		axis = 1
	}
	if component(extent, 2) > component(extent, axis) {
		axis = 2
	}
	sort.Slice(q.order[start:end], func(i, j int) bool {
		a, c := q.triangleBox(q.order[start+i]), q.triangleBox(q.order[start+j])
		x := component(a.Min, axis)*.5 + component(a.Max, axis)*.5
		y := component(c.Min, axis)*.5 + component(c.Max, axis)*.5
		if x == y {
			return q.order[start+i] < q.order[start+j]
		}
		return x < y
	})
	mid := start + (end-start)/2
	left := q.build(start, mid)
	right := q.build(mid, end)
	q.nodes[n].left = left
	q.nodes[n].right = right
	return n
}
func component(p geom.Vec3, i int) float64 {
	switch i {
	case 0:
		return p.X
	case 1:
		return p.Y
	default:
		return p.Z
	}
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Hit identifies a point on a triangle. Barycentric components sum to one.
// Distance is Euclidean distance; Ray normalizes the input direction.
type Hit struct {
	Point       geom.Vec3
	Distance    float64
	Triangle    int
	Barycentric geom.Vec3
}

func rayBox(b geom.Box, o, d geom.Vec3, max float64) bool {
	lo, hi := 0., max
	for a := 0; a < 3; a++ {
		x, v := component(o, a), component(d, a)
		min, max := component(b.Min, a), component(b.Max, a)
		if v == 0 {
			if x < min || x > max {
				return false
			}
			continue
		}
		t0, t1 := (min-x)/v, (max-x)/v
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		lo = math.Max(lo, t0)
		hi = math.Min(hi, t1)
		if lo > hi {
			return false
		}
	}
	return true
}

// Ray returns the closest nonnegative ray hit, up to maxDistance (positive or
// +Inf). Coplanar rays have no isolated hit. Shared-edge ties choose a triangle
// deterministically, but this floating-point test is not an exact predicate.
func (q *Index) Ray(origin, direction geom.Vec3, maxDistance float64) (Hit, bool, error) {
	if q == nil || !origin.IsFinite() || math.IsNaN(maxDistance) || maxDistance <= 0 {
		return Hit{}, false, errors.New("query: invalid ray")
	}
	d, e := direction.Unit()
	if e != nil {
		return Hit{}, false, e
	}
	best := Hit{Distance: maxDistance, Triangle: -1}
	if len(q.nodes) == 0 {
		return best, false, nil
	}
	stack := []int{0}
	for len(stack) > 0 {
		n := q.nodes[stack[len(stack)-1]]
		stack = stack[:len(stack)-1]
		if !rayBox(n.box, origin, d, best.Distance) {
			continue
		}
		if n.left >= 0 {
			stack = append(stack, n.right, n.left)
			continue
		}
		for _, f := range q.order[n.start:n.end] {
			t := q.m.Triangles[f]
			a, b, c := q.m.Vertices[t[0]], q.m.Vertices[t[1]], q.m.Vertices[t[2]]
			e1, e2 := b.Sub(a), c.Sub(a)
			l1, l2 := maxAbs(e1), maxAbs(e2)
			eu1 := geom.Vec3{X: e1.X / l1, Y: e1.Y / l1, Z: e1.Z / l1}
			eu2 := geom.Vec3{X: e2.X / l2, Y: e2.Y / l2, Z: e2.Z / l2}
			delta := origin.Sub(a)
			scale := maxAbs(delta)
			if !eu1.IsFinite() || !eu2.IsFinite() || !finite(l1) || !finite(l2) || !finite(scale) {
				return Hit{}, false, errors.New("query: ray arithmetic overflow")
			}
			h := d.Cross(eu2)
			det := eu1.Dot(h)
			if det == 0 {
				continue
			}
			unitDelta := geom.Vec3{}
			if scale != 0 {
				unitDelta = geom.Vec3{X: delta.X / scale, Y: delta.Y / scale, Z: delta.Z / scale}
			}
			// Normalize each edge independently to keep triple products in
			// range across very large and very small triangle scales.
			u := (unitDelta.Dot(h) / det) * (scale / l1)
			if !finite(u) {
				return Hit{}, false, errors.New("query: ray barycentric overflow")
			}
			if u < 0 || u > 1 {
				continue
			}
			r := unitDelta.Cross(eu1)
			v := (d.Dot(r) / det) * (scale / l2)
			if !finite(v) {
				return Hit{}, false, errors.New("query: ray barycentric overflow")
			}
			if v < 0 || u+v > 1 {
				continue
			}
			ratio := eu2.Dot(r) / det
			dist := ratio * scale
			if !finite(dist) || (ratio != 0 && scale != 0 && dist == 0) {
				return Hit{}, false, errors.New("query: ray distance outside float64 range")
			}
			if dist >= 0 && (dist < best.Distance || (dist == best.Distance && (best.Triangle < 0 || f < best.Triangle))) {
				p := a.Scale(1 - u - v).Add(b.Scale(u)).Add(c.Scale(v))
				if !p.IsFinite() {
					return Hit{}, false, errors.New("query: ray arithmetic overflow")
				}
				best = Hit{p, dist, f, geom.Vec3{X: 1 - u - v, Y: u, Z: v}}
			}
		}
	}
	return best, best.Triangle >= 0, nil
}
func boxDistance(b geom.Box, p geom.Vec3) float64 {
	v := geom.Vec3{}
	for a := 0; a < 3; a++ {
		d := math.Max(component(b.Min, a)-component(p, a), math.Max(0, component(p, a)-component(b.Max, a)))
		switch a {
		case 0:
			v.X = d
		case 1:
			v.Y = d
		case 2:
			v.Z = d
		}
	}
	return v.Length()
}

// closestTriangle uses Voronoi regions; inputs must have a nonzero area.
func closestTriangle(p, a, b, c geom.Vec3) (geom.Vec3, geom.Vec3) {
	invalid := func() (geom.Vec3, geom.Vec3) { n := geom.Vec3{X: math.NaN()}; return n, n }
	ab, ac, ap := b.Sub(a), c.Sub(a), p.Sub(a)
	scale := math.Max(maxAbs(ab), maxAbs(ac))
	if !finite(scale) || scale == 0 || !ap.IsFinite() {
		return invalid()
	}
	divide := func(v geom.Vec3) geom.Vec3 { return geom.Vec3{X: v.X / scale, Y: v.Y / scale, Z: v.Z / scale} }
	nb, nc, np := divide(ab), divide(ac), divide(ap)
	// Voronoi tests use fourth-order products. Normalize first and reject
	// ill-conditioned aspect ratios or extreme relative point distances.
	if !np.IsFinite() || maxAbs(np) > 1e70 || nb.Cross(nc).Length() < 1e-14 {
		return invalid()
	}
	_, bary := closestTriangleNormalized(np, geom.Vec3{}, nb, nc)
	point := a.Scale(bary.X).Add(b.Scale(bary.Y)).Add(c.Scale(bary.Z))
	return point, bary
}

func closestTriangleNormalized(p, a, b, c geom.Vec3) (geom.Vec3, geom.Vec3) {
	ab, ac, ap := b.Sub(a), c.Sub(a), p.Sub(a)
	d1, d2 := ab.Dot(ap), ac.Dot(ap)
	if d1 <= 0 && d2 <= 0 {
		return a, geom.Vec3{X: 1, Y: 0, Z: 0}
	}
	bp := p.Sub(b)
	d3, d4 := ab.Dot(bp), ac.Dot(bp)
	if d3 >= 0 && d4 <= d3 {
		return b, geom.Vec3{X: 0, Y: 1, Z: 0}
	}
	vc := d1*d4 - d3*d2
	if vc <= 0 && d1 >= 0 && d3 <= 0 {
		v := d1 / (d1 - d3)
		return a.Add(ab.Scale(v)), geom.Vec3{X: 1 - v, Y: v, Z: 0}
	}
	cp := p.Sub(c)
	d5, d6 := ab.Dot(cp), ac.Dot(cp)
	if d6 >= 0 && d5 <= d6 {
		return c, geom.Vec3{X: 0, Y: 0, Z: 1}
	}
	vb := d5*d2 - d1*d6
	if vb <= 0 && d2 >= 0 && d6 <= 0 {
		w := d2 / (d2 - d6)
		return a.Add(ac.Scale(w)), geom.Vec3{X: 1 - w, Y: 0, Z: w}
	}
	va := d3*d6 - d5*d4
	if va <= 0 && (d4-d3) >= 0 && (d5-d6) >= 0 {
		w := (d4 - d3) / ((d4 - d3) + (d5 - d6))
		return b.Add(c.Sub(b).Scale(w)), geom.Vec3{X: 0, Y: 1 - w, Z: w}
	}
	den := 1 / (va + vb + vc)
	v, w := vb*den, vc*den
	return a.Add(ab.Scale(v)).Add(ac.Scale(w)), geom.Vec3{X: 1 - v - w, Y: v, Z: w}
}

// Closest returns the closest surface point, or false for an empty mesh.
// Ill-conditioned triangles (normalized area below 1e-14) and extreme relative
// distances may return an arithmetic error rather than an unreliable answer.
func (q *Index) Closest(p geom.Vec3) (Hit, bool, error) {
	if q == nil || !p.IsFinite() {
		return Hit{}, false, errors.New("query: invalid point")
	}
	best := Hit{Distance: math.Inf(1), Triangle: -1}
	if len(q.nodes) == 0 {
		return best, false, nil
	}
	stack := []int{0}
	for len(stack) > 0 {
		n := q.nodes[stack[len(stack)-1]]
		stack = stack[:len(stack)-1]
		if boxDistance(n.box, p) > best.Distance {
			continue
		}
		if n.left >= 0 {
			stack = append(stack, n.right, n.left)
			continue
		}
		for _, f := range q.order[n.start:n.end] {
			t := q.m.Triangles[f]
			a, b, c := q.m.Vertices[t[0]], q.m.Vertices[t[1]], q.m.Vertices[t[2]]
			v, bary := closestTriangle(p, a, b, c)
			dist := v.Sub(p).Length()
			if !v.IsFinite() || !bary.IsFinite() || !finite(dist) {
				return Hit{}, false, errors.New("query: closest-point arithmetic overflow")
			}
			if dist < best.Distance || (dist == best.Distance && (best.Triangle < 0 || f < best.Triangle)) {
				best = Hit{v, dist, f, bary}
			}
		}
	}
	return best, best.Triangle >= 0, nil
}

type Location uint8

const (
	Outside Location = iota
	Boundary
	Inside
)

// Classify uses boundary distance followed by the oriented solid angle winding
// number. tolerance is an absolute nonnegative length. Topological closure is
// checked; self-intersection and cavity orientation are caller preconditions.
func (q *Index) Classify(p geom.Vec3, tolerance float64) (Location, error) {
	if q == nil || !p.IsFinite() || !finite(tolerance) || tolerance < 0 {
		return Outside, errors.New("query: invalid classification arguments")
	}
	if e := q.m.ValidateSolid(); e != nil {
		return Outside, e
	}
	h, ok, e := q.Closest(p)
	if e != nil {
		return Outside, e
	}
	if !ok {
		return Outside, nil
	}
	if h.Distance <= tolerance {
		return Boundary, nil
	}
	sum := 0.
	for _, t := range q.m.Triangles {
		a, e := q.m.Vertices[t[0]].Sub(p).Unit()
		if e != nil {
			return Outside, e
		}
		b, e := q.m.Vertices[t[1]].Sub(p).Unit()
		if e != nil {
			return Outside, e
		}
		c, e := q.m.Vertices[t[2]].Sub(p).Unit()
		if e != nil {
			return Outside, e
		}
		sum += 2 * math.Atan2(a.Dot(b.Cross(c)), 1+a.Dot(b)+b.Dot(c)+c.Dot(a))
	}
	if math.Abs(sum) > 2*math.Pi {
		return Inside, nil
	}
	return Outside, nil
}

func maxAbs(v geom.Vec3) float64 {
	return math.Max(math.Abs(v.X), math.Max(math.Abs(v.Y), math.Abs(v.Z)))
}
