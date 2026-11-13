package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSteinmetzNegativeCap(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	for _, c := range []float64{3, 4} {
		r, e := PerpendicularCylinderNegativeCap(ctx, center, 5, -c, 6, 7, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Solid.Faces()) != 4 || len(r.Solid.Vertices()) != 4 || r.CapFace != 3 {
			t.Fatal("topology", len(r.Solid.Faces()), len(r.Solid.Vertices()))
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
		mass, e := r.Solid.MassProperties(ctx, IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		a := math.Sqrt(25 - c*c)
		want := 2*a*a*a/3 + 50*(a-c*math.Acos(c/5))
		if math.Abs(mass.Volume-want) > 1e-6 || mass.Centroid.X >= center.X-c {
			t.Fatal(mass, want)
		}
		lo, _ := r.IdealVolumeBounds[0].Float64()
		hi, _ := r.IdealVolumeBounds[1].Float64()
		if hi-lo > 1e-8 || want < lo-1e-10 || want > hi+1e-10 {
			t.Fatal("volume enclosure", lo, hi, want)
		}
		positive, e := PerpendicularCylinderPositiveCap(ctx, center, 5, c, 6, 7, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		pm, e := positive.Solid.MassProperties(ctx, IntegrationOptions{})
		if e != nil || math.Abs(pm.Volume+mass.Volume-2000.0/3) > 1e-6 {
			t.Fatal("complement", pm.Volume, mass.Volume, e)
		}
		mesh, e := r.Solid.Tessellate(ctx, TessellationOptions{Deflection: .2, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		for _, v := range mesh.Vertices {
			if v.X > center.X-c+1e-8 {
				t.Fatal("wrong side", v)
			}
		}
		report, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) != 0 {
			t.Fatal(e, len(report.Contacts))
		}
	}
}
func TestSteinmetzNegativeCapRejects(t *testing.T) {
	for _, c := range []float64{-5, -1, 0, 1} {
		r, e := PerpendicularCylinderNegativeCap(context.Background(), geom.Vec3{}, 5, c, 6, 7, 1e-8)
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal(c, e)
		}
	}
}

func TestNegativeCapCodecRejectsCollapsedRange(t *testing.T) {
	raw, e := EncodeSurface(steinmetzClippedCylinder{radius: 5, cap: -3, sign: 1})
	if e != nil {
		t.Fatal(e)
	}
	raw = bytes.Replace(raw, []byte(`"B":-3`), []byte(`"B":-5`), 1)
	if _, e = DecodeSurface(raw, 100); e == nil {
		t.Fatal("collapsed range")
	}
}
