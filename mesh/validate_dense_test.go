package mesh

import (
	"math/rand"
	"plainsolid/geom"
	"testing"
)

func TestDenseValidationReference(t *testing.T) {
	base, e := Sphere(geom.Vec3{}, 1, 24, 12)
	if e != nil {
		t.Fatal(e)
	}
	check := func(m Mesh) {
		t.Helper()
		a, b := referenceValidateSolid(m), m.ValidateSolid()
		if (a == nil) != (b == nil) {
			t.Fatal("validation mismatch", a, b)
		}
	}
	check(base)
	check(Mesh{})
	rng := rand.New(rand.NewSource(19))
	for n := 0; n < 300; n++ {
		m := Mesh{Vertices: append([]geom.Vec3(nil), base.Vertices...), Triangles: append([][3]int(nil), base.Triangles...)}
		i := rng.Intn(len(m.Triangles))
		switch n % 5 {
		case 0:
			m.Triangles[i][0], m.Triangles[i][1] = m.Triangles[i][1], m.Triangles[i][0]
		case 1:
			m.Triangles = append(m.Triangles[:i], m.Triangles[i+1:]...)
		case 2:
			m.Triangles = append(m.Triangles, m.Triangles[i])
		case 3:
			m.Triangles[i][rng.Intn(3)] = rng.Intn(len(m.Vertices))
		case 4:
			for j := range m.Triangles {
				m.Triangles[j][1], m.Triangles[j][2] = m.Triangles[j][2], m.Triangles[j][1]
			}
		}
		check(m)
	}
	// Two closed spheres sharing exactly one indexed pole have disconnected fans.
	joined := Mesh{Vertices: append([]geom.Vec3(nil), base.Vertices...), Triangles: append([][3]int(nil), base.Triangles...)}
	offset := len(joined.Vertices) - 1
	joined.Vertices = append(joined.Vertices, base.Vertices[1:]...)
	for _, tri := range base.Triangles {
		for i, id := range tri {
			if id != 0 {
				tri[i] = id + offset
			}
		}
		joined.Triangles = append(joined.Triangles, tri)
	}
	check(joined)
	if joined.ValidateSolid() == nil {
		t.Fatal("pinched fan accepted")
	}
}
func BenchmarkSolidValidationDense(b *testing.B) {
	m, e := Sphere(geom.Vec3{}, 1, 128, 64)
	if e != nil {
		b.Fatal(e)
	}
	for _, c := range []struct {
		name string
		run  func(Mesh) error
	}{{"reference", referenceValidateSolid}, {"dense", func(m Mesh) error { return m.ValidateSolid() }}} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if e := c.run(m); e != nil {
					b.Fatal(e)
				}
			}
		})
	}
}
