package plaincadbridge

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/mesh"
	"testing"
)

func TestHelicalCacheHonorsOptions(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	s, e := trimmed.GrooveCylinderWithHelicalStarts(context.Background(), f, 2, 3, []geom.Vec2{{X: 1.5, Y: .5}, {X: 2, Y: .375}, {X: 2, Y: .625}}, 1, .125, 1.125, geom.RightHanded, 1)
	if e != nil {
		t.Fatal(e)
	}
	cached := analyticBody{solid: s, helical: true, helicalMesh: &mesh.Mesh{Vertices: []geom.Vec3{{}}, Triangles: [][3]int{{0, 0, 0}}}}
	plain := cached
	plain.helicalMesh = nil
	for _, o := range []parametric.TessellationOptions{{Deflection: .5, Angle: .1, MaxSubdivisions: 256, MaxTriangles: 10, AnalyticBound: true}, {Deflection: .5, Angle: .1, MaxSubdivisions: 1, MaxTriangles: 10}, {Deflection: math.Inf(1), Angle: .1, MaxSubdivisions: 256, MaxTriangles: 10}, {Deflection: .5, Angle: 4, MaxSubdivisions: 256, MaxTriangles: 10}, {Deflection: .5, Angle: .1, MaxSubdivisions: 2048, MaxTriangles: 10}} {
		m, got := cached.tessellate(o)
		_, want := plain.tessellate(o)
		if got == nil || want == nil || !errors.Is(got, want) || len(m.Triangles) != 0 {
			t.Fatal(o, got, want, m)
		}
	}
	m, e := cached.tessellate(parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 256, MaxTriangles: 10})
	if e != nil || len(m.Triangles) != 1 {
		t.Fatal("valid reuse", e)
	}
}
