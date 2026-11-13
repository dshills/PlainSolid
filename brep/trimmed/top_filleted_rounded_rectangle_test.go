package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/exchange"
	"plainsolid/geom"
	"testing"
)

func TestTopFilletedRoundedRectangle(t *testing.T) {
	ctx := context.Background()
	const w, d, h, R, r = 200., 180., 8., 12., 1.
	A := w*d - (4-math.Pi)*R*R
	P := 2*(w+d) - 8*R + 2*math.Pi*R
	removed := P*r*r*(1-math.Pi/4) - math.Pi*r*r*r*(5./3-math.Pi/2)
	volume := A*h - removed
	area := A + (A - P*r + math.Pi*r*r) + P*(h-r) + math.Pi*r*P/2 - math.Pi*math.Pi*r*r + 2*math.Pi*r*r
	moment := (h-r)*removed + P*r*r*r/6 - math.Pi*r*r*r*r/12
	center := geom.Vec3{Z: (A*h*h/2 - moment) / volume}
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, e := geom.FrameFromNormal(geom.Vec3{X: 3, Y: 5, Z: 7}, axis)
		if e != nil {
			t.Fatal(e)
		}
		profile := RoundedRectangle{Min: geom.Vec2{X: -w / 2, Y: -d / 2}, Max: geom.Vec2{X: w / 2, Y: d / 2}, Radii: [4]float64{R, R, R, R}}
		s, e := TopFilletedRoundedRectangle(ctx, f, profile, h, r, 1e-8)
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

func TestTopFilletedRoundedRectangleRejects(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := RoundedRectangle{Min: geom.Vec2{X: -100, Y: -90}, Max: geom.Vec2{X: 100, Y: 90}, Radii: [4]float64{12, 12, 12, 12}}
	for _, r := range []float64{-1, 0, 6, 8, math.Inf(1), math.NaN()} {
		if _, e := TopFilletedRoundedRectangle(ctx, f, p, 8, r, 1e-8); e == nil {
			t.Fatal("accepted radius", r)
		}
	}
	mixed := p
	mixed.Radii[1] = 11
	if _, e := TopFilletedRoundedRectangle(ctx, f, mixed, 8, 1, 1e-8); e == nil {
		t.Fatal("accepted nonuniform corners")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := TopFilletedRoundedRectangle(cancelled, f, p, 8, 1, 1e-8); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	s, e := TopFilletedRoundedRectangle(ctx, f, p, 8, 1, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	tori, cylinders := 0, 0
	for _, face := range s.Faces() {
		switch face.Surface.(type) {
		case geom.TorusSurface:
			tori++
		case geom.CylinderSurface:
			cylinders++
		}
	}
	if tori != 4 || cylinders != 8 {
		t.Fatal("lost analytic supports", tori, cylinders)
	}
	if _, e = s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .01, MaxTriangles: 1}); !errors.Is(e, parametric.ErrBudget) {
		t.Fatal(e)
	}
}
