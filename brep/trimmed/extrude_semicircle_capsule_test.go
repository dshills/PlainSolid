package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestSemicircleCapsuleSignedFrames(t *testing.T) {
	for _, axis := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}, {X: -1}, {Y: -1}, {Z: -1}} {
		frame, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		if e != nil {
			t.Fatal(e)
		}
		for _, height := range []float64{-5, 5} {
			for _, capsule := range []bool{false, true} {
				var s Solid
				var area, perimeter, cy float64
				if capsule {
					s, e = ExtrudeCapsule(context.Background(), frame, geom.Vec2{X: -10}, geom.Vec2{X: 10}, 5, height)
					area, perimeter = 200+25*math.Pi, 40+10*math.Pi
				} else {
					s, e = ExtrudeSemicircle(context.Background(), frame, geom.Vec2{}, 10, height)
					area, perimeter, cy = 50*math.Pi, 20+10*math.Pi, 40/(3*math.Pi)
				}
				if e != nil {
					t.Fatal(axis, height, capsule, e)
				}
				m, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if e != nil || math.Abs(m.Volume-area*5) > 1e-7 || math.Abs(m.Area-(2*area+perimeter*5)) > 1e-7 {
					t.Fatal(axis, height, capsule, e, m)
				}
				want := frame.ToWorld(geom.Vec3{Y: cy, Z: height / 2})
				if m.Centroid.Sub(want).Length() > 1e-8 {
					t.Fatal("centroid", m.Centroid, want)
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
					t.Fatal("persistence", e, lm)
				}
			}
		}
	}
}
func TestSemicircleCapsuleRejectsAndLimits(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if _, e := ExtrudeCapsule(context.Background(), f, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}, 1, 5); e == nil {
		t.Fatal("nonparallel")
	}
	if _, e := ExtrudeSemicircle(context.Background(), f, geom.Vec2{X: 1}, math.SmallestNonzeroFloat64, 5); e == nil {
		t.Fatal("unrepresentable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ExtrudeSemicircle(ctx, f, geom.Vec2{}, 1, 5); e != context.Canceled {
		t.Fatal(e)
	}
	s, e := ExtrudeCapsule(context.Background(), f, geom.Vec2{}, geom.Vec2{X: 2}, 1, 5)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .01, Angle: .01, MaxTriangles: 2}); e == nil {
		t.Fatal("ignored mesh budget")
	}
}

func TestMixedCircularHolesExistingNativePaths(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	hole := ExtrusionCircle{Radius: 2}
	annulus, e := ExtrudeDiskWithHoles(context.Background(), f, ExtrusionCircle{Radius: 5}, []ExtrusionCircle{hole}, nil, 3)
	if e != nil {
		t.Fatal(e)
	}
	polygon, e := ExtrudePolygonWithHoles(context.Background(), f, []geom.Vec2{{X: -5, Y: -5}, {X: 5, Y: -5}, {X: 5, Y: 5}, {X: -5, Y: 5}}, []ExtrusionCircle{hole}, nil, 3)
	if e != nil {
		t.Fatal(e)
	}
	for i, s := range []Solid{annulus, polygon} {
		area := 21 * math.Pi
		perimeter := 14 * math.Pi
		if i == 1 {
			area = 100 - 4*math.Pi
			perimeter = 40 + 4*math.Pi
		}
		m, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil || math.Abs(m.Volume-3*area) > 1e-7 || math.Abs(m.Area-(2*area+3*perimeter)) > 1e-7 {
			t.Fatal(i, e, m)
		}
		mesh, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
		if e != nil {
			t.Fatal(i, e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(i, e)
		}
	}
}
