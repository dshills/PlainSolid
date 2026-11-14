package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestDisjointObliqueMesh(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 20, Y: 20, Z: 20})
	if e != nil {
		t.Fatal(e)
	}
	opts := parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 400}
	for _, oblique := range []CylinderCut{
		{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 2, Y: 4}, Direction: geom.Vec3{X: 1, Z: 4}, Radius: 1},
		{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 3, Y: 4}, Direction: geom.Vec3{X: 2, Z: 3}, Radius: 1},
		{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 3, Y: 4}, Direction: geom.Vec3{X: 1, Y: 1, Z: 4}, Radius: 1},
		{Entry: 1, Exit: 0, AxisPoint: geom.Vec3{X: 9, Y: 3, Z: 20}, Direction: geom.Vec3{X: -1, Y: 1, Z: -4}, Radius: 1},
	} {
		cuts := []CylinderCut{oblique, {Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 16, Y: 16}, Direction: geom.Vec3{Z: 1}, Radius: 1}}
		s, h, e := BoreCylinders(b, cuts, 1e-9)
		if e != nil {
			t.Fatal(oblique, e)
		}
		if len(h) != 2 || h[0].Entry != oblique.Entry || h[1].Entry != 0 {
			t.Fatal(h)
		}
		m, normals, handled, e := s.TessellateCompactNormals(context.Background(), opts)
		if !handled || e != nil || len(normals) != len(m.Triangles) {
			t.Fatal(oblique, handled, e)
		}
		m, e = s.TessellateDisjointObliqueBores(context.Background(), opts)
		if e != nil {
			t.Fatal(oblique, e)
		}
		tight := opts
		tight.MaxTriangles = 2
		if _, e = s.TessellateDisjointObliqueBores(context.Background(), tight); !errors.Is(e, parametric.ErrBudget) {
			t.Fatal("triangle budget", e)
		}
		for i, p := range m.Vertices {
			m.Vertices[i] = geom.Vec3{X: float64(float32(p.X)), Y: float64(float32(p.Y)), Z: float64(float32(p.Z))}
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		t.Log(oblique.Direction, len(m.Triangles))
	}
}
