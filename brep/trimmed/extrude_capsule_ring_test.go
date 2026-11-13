package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCapsuleRingSignedAxes(t *testing.T) {
	for _, axis := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}, {X: -1}, {Y: -1}, {Z: -1}} {
		frame, _ := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		for _, h := range []float64{-5, 5} {
			for _, r := range [][2]float64{{6, 5}, {5, 4}} {
				s, e := ExtrudeCapsuleRing(context.Background(), frame, geom.Vec2{X: -10}, geom.Vec2{X: 10}, r[0], r[1], h)
				if e != nil {
					t.Fatal(axis, h, r, e)
				}
				area := 40*(r[0]-r[1]) + math.Pi*(r[0]*r[0]-r[1]*r[1])
				perimeter := 80 + 2*math.Pi*(r[0]+r[1])
				m, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil || math.Abs(m.Volume-5*area) > 1e-7 || math.Abs(m.Area-(2*area+5*perimeter)) > 1e-7 || m.Centroid.Sub(frame.ToWorld(geom.Vec3{Z: h / 2})).Length() > 1e-8 {
					t.Fatal(e, m)
				}
				mesh, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
				if e != nil {
					t.Fatal(e)
				}
				if e = mesh.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				var data bytes.Buffer
				if e = WriteJSON(&data, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadJSON(&data, JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				lm, e := loaded.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil || math.Abs(lm.Volume-m.Volume) > 1e-8 {
					t.Fatal(e, lm)
				}
			}
		}
	}
}
func TestCapsuleRingRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, r := range [][2]float64{{5, 5}, {4, 5}, {5, 0}, {5, math.NaN()}} {
		if _, e := ExtrudeCapsuleRing(context.Background(), f, geom.Vec2{}, geom.Vec2{X: 20}, r[0], r[1], 5); e == nil {
			t.Fatal(r)
		}
	}
}
