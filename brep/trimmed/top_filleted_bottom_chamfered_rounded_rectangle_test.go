package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/exchange"
	"plainsolid/geom"
	"testing"
)

func TestTopFilletedBottomChamferedRoundedRectangle(t *testing.T) {
	ctx := context.Background()
	const w, d, h, R, r = 140., 108., 10., 10., 1.
	A := w*d - (4-math.Pi)*R*R
	P := 2*(w+d) - 8*R + 2*math.Pi*R
	removed := P*r*r*(1-math.Pi/4) - math.Pi*r*r*r*(5./3-math.Pi/2)
	const c = 1.
	lowerRemoved := P*c*c/2 - math.Pi*c*c*c/3
	volume := A*h - removed - lowerRemoved
	area := A + (A - P*r + math.Pi*r*r) + P*(h-r) + math.Pi*r*P/2 - math.Pi*math.Pi*r*r + 2*math.Pi*r*r
	area += P*c*(math.Sqrt2-2) + math.Pi*c*c*(1-math.Sqrt2)
	moment := (h-r)*removed + P*r*r*r/6 - math.Pi*r*r*r*r/12
	moment += P*c*c*c/6 - math.Pi*c*c*c*c/12
	center := geom.Vec3{Z: (A*h*h/2 - moment) / volume}
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		if e != nil {
			t.Fatal(e)
		}
		profile := RoundedRectangle{Min: geom.Vec2{X: -w / 2, Y: -d / 2}, Max: geom.Vec2{X: w / 2, Y: d / 2}, Radii: [4]float64{R, R, R, R}}
		s, e := TopFilletedBottomChamferedRoundedRectangle(ctx, f, profile, h, r, c, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-volume) > 1e-7 || math.Abs(p.Area-area) > 1e-7 || p.Centroid.Sub(f.ToWorld(center)).Length() > 1e-8 {
			t.Fatal(p, volume, area, center)
		}
		compact, eligible, err := s.compactCylinderMesh(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if err != nil || !eligible {
			t.Fatal("compact", eligible, err)
		}
		if err = compact.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		t.Logf("axis %v: %d compact triangles", axis, len(compact.Triangles))
		m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 1000000})
		if e != nil {
			t.Fatal("mesh", e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		var stl bytes.Buffer
		if e = exchange.WriteSTL(&stl, m); e != nil {
			t.Fatal(e)
		}
		exported, e := exchange.ReadSTL(&stl, exchange.Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if e = exported.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		var buf bytes.Buffer
		if e = WriteJSON(&buf, s); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		q, e := loaded.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(q.Volume-volume) > 1e-7 {
			t.Fatal(q, e)
		}
	}
}

func TestTopFilletedBottomChamferedRoundedRectangleRejects(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := RoundedRectangle{Min: geom.Vec2{X: -70, Y: -54}, Max: geom.Vec2{X: 70, Y: 54}, Radii: [4]float64{10, 10, 10, 10}}
	for _, q := range [][3]float64{{10, 1, 0}, {10, 0, 1}, {10, 1, -1}, {10, 1, 9}, {10, 1, 10}, {10, 5, 1}, {10, 1, math.NaN()}} {
		if _, e := TopFilletedBottomChamferedRoundedRectangle(context.Background(), f, p, q[0], q[1], q[2], 1e-8); e == nil {
			t.Fatal("accepted invalid", q)
		}
	}
	if _, e := TopFilletedBottomChamferedRoundedRectangle(nil, f, p, 10, 1, 1, 1e-8); e == nil {
		t.Fatal("accepted nil context")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := TopFilletedBottomChamferedRoundedRectangle(ctx, f, p, 10, 1, 1, 1e-8); e != context.Canceled {
		t.Fatal(e)
	}
	p.Radii[1] = 9
	if _, e := TopFilletedBottomChamferedRoundedRectangle(context.Background(), f, p, 10, 1, 1, 1e-8); e == nil {
		t.Fatal("accepted nonuniform corners")
	}
}
