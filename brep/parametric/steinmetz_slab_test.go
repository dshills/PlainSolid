package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSteinmetzSlab(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	cdf := func(x float64) float64 {
		a := math.Sqrt(25 - x*x)
		tail := 2*a*a*a/3 + 50*(a-math.Abs(x)*math.Acos(math.Abs(x)/5))
		if x < 0 {
			return tail
		}
		return 2000.0/3 - tail
	}
	for _, ab := range [][2]float64{{-4, -3}, {-2, 3}, {-1, 1}, {0, 2}} {
		r, e := PerpendicularCylinderSlab(ctx, center, 5, ab[0], ab[1], 6, 1e-8, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(ab, e)
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		r.Solid = loaded
		m, e := r.Solid.MassProperties(ctx, IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := cdf(ab[1]) - cdf(ab[0])
		if math.Abs(m.Volume-want) > 1e-6 {
			t.Fatal(ab, m.Volume, want)
		}
		lo, _ := r.IdealVolumeBounds[0].Float64()
		hi, _ := r.IdealVolumeBounds[1].Float64()
		if lo > want+1e-9 || hi < want-1e-9 || hi-lo > 1e-5 {
			t.Fatal("volume interval", lo, hi, want)
		}
		if _, e := loaded.Bounds(); e != nil {
			t.Fatal(e)
		}
		mesh, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .3, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		for _, p := range mesh.Vertices {
			if p.X < center.X+ab[0]-1e-8 || p.X > center.X+ab[1]+1e-8 {
				t.Fatal("wrong slab", p)
			}
		}
		report, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal(e, len(report.Contacts))
		}
		seen := make(map[int]bool)
		for _, group := range [][]int{r.XAxisFaces, r.ZAxisFaces, r.LowerCapFaces, r.UpperCapFaces} {
			for _, id := range group {
				if seen[id] {
					t.Fatal("duplicate face")
				}
				seen[id] = true
			}
		}
		if len(seen) != len(r.Solid.Faces()) || len(r.LowerCapFaces) == 0 || len(r.UpperCapFaces) == 0 {
			t.Fatal("ancestry")
		}
	}
}
func TestSteinmetzSlabLimits(t *testing.T) {
	for _, ab := range [][2]float64{{-5, 1}, {-1, 5}, {2, 2}, {3, 2}} {
		r, e := PerpendicularCylinderSlab(context.Background(), geom.Vec3{}, 5, ab[0], ab[1], 6, 1e-8, geom.ExactCurvePlaneEventOptions{})
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal(ab, e)
		}
	}
	r, e := PerpendicularCylinderSlab(context.Background(), geom.Vec3{}, 5, -1, 2, 6, 1e-8, geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal(r, e)
	}
}
