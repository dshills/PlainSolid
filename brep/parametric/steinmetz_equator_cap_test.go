package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSteinmetzEquatorHalf(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	for _, positive := range []bool{false, true} {
		r, e := PerpendicularCylinderEquatorHalf(ctx, center, 2, 3, 4, 1e-9, positive)
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Solid.Faces()) != 4 || len(r.Solid.Vertices()) != 2 || r.CapFace != 3 {
			t.Fatal(r)
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&b, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		mass, e := loaded.MassProperties(ctx, IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want, _ := r.IdealVolume.Float64()
		shift := -9 * math.Pi * 2 / 64
		if positive {
			shift = -shift
		}
		if math.Abs(mass.Volume-want) > 1e-7 || math.Abs(mass.Area-(8+math.Pi)*4) > 1e-7 || mass.Centroid.Sub(center.Add(geom.Vec3{X: shift})).Length() > 1e-8 {
			t.Fatal(mass, want, shift)
		}
		bounds, e := loaded.Bounds()
		if e != nil || bounds.Min.Y > center.Y-2 || bounds.Max.Y < center.Y+2 {
			t.Fatal(bounds, e)
		}
		m, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		for _, p := range m.Vertices {
			if !positive && p.X > center.X || positive && p.X < center.X {
				t.Fatal("wrong half", p)
			}
		}
		report, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal(e, len(report.Contacts))
		}
	}
}
func TestSteinmetzEquatorHalfLimits(t *testing.T) {
	r, e := PerpendicularCylinderEquatorHalf(context.Background(), geom.Vec3{}, 2, 2, 4, 1e-9, false)
	if e == nil || len(r.Solid.Faces()) != 0 {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := PerpendicularCylinderEquatorHalf(ctx, geom.Vec3{}, 2, 3, 4, 1e-9, true); e != context.Canceled {
		t.Fatal(e)
	}
	raw, e := EncodeSurface(steinmetzDisk{radius: 2})
	if e != nil {
		t.Fatal(e)
	}
	raw = bytes.Replace(raw, []byte(`"A":2`), []byte(`"A":-2`), 1)
	if _, e = DecodeSurface(raw, 100); e == nil {
		t.Fatal("negative disk radius")
	}
}
