package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"plainsolid/mesh"
	"testing"
)

func TestSphereDisjointFiniteCylinderUnion(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	// Diagonal case has radial gap4<5 and axial gap4<5, but total sqrt32>5.
	for _, tc := range []struct{ d, lo, hi float64 }{{7, -1, 1}, {0, 6, 8}, {5, 4, 6}, {5, -6, -4}} {
		r, e := SphereDisjointFiniteCylinderUnion(ctx, center, 5, 1, geom.Vec2{X: tc.d}, tc.lo, tc.hi, 1e-9)
		if e != nil {
			t.Fatal(tc, e)
		}
		total := 0.0
		var combined mesh.Mesh
		for i, s := range r.Solids {
			var b bytes.Buffer
			if e = WriteJSON(&b, s); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadJSON(&b, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			p, e := loaded.MassProperties(ctx, IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			coef, _ := r.VolumePiCoefficients[i].Float64()
			if math.Abs(p.Volume-math.Pi*coef) > 1e-7 {
				t.Fatal(p, coef)
			}
			total += p.Volume
			want := center
			if i == 1 {
				want = want.Add(geom.Vec3{X: tc.d, Z: (tc.lo + tc.hi) / 2})
			}
			if p.Centroid.Sub(want).Length() > 1e-8 {
				t.Fatal(p.Centroid, want)
			}
			if tc.d == 5 && tc.lo == 4 {
				m, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .3, MaxSubdivisions: 128, MaxTriangles: 100000})
				if e != nil {
					t.Fatal(e)
				}
				base := len(combined.Vertices)
				combined.Vertices = append(combined.Vertices, m.Vertices...)
				for _, tr := range m.Triangles {
					combined.Triangles = append(combined.Triangles, [3]int{tr[0] + base, tr[1] + base, tr[2] + base})
				}
			}
		}
		coef, _ := r.TotalVolumePiCoefficient.Float64()
		if math.Abs(total-math.Pi*coef) > 1e-7 {
			t.Fatal(total, coef)
		}
		if len(combined.Triangles) > 0 {
			if e = combined.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			report, e := collision.SelfIntersectionsBVHContext(ctx, combined, collision.Options{})
			if e != nil || !report.Complete || len(report.Contacts) != 0 {
				t.Fatal(e, len(report.Contacts))
			}
		}
	}
}
func TestSphereDisjointFiniteCylinderUnionRejects(t *testing.T) {
	// Radial, axial, and diagonal exact tangencies all fail closed.
	for _, tc := range []struct{ d, lo, hi float64 }{{6, -1, 1}, {0, 5, 7}, {4, 4, 6}, {5, 0, 2}} {
		r, e := SphereDisjointFiniteCylinderUnion(context.Background(), geom.Vec3{}, 5, 1, geom.Vec2{X: tc.d}, tc.lo, tc.hi, 1e-9)
		if e == nil || len(r.Solids[0].Faces())+len(r.Solids[1].Faces()) != 0 {
			t.Fatal(tc, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := SphereDisjointFiniteCylinderUnion(ctx, geom.Vec3{}, 5, 1, geom.Vec2{X: 7}, -1, 1, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
	if _, e := SphereDisjointFiniteCylinderUnion(context.Background(), geom.Vec3{X: .1}, 5, 1, geom.Vec2{X: 7.2}, -1, 1, 1e-9); e == nil {
		t.Fatal("rounded translation")
	}
}
