package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestPerpendicularCylinderCommon(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	r, e := PerpendicularCylinderCommon(ctx, center, 2, 3, 4, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Solid.Faces()) != 4 || len(r.Solid.Vertices()) != 2 {
		t.Fatal("topology", len(r.Solid.Faces()), len(r.Solid.Vertices()))
	}
	var b bytes.Buffer
	if e = WriteJSON(&b, r.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := ReadJSON(&b, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []Solid{r.Solid, loaded} {
		mass, e := s.MassProperties(ctx, IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		v, _ := r.IdealVolume.Float64()
		if math.Abs(mass.Volume-v) > 1e-7 || math.Abs(mass.Area-64) > 1e-7 || mass.Centroid.Sub(center).Length() > 1e-8 {
			t.Fatal(mass, v)
		}
		bounds, e := s.Bounds()
		if e != nil || bounds.Min.X > 6 || bounds.Max.Z < 4 {
			t.Fatal(bounds, e)
		}
		for _, face := range s.Faces() {
			for _, u := range []float64{-1, -.5, 0, .5, 1} {
				p, e := face.Surface.Eval(u, .3)
				if e != nil {
					t.Fatal(e)
				}
				p = p.Sub(center)
				if p.X*p.X+p.Y*p.Y > 4+1e-12 || p.Z*p.Z+p.Y*p.Y > 4+1e-12 {
					t.Fatal("outside common", p)
				}
			}
		}
	}
	m, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .05, MaxSubdivisions: 128, MaxTriangles: 100000})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	report, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if e != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatal(e, len(report.Contacts))
	}
}
func TestPerpendicularCylinderCommonRejects(t *testing.T) {
	for _, v := range []float64{1, 2, math.Nextafter(2, 3)} {
		r, e := PerpendicularCylinderCommon(context.Background(), geom.Vec3{}, 2, v, 3, 1e-9)
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal(v, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := PerpendicularCylinderCommon(ctx, geom.Vec3{}, 2, 3, 3, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
	raw, e := EncodeSurface(steinmetzChart{radius: 2, sign: 1})
	if e != nil {
		t.Fatal(e)
	}
	raw = bytes.Replace(raw, []byte(`"Steinmetz":[0,1]`), []byte(`"Steinmetz":[0,0]`), 1)
	if _, e = DecodeSurface(raw, 100); e == nil {
		t.Fatal("invalid chart accepted", string(raw))
	}
}
