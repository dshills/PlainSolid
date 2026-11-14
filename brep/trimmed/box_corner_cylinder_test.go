package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestBoxCornerCylinder(t *testing.T) {
	for _, axis := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}, {X: -1}, {Y: -1}, {Z: -1}} {
		frame, _ := geom.FrameFromNormal(geom.Vec3{}, axis)
		for _, fuse := range []bool{false, true} {
			s, e := BoxCornerCylinder(context.Background(), frame, 20, 10, 20, 5, 10, fuse, 1e-9)
			if e != nil {
				t.Fatal(axis, fuse, e)
			}
			m, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
			delta := -25 * math.Pi * 10 / 4
			if fuse {
				delta = 3 * 25 * math.Pi * 10 / 4
			}
			if e != nil || math.Abs(m.Volume-(4000+delta)) > 1e-7 {
				t.Fatal(axis, fuse, e, m.Volume, 4000+delta)
			}
			wantArea := 1600 - 100 + 25*math.Pi
			radialMoment := delta * 20 / (3 * math.Pi)
			if fuse {
				wantArea = 1600 - 100 + 112.5*math.Pi
				radialMoment = delta * (-20 / (9 * math.Pi))
			}
			wantCenter := frame.ToWorld(geom.Vec3{X: (4000*10 + radialMoment) / (4000 + delta), Y: (4000*5 + radialMoment) / (4000 + delta), Z: (4000*10 + delta*5) / (4000 + delta)})
			if math.Abs(m.Area-wantArea) > 1e-7 || m.Centroid.Sub(wantCenter).Length() > 1e-8 {
				t.Fatal("independent area/centroid", m.Area, wantArea, m.Centroid, wantCenter)
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
				t.Fatal("reload", e, lm)
			}
			mesh, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
			if e != nil {
				t.Fatal(axis, fuse, e)
			}
			if e = mesh.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
		}
	}
}

func TestBoxCornerCylinderRejects(t *testing.T) {
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, x := range [][2]float64{{0, 10}, {10, 10}, {5, 20}, {5, 0}, {math.Inf(1), 10}, {5, math.NaN()}} {
		if _, e := BoxCornerCylinder(context.Background(), frame, 20, 10, 20, x[0], x[1], false, 1e-9); e == nil {
			t.Fatal("accepted", x)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := BoxCornerCylinder(ctx, frame, 20, 10, 20, 5, 10, false, 1e-9); e != context.Canceled {
		t.Fatal(e)
	}
	s, e := BoxCornerCylinder(context.Background(), frame, 20, 10, 20, 5, 10, true, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .01, Angle: .01, MaxTriangles: 2}); e == nil {
		t.Fatal("ignoredbudget")
	}
}
