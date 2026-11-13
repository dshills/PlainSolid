package mesh

import (
	"fmt"
	"math"
	"plainsolid/geom"
)

// Box constructs an outward-oriented box with strictly positive side lengths.
func Box(min, max geom.Vec3) (Mesh, error) {
	if !min.IsFinite() || !max.IsFinite() || min.X >= max.X || min.Y >= max.Y || min.Z >= max.Z {
		return Mesh{}, fmt.Errorf("mesh: invalid box bounds")
	}
	m := Mesh{Vertices: []geom.Vec3{{X: min.X, Y: min.Y, Z: min.Z}, {X: max.X, Y: min.Y, Z: min.Z}, {X: max.X, Y: max.Y, Z: min.Z}, {X: min.X, Y: max.Y, Z: min.Z}, {X: min.X, Y: min.Y, Z: max.Z}, {X: max.X, Y: min.Y, Z: max.Z}, {X: max.X, Y: max.Y, Z: max.Z}, {X: min.X, Y: max.Y, Z: max.Z}}, Triangles: [][3]int{{0, 2, 1}, {0, 3, 2}, {4, 5, 6}, {4, 6, 7}, {0, 1, 5}, {0, 5, 4}, {1, 2, 6}, {1, 6, 5}, {2, 3, 7}, {2, 7, 6}, {3, 0, 4}, {3, 4, 7}}}
	return m, m.Validate()
}

// Sphere constructs an outward latitude-longitude mesh. Rings is the number
// of latitude intervals (at least 2), segments the longitude divisions (at least 3).
// The generated vertices lie on the sphere; faces lie inside it.
func Sphere(center geom.Vec3, radius float64, segments, rings int) (Mesh, error) {
	if !center.IsFinite() || !finite(radius) || radius <= 0 || segments < 3 || rings < 2 || segments > 100000 || rings > 100000 || int64(segments)*int64(rings) > 2000000 {
		return Mesh{}, fmt.Errorf("mesh: invalid sphere parameters or resolution exceeds limit")
	}
	m := Mesh{Vertices: []geom.Vec3{center.Add(geom.Vec3{Z: radius})}}
	for j := 1; j < rings; j++ {
		theta := math.Pi * float64(j) / float64(rings)
		for i := 0; i < segments; i++ {
			phi := 2 * math.Pi * float64(i) / float64(segments)
			m.Vertices = append(m.Vertices, center.Add(geom.Vec3{X: radius * math.Sin(theta) * math.Cos(phi), Y: radius * math.Sin(theta) * math.Sin(phi), Z: radius * math.Cos(theta)}))
		}
	}
	south := len(m.Vertices)
	m.Vertices = append(m.Vertices, center.Add(geom.Vec3{Z: -radius}))
	for i := 0; i < segments; i++ {
		next := (i + 1) % segments
		m.Triangles = append(m.Triangles, [3]int{0, 1 + i, 1 + next})
		for j := 0; j < rings-2; j++ {
			a := 1 + j*segments + i
			b := 1 + j*segments + next
			c := a + segments
			d := b + segments
			m.Triangles = append(m.Triangles, [3]int{a, c, b}, [3]int{b, c, d})
		}
		a := 1 + (rings-2)*segments + i
		b := 1 + (rings-2)*segments + next
		m.Triangles = append(m.Triangles, [3]int{a, south, b})
	}
	return m, m.Validate()
}

// Cylinder constructs a capped cylinder aligned with +Z. Base is the bottom
// center and height and radius must be positive. Segments controls approximation.
func Cylinder(base geom.Vec3, radius, height float64, segments int) (Mesh, error) {
	if !base.IsFinite() || !finite(radius) || !finite(height) || radius <= 0 || height <= 0 || segments < 3 || segments > 1000000 {
		return Mesh{}, fmt.Errorf("mesh: invalid cylinder parameters or resolution exceeds limit")
	}
	p := make([]geom.Vec3, segments)
	for i := range p {
		a := 2 * math.Pi * float64(i) / float64(segments)
		p[i] = base.Add(geom.Vec3{X: radius * math.Cos(a), Y: radius * math.Sin(a)})
	}
	// Convex fan avoids the quadratic general polygon triangulator.
	m := Mesh{Vertices: make([]geom.Vec3, 2*segments+2)}
	copy(m.Vertices, p)
	for i, v := range p {
		m.Vertices[i+segments] = v.Add(geom.Vec3{Z: height})
	}
	bc, tc := 2*segments, 2*segments+1
	m.Vertices[bc] = base
	m.Vertices[tc] = base.Add(geom.Vec3{Z: height})
	for i := 0; i < segments; i++ {
		j := (i + 1) % segments
		m.Triangles = append(m.Triangles, [3]int{bc, j, i}, [3]int{tc, i + segments, j + segments}, [3]int{i, j, j + segments}, [3]int{i, j + segments, i + segments})
	}
	return m, m.Validate()
}
