package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestPerpendicularCylinderDifference(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	result, e := PerpendicularCylinderDifference(ctx, center, 2, -3, 4, 5, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	total := 0.0
	for i, s := range result.Solids {
		if len(s.Faces()) != 4 || len(s.Vertices()) != 4 {
			t.Fatal("topology", len(s.Faces()), len(s.Vertices()))
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, s); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := loaded.MassProperties(ctx, IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		L := float64(3 + i)
		volume := math.Pi*4*L - 64.0/3
		area := math.Pi * (4*L + 4)
		shift := math.Pi * (2*L*L - 6) / volume
		if i == 0 {
			shift = -shift
		}
		a, _ := result.VolumePiCoefficients[i].Float64()
		b, _ := result.VolumeOffsets[i].Float64()
		if math.Abs(m.Volume-volume) > 1e-7 || math.Abs(m.Volume-(math.Pi*a+b)) > 1e-7 || math.Abs(m.Area-area) > 1e-7 || m.Centroid.Sub(center.Add(geom.Vec3{X: shift})).Length() > 1e-8 {
			t.Fatal(m, volume, area, shift)
		}
		total += m.Volume
		if _, e = loaded.Bounds(); e != nil {
			t.Fatal(e)
		}
		mesh, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .2, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		report, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal(e, len(report.Contacts))
		}
		for _, contact := range result.ContactPoints {
			found := false
			for _, v := range loaded.Vertices() {
				if v == contact {
					found = true
				}
			}
			if !found {
				t.Fatal("unreported closure contact", contact)
			}
		}
	}
	if math.Abs(total+128.0/3-28*math.Pi) > 1e-7 {
		t.Fatal("material partition volume", total)
	}
}
func TestPerpendicularCylinderDifferenceRejects(t *testing.T) {
	for _, ab := range [][2]float64{{-2, 4}, {-3, 2}, {0, 4}} {
		r, e := PerpendicularCylinderDifference(context.Background(), geom.Vec3{}, 2, ab[0], ab[1], 5, 1e-9)
		if e == nil || len(r.Solids[0].Faces()) != 0 {
			t.Fatal(ab, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := PerpendicularCylinderDifference(ctx, geom.Vec3{}, 2, -3, 4, 5, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
}
