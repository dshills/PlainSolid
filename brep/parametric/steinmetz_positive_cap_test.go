package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSteinmetzPositiveCap(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	for _, c := range []float64{3, 4} {
		r, e := PerpendicularCylinderPositiveCap(ctx, center, 5, c, 6, 7, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Solid.Faces()) != 12 || len(r.XAxisFaces) != 6 || len(r.ZAxisFaces) != 5 || r.CapFace != 11 {
			t.Fatal(r)
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, r.Solid); e != nil {
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
		a := math.Sqrt(25 - c*c)
		want := 2000.0/3 - (2*a*a*a/3 + 50*(a-c*math.Acos(c/5)))
		if math.Abs(m.Volume-want) > 1e-6 {
			t.Fatal(m.Volume, want)
		}
		lo, _ := r.IdealVolumeBounds[0].Float64()
		hi, _ := r.IdealVolumeBounds[1].Float64()
		if hi-lo > 1e-8 || want < lo-1e-10 || want > hi+1e-10 {
			t.Fatal("volume enclosure", lo, hi, want)
		}
		for _, f := range r.Solid.Faces() {
			for _, t0 := range []float64{.1, .5, .9} {
				p, e := f.Surface.Eval(f.U.at(t0), f.V.at(.3))
				if e != nil {
					t.Fatal(e)
				}
				p = p.Sub(center)
				if p.X > c+1e-10 || p.X*p.X+p.Y*p.Y > 25+1e-10 || p.Z*p.Z+p.Y*p.Y > 25+1e-10 {
					t.Fatal("outside", p)
				}
			}
		}
		bounds, e := loaded.Bounds()
		if e != nil || bounds.Max.X < center.X+c {
			t.Fatal(bounds, e)
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
	}
}
func TestSteinmetzPositiveCapRejects(t *testing.T) {
	for _, c := range []float64{0, 1, 5} {
		r, e := PerpendicularCylinderPositiveCap(context.Background(), geom.Vec3{}, 5, c, 6, 7, 1e-8)
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal(c, e)
		}
	}
}
