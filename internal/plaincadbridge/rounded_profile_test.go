package plaincadbridge

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestMixedRoundedRectangle(t *testing.T) {
	for _, axis := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{}, axis)
		x := coordinates(f.XAxis())
		z := coordinates(f.ZAxis())
		for _, height := range []number{-8, 8} {
			q := mixedProfileRequest{Kind: "roundedRectangle", Origin: &vector{0, 0, 13}, Normal: &z, XDirection: &x, Min: []number{-100, -90}, Max: []number{100, 90}, Radii: []number{12, 12, 12, 12}, Height: height}
			s, v, a, e := newMixedProfile(q)
			if e != nil {
				t.Fatal(e)
			}
			area := 36000 - (4-math.Pi)*144
			perimeter := 760 - 96 + 24*math.Pi
			near(t, v, area*8)
			near(t, a, 2*area+perimeter*8)
			integrated, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			near(t, v, integrated.Volume)
			near(t, a, integrated.Area)
			b := New()
			shape, e := b.mixedExtrude(q)
			if e != nil {
				t.Fatal(e)
			}
			box, e := geom.Bounds([]geom.Vec3{f.ToWorld(geom.Vec3{X: -100, Y: -90, Z: float64(min(0, height))}).Add(geom.Vec3{Z: 13}), f.ToWorld(geom.Vec3{X: 100, Y: 90, Z: float64(max(0, height))}).Add(geom.Vec3{Z: 13})})
			if e != nil {
				t.Fatal(e)
			}
			for i, want := range coordinates(box.Min) {
				if shape.Bounds.Min[i] > want || want-shape.Bounds.Min[i] > 1e-7 {
					t.Fatal("lower bounds", shape.Bounds, box)
				}
			}
			for i, want := range coordinates(box.Max) {
				if shape.Bounds.Max[i] < want || shape.Bounds.Max[i]-want > 1e-7 {
					t.Fatal("upper bounds", shape.Bounds, box)
				}
			}
			m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
		}
	}
}

func TestMixedRoundedRectangleWholeCapFillet(t *testing.T) {
	b := New()
	base, err := b.mixedExtrude(mixedProfileRequest{Kind: "roundedRectangle", Origin: &vector{}, Normal: &vector{0, 0, 1},
		XDirection: &vector{1, 0, 0}, Min: []number{-100, -90}, Max: []number{100, 90}, Radii: []number{12, 12, 12, 12}, Height: 12})
	if err != nil {
		t.Fatal(err)
	}
	result := shape(t, b, map[string]any{"op": "capFillet", "handle": base.Handle, "normal": vector{0, 0, 1}, "origin": vector{0, 0, 12}, "radius": 1})
	geometry := b.shapes[result.Handle].analytic
	actual, err := geometry.solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	near(t, result.GeometryAssertions.Volume, actual.Volume)
	near(t, result.GeometryAssertions.SurfaceArea, actual.Area)
}

func TestRoundedCableGuideWithDecimalFilletAndBores(t *testing.T) {
	b := New()
	base, err := b.mixedExtrude(mixedProfileRequest{Kind: "roundedRectangle", Origin: &vector{}, Normal: &vector{0, 0, 1},
		XDirection: &vector{1, 0, 0}, Min: []number{-29, -14}, Max: []number{29, 14}, Radii: []number{4, 4, 4, 4}, Height: 5})
	if err != nil {
		t.Fatal(err)
	}
	fillet := shape(t, b, map[string]any{"op": "capFillet", "handle": base.Handle, "normal": vector{0, 0, 1}, "origin": vector{0, 0, 5}, "radius": .6})
	first := circle(t, b, vector{22, 0, 0}, vector{0, 0, 1}, 2.2, 5.000002)
	second := circle(t, b, vector{-22, 0, 0}, vector{0, 0, 1}, 2.2, 5.000002)
	bored := shape(t, b, map[string]any{"op": "cutMany", "a": fillet.Handle, "tools": []string{first.Handle, second.Handle}})
	near(t, bored.GeometryAssertions.Volume, fillet.GeometryAssertions.Volume-10*math.Pi*2.2*2.2)
	native := b.shapes[bored.Handle].analytic.solid
	measured, err := native.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	near(t, bored.GeometryAssertions.Volume, measured.Volume)
	m, err := native.Tessellate(context.Background(), parametric.TessellationOptions{
		Deflection: .5, Angle: .1, MaxSubdivisions: 256, MaxTriangles: 100000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if len(m.Triangles) > 5000 {
		t.Fatal("rounded guide mesh exceeds compact display expectation", len(m.Triangles))
	}
}
func TestMixedRoundedRectangleRejects(t *testing.T) {
	base := mixedProfileRequest{Kind: "roundedRectangle", Origin: &vector{}, Normal: &vector{0, 0, 1}, XDirection: &vector{1, 0, 0}, Min: []number{0, 0}, Max: []number{20, 10}, Radii: []number{1, 1, 1, 1}, Height: 5}
	for _, mutate := range []func(*mixedProfileRequest){func(q *mixedProfileRequest) { q.Radii = []number{1, 1, 1} }, func(q *mixedProfileRequest) { q.Radii = []number{5, 5, 5, 5} }, func(q *mixedProfileRequest) { q.Radii = []number{-1, 1, 1, 1} }, func(q *mixedProfileRequest) { q.Center = []number{0, 0} }, func(q *mixedProfileRequest) { q.Circles = []mixedProfileCircle{{Center: []number{1, 1}, Radius: 1}} }} {
		q := base
		mutate(&q)
		if _, _, _, e := newMixedProfile(q); e == nil {
			t.Fatal("unsupported recipe accepted", q)
		}
	}
}
