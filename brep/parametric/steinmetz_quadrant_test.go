package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestSteinmetzQuadrant(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	total := 0.0
	for _, px := range []bool{false, true} {
		for _, pz := range []bool{false, true} {
			r, e := PerpendicularCylinderQuadrant(ctx, center, 2, 3, 4, 1e-9, px, pz)
			if e != nil {
				t.Fatal(px, pz, e)
			}
			if len(r.Solid.Faces()) != 4 || len(r.Solid.Vertices()) != 2 || r.ZAxisFace != 0 || r.XAxisFace != 1 || r.XCapFace != 2 || r.ZCapFace != 3 {
				t.Fatal("topology", r)
			}
			var b bytes.Buffer
			if e = WriteJSON(&b, r.Solid); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadJSON(&b, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			m, e := loaded.MassProperties(ctx, IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			v, _ := r.IdealVolume.Float64()
			dx, dz := -9*math.Pi*2/64, -9*math.Pi*2/64
			if px {
				dx = -dx
			}
			if pz {
				dz = -dz
			}
			wantCenter := center.Add(geom.Vec3{X: dx, Z: dz})
			if math.Abs(m.Volume-v) > 1e-7 || math.Abs(m.Area-(4+math.Pi)*4) > 1e-7 || m.Centroid.Sub(wantCenter).Length() > 1e-8 {
				t.Fatal(m, v, wantCenter)
			}
			total += m.Volume
			if _, e = loaded.Bounds(); e != nil {
				t.Fatal(e)
			}
			mesh, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .15, MaxSubdivisions: 128, MaxTriangles: 100000})
			if e != nil {
				t.Fatal(e)
			}
			if e = mesh.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			for _, p := range mesh.Vertices {
				if px && p.X < center.X || !px && p.X > center.X || pz && p.Z < center.Z || !pz && p.Z > center.Z {
					t.Fatal("wrong quadrant", p)
				}
			}
			report, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
			if e != nil || !report.Complete || len(report.Contacts) != 0 {
				t.Fatal(e, len(report.Contacts))
			}
		}
	}
	if math.Abs(total-128.0/3) > 1e-7 {
		t.Fatal("partition volume", total)
	}
}
func TestSteinmetzQuadrantRejects(t *testing.T) {
	for _, ext := range [][2]float64{{2, 4}, {3, 2}} {
		r, e := PerpendicularCylinderQuadrant(context.Background(), geom.Vec3{}, 2, ext[0], ext[1], 1e-9, false, true)
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal(ext, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := PerpendicularCylinderQuadrant(ctx, geom.Vec3{}, 2, 3, 4, 1e-9, true, false); e != context.Canceled {
		t.Fatal(e)
	}
}
