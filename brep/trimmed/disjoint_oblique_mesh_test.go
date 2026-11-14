package trimmed

import (
	"context"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestDisjointObliqueMesh(t *testing.T) {
	b, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 20, Y: 20, Z: 20})
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range []geom.Vec3{{X: 1, Z: 4}} {
		cuts := []CylinderCut{{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 2, Y: 4}, Direction: d, Radius: 1}, {Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 16, Y: 16}, Direction: geom.Vec3{Z: 1}, Radius: 1}}
		s, h, e := BoreCylinders(b, cuts, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		if len(h) != 2 || h[0].Entry != 0 || h[1].Entry != 0 {
			t.Fatal(h)
		}
		m, e := s.TessellateDisjointObliqueBores(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 1000000})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		for i, p := range m.Vertices {
			m.Vertices[i] = geom.Vec3{X: float64(float32(p.X)), Y: float64(float32(p.Y)), Z: float64(float32(p.Z))}
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		t.Log(len(m.Triangles))
	}
}
