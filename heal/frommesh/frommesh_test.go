package frommesh

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/heal/unify"
	"plainsolid/mesh"
	"testing"
)

func boxMesh(t *testing.T) mesh.Mesh {
	t.Helper()
	m, e := mesh.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestBoxConversionAncestry(t *testing.T) {
	m := boxMesh(t)
	m.Vertices = append(m.Vertices, geom.Vec3{X: 99})
	r, e := Convert(context.Background(), m, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Solid.Faces()) != 6 || len(r.Solid.Edges()) != 12 || r.VertexMap[len(m.Vertices)-1] != -1 {
		t.Fatal("topology")
	}
	seen := map[int]bool{}
	for f, triangles := range r.FaceTriangles {
		if len(triangles) != 2 {
			t.Fatal(triangles)
		}
		for _, tr := range triangles {
			if r.TriangleFaces[tr] != f || seen[tr] {
				t.Fatal("ancestry")
			}
			seen[tr] = true
		}
	}
	p, e := r.Solid.MassProperties()
	if e != nil || math.Abs(p.Volume-24) > 1e-12 {
		t.Fatal(e, p)
	}
}
func TestHoleAndConcaveConversion(t *testing.T) {
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	hole := []geom.Vec3{{X: 1, Y: 1}, {X: 1, Y: 3}, {X: 3, Y: 3}, {X: 3, Y: 1}}
	s, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	r, e := Convert(context.Background(), s.Tessellate(), Options{})
	if e != nil {
		t.Fatal(e)
	}
	holes := 0
	for _, f := range r.Solid.Faces() {
		holes += len(f.Holes)
	}
	if holes != 2 || r.Solid.Topology().EulerCharacteristic != 0 {
		t.Fatal("lost holes")
	}
	s, e = brep.Prism([]geom.Vec3{{}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	r, e = Convert(context.Background(), s.Tessellate(), Options{})
	if e != nil || len(r.Solid.Faces()) != 8 {
		t.Fatal(e)
	}
}
func TestConversionFailureAndEmpty(t *testing.T) {
	m := boxMesh(t)
	open := mesh.Mesh{Vertices: m.Vertices, Triangles: m.Triangles[:len(m.Triangles)-1]}
	if _, e := Convert(context.Background(), open, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	inward := m
	inward.Triangles = append([][3]int(nil), m.Triangles...)
	for i := range inward.Triangles {
		inward.Triangles[i][0], inward.Triangles[i][1] = inward.Triangles[i][1], inward.Triangles[i][0]
	}
	if _, e := Convert(context.Background(), inward, Options{}); e == nil {
		t.Fatal("accepted inward shell")
	}
	if _, e := Convert(context.Background(), m, Options{MaxTriangles: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if _, e := Convert(context.Background(), m, Options{Unify: unify.Options{MaxWork: 1}}); !errors.Is(e, unify.ErrLimit) {
		t.Fatal(e)
	}
	r, e := Convert(context.Background(), mesh.Mesh{}, Options{})
	if e != nil || len(r.Solid.Faces()) != 0 {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Convert(ctx, m, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestNoApproximateWeldingOrPlanarity(t *testing.T) {
	m := boxMesh(t)
	m.Vertices[0].X += math.Ldexp(1, -20)
	r, e := Convert(context.Background(), m, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Solid.Faces()) <= 6 {
		t.Fatal("nearly planar triangles were merged")
	}
	m = boxMesh(t)
	n := len(m.Vertices)
	vs := append(append([]geom.Vec3(nil), m.Vertices...), m.Vertices...)
	ts := append([][3]int(nil), m.Triangles...)
	for _, tr := range m.Triangles {
		ts = append(ts, [3]int{tr[0] + n, tr[1] + n, tr[2] + n})
	}
	if _, e = Convert(context.Background(), mesh.Mesh{Vertices: vs, Triangles: ts}, Options{}); e == nil {
		t.Fatal("coincident disconnected shells accepted")
	}
}
