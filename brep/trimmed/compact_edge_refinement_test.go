package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCompactEdgesRefineIndependentRadii(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	s, e := ExtrudeDiskWithHoles(context.Background(), f, ExtrusionCircle{Radius: 100}, []ExtrusionCircle{{Radius: 1}}, nil, 10)
	if e != nil {
		t.Fatal(e)
	}
	m, ok, e := s.compactCylinderMesh(context.Background(), parametric.TessellationOptions{Deflection: .01, MaxSubdivisions: 128, MaxTriangles: 10000})
	if !ok || e != nil {
		t.Fatal(ok, e)
	}
	outer, inner := 0, 0
	for _, p := range m.Vertices {
		if p.Z != 0 {
			continue
		}
		r := math.Hypot(p.X, p.Y)
		if math.Abs(r-100) < 1e-9 {
			outer++
		}
		if math.Abs(r-1) < 1e-9 {
			inner++
		}
	}
	if outer < 128 || inner > 64 || outer < 4*inner {
		t.Fatal("unrelated ring resolution coupled", outer, inner)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
}
