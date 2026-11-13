package mesh

import (
	"fmt"
	"math"
	"plainsolid/geom"
)

type point2 struct{ x, y float64 }

func orient(a, b, c point2) float64 { return (b.x-a.x)*(c.y-a.y) - (b.y-a.y)*(c.x-a.x) }

const polygonTolerance = 1e-12

// Extrude constructs a solid from a simple planar polygon and an offset vector.
// The polygon excludes a repeated closing vertex; holes and collinear consecutive
// vertices are unsupported. Either winding is accepted. Predicates use a relative
// tolerance of 1e-12 of the polygon's scale; near-degenerate inputs are rejected.
// Offset may be oblique but must not be parallel to the polygon plane.
func Extrude(polygon []geom.Vec3, offset geom.Vec3) (Mesh, error) {
	n := len(polygon)
	if n < 3 || n > 10000 || !offset.IsFinite() || offset.Length() == 0 {
		return Mesh{}, fmt.Errorf("mesh: invalid extrusion parameters")
	}
	p := append([]geom.Vec3(nil), polygon...)
	scale := 0.0
	for _, v := range p {
		if !v.IsFinite() {
			return Mesh{}, fmt.Errorf("mesh: nonfinite polygon")
		}
		d := v.Sub(p[0])
		scale = math.Max(scale, math.Max(math.Abs(d.X), math.Max(math.Abs(d.Y), math.Abs(d.Z))))
	}
	if !finite(scale) || scale == 0 {
		return Mesh{}, fmt.Errorf("mesh: degenerate polygon")
	}
	q := make([]geom.Vec3, n)
	for i, v := range p {
		q[i] = v.Sub(p[0]).Scale(1 / scale)
	}
	normal := geom.Vec3{}
	for i := range q {
		normal = normal.Add(q[i].Cross(q[(i+1)%n]))
	}
	length := normal.Length()
	if !finite(length) || length < polygonTolerance {
		return Mesh{}, fmt.Errorf("mesh: degenerate polygon normal")
	}
	normal = normal.Scale(1 / length)
	for _, v := range q {
		if math.Abs(v.Dot(normal)) > polygonTolerance {
			return Mesh{}, fmt.Errorf("mesh: polygon is not planar")
		}
	}
	if math.Abs(offset.Dot(normal)) <= polygonTolerance*offset.Length() {
		return Mesh{}, fmt.Errorf("mesh: extrusion offset is parallel to polygon")
	}
	if offset.Dot(normal) < 0 {
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			p[i], p[j] = p[j], p[i]
			q[i], q[j] = q[j], q[i]
		}
	}
	// Drop the dominant normal coordinate to preserve a well-conditioned projection.
	axis := 0
	if math.Abs(normal.Y) > math.Abs(normal.X) {
		axis = 1
	}
	if math.Abs(normal.Z) > math.Abs([]float64{normal.X, normal.Y}[axis]) {
		axis = 2
	}
	pts := make([]point2, n)
	for i, v := range q {
		switch axis {
		case 0:
			pts[i] = point2{v.Y, v.Z}
		case 1:
			pts[i] = point2{v.X, v.Z}
		case 2:
			pts[i] = point2{v.X, v.Y}
		}
	}
	area := 0.0
	for i, a := range pts {
		b := pts[(i+1)%n]
		area += a.x*b.y - a.y*b.x
		if math.Abs(orient(pts[(i+n-1)%n], a, b)) <= polygonTolerance {
			return Mesh{}, fmt.Errorf("mesh: collinear or repeated polygon vertex")
		}
	}
	sign := 1.0
	if area < 0 {
		sign = -1
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if j == i+1 || (i == 0 && j == n-1) {
				continue
			}
			if segmentsIntersect(pts[i], pts[(i+1)%n], pts[j], pts[(j+1)%n]) {
				return Mesh{}, fmt.Errorf("mesh: polygon self-intersects")
			}
		}
	}
	ids := make([]int, n)
	for i := range ids {
		ids[i] = i
	}
	caps := make([][3]int, 0, n-2)
	for len(ids) > 3 {
		found := false
		for k, b := range ids {
			a, c := ids[(k+len(ids)-1)%len(ids)], ids[(k+1)%len(ids)]
			if sign*orient(pts[a], pts[b], pts[c]) <= polygonTolerance {
				continue
			}
			inside := false
			for _, v := range ids {
				if v == a || v == b || v == c {
					continue
				}
				if sign*orient(pts[a], pts[b], pts[v]) >= -polygonTolerance && sign*orient(pts[b], pts[c], pts[v]) >= -polygonTolerance && sign*orient(pts[c], pts[a], pts[v]) >= -polygonTolerance {
					inside = true
					break
				}
			}
			if inside {
				continue
			}
			caps = append(caps, [3]int{a, b, c})
			ids = append(ids[:k], ids[k+1:]...)
			found = true
			break
		}
		if !found {
			return Mesh{}, fmt.Errorf("mesh: polygon cannot be robustly triangulated")
		}
	}
	caps = append(caps, [3]int{ids[0], ids[1], ids[2]})
	m := Mesh{Vertices: make([]geom.Vec3, 2*n)}
	copy(m.Vertices, p)
	for i, v := range p {
		m.Vertices[n+i] = v.Add(offset)
	}
	for _, t := range caps {
		m.Triangles = append(m.Triangles, [3]int{t[0], t[2], t[1]}, [3]int{t[0] + n, t[1] + n, t[2] + n})
	}
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		m.Triangles = append(m.Triangles, [3]int{i, j, j + n}, [3]int{i, j + n, i + n})
	}
	return m, m.ValidateSolid()
}
func segmentsIntersect(a, b, c, d point2) bool {
	x, y, z, w := orient(a, b, c), orient(a, b, d), orient(c, d, a), orient(c, d, b)
	if ((x > polygonTolerance && y < -polygonTolerance) || (x < -polygonTolerance && y > polygonTolerance)) && ((z > polygonTolerance && w < -polygonTolerance) || (z < -polygonTolerance && w > polygonTolerance)) {
		return true
	}
	on := func(p, a, b point2) bool {
		return p.x >= math.Min(a.x, b.x)-polygonTolerance && p.x <= math.Max(a.x, b.x)+polygonTolerance && p.y >= math.Min(a.y, b.y)-polygonTolerance && p.y <= math.Max(a.y, b.y)+polygonTolerance
	}
	return math.Abs(x) <= polygonTolerance && on(c, a, b) || math.Abs(y) <= polygonTolerance && on(d, a, b) || math.Abs(z) <= polygonTolerance && on(a, c, d) || math.Abs(w) <= polygonTolerance && on(b, c, d)
}
