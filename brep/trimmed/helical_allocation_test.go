package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func allocationThreadFixture(t testing.TB) Solid {
	t.Helper()
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	a, c := .5/16+.25/math.Sqrt(3), .5/16
	center := .275
	p := []geom.Vec2{{X: 1.25, Y: center - a}, {X: 1.5, Y: center - c}, {X: 1.5, Y: center + c}, {X: 1.25, Y: center + a}}
	r, e := GrooveWithOverlappingEntryRelief(context.Background(), HelicalGrooveRecipe{Frame: f, Height: 4, BoreRadius: 1.25, OuterRadius: 3, Profile: p, Lead: .5, End: 6, Hand: geom.RightHanded, Starts: 1}, 1.51, .37, 4e-12)
	if e != nil {
		t.Fatal(e)
	}
	s, e := r.Solid.transformed(context.Background(), geom.Affine{{1, 0, 0, 10}, {0, 1, 0, 0}, {0, 0, 1, 0}}, false)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestHelicalCheckAllocationExactMesh(t *testing.T) {
	s := allocationThreadFixture(t)
	for _, o := range []parametric.TessellationOptions{{Deflection: .5, Angle: .1, MaxSubdivisions: 256, MaxTriangles: 100000}, {Deflection: .5, Angle: .1, MaxSubdivisions: 256, MaxTriangles: 10}} {
		a, ea := s.tessellateHelicalAllocation(context.Background(), o, false)
		b, eb := s.tessellateHelicalAllocation(context.Background(), o, true)
		if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(ea, eb) {
			t.Fatal("allocation changed mesh or failure", ea, eb)
		}
	}
}
func BenchmarkHelicalCheckAllocation(b *testing.B) {
	s := allocationThreadFixture(b)
	o := parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 256, MaxTriangles: 100000}
	for _, c := range []struct {
		name    string
		enabled bool
	}{{"baseline", false}, {"preallocated", true}} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, e := s.tessellateHelicalAllocation(context.Background(), o, c.enabled); e != nil {
					b.Fatal(e)
				}
			}
		})
	}
}
