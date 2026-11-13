package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"testing"
)

func mixedFixture(kind string) mixedProfileRequest {
	return mixedProfileRequest{Kind: kind, Origin: &vector{0, 0, 13}, Normal: &vector{0, 0, 1}, XDirection: &vector{1, 0, 0}, Center: []number{0, 0}, Radius: 10, Height: 5}
}
func TestMixedProfilesRetainAnalyticQuantities(t *testing.T) {
	for _, kind := range []string{"semicircle", "capsule", "disk", "polygon"} {
		for _, sign := range []number{-1, 1} {
			q := mixedFixture(kind)
			q.Height *= sign
			var area, perimeter float64
			switch kind {
			case "semicircle":
				area = 50 * math.Pi
				perimeter = 20 + 10*math.Pi
			case "capsule":
				q.Right = []number{20, 0}
				area = 400 + 100*math.Pi
				perimeter = 40 + 20*math.Pi
			case "disk":
				q.Circles = []mixedProfileCircle{{Center: []number{2, 0}, Radius: 2}}
				area = 96 * math.Pi
				perimeter = 24 * math.Pi
			case "polygon":
				q.Center = nil
				q.Radius = 0
				q.Outer = [][]number{{-10, -10}, {10, -10}, {10, 10}, {-10, 10}}
				q.Circles = []mixedProfileCircle{{Center: []number{2, 0}, Radius: 2}}
				area = 400 - 4*math.Pi
				perimeter = 80 + 4*math.Pi
			}
			s, v, a, e := newMixedProfile(q)
			if e != nil {
				t.Fatal(kind, e)
			}
			near(t, v, 5*area)
			near(t, a, 2*area+5*perimeter)
			integrated, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			near(t, integrated.Volume, v)
			near(t, integrated.Area, a)
			m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .1, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
			if e != nil {
				t.Fatal(kind, e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestMixedProfilesRejectUnprovedFamilies(t *testing.T) {
	cases := []mixedProfileRequest{}
	q := mixedFixture("semicircle")
	q.Circles = []mixedProfileCircle{{Center: []number{0, 3}, Radius: 1}}
	cases = append(cases, q)
	q = mixedFixture("capsule")
	q.Right = []number{20, 1}
	cases = append(cases, q)
	q = mixedFixture("disk")
	q.Circles = []mixedProfileCircle{{Center: []number{8, 0}, Radius: 2}}
	cases = append(cases, q)
	q = mixedFixture("semicircle")
	q.XDirection = &vector{1, 0, 1}
	cases = append(cases, q)
	q = mixedFixture("semicircle")
	q.Center = []number{0}
	cases = append(cases, q)
	q = mixedFixture("unknown")
	cases = append(cases, q)
	q = mixedFixture("semicircle")
	q.Height = 0
	cases = append(cases, q)
	for i, q := range cases {
		if _, _, _, e := newMixedProfile(q); e == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestMixedCapsuleRing(t *testing.T) {
	for _, h := range []number{-5, 5} {
		q := mixedFixture("capsule")
		q.Center = []number{-10, 0}
		q.Right = []number{10, 0}
		q.Radius = 6
		q.InnerRadius = 5
		q.Height = h
		s, v, a, e := newMixedProfile(q)
		if e != nil {
			t.Fatal(e)
		}
		area := 11*math.Pi + 40
		perimeter := 22*math.Pi + 80
		near(t, v, area*5)
		near(t, a, 2*area+perimeter*5)
		m, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		near(t, m.Volume, v)
		near(t, m.Area, a)
		for _, bad := range []number{-1, 6, 7} {
			q.InnerRadius = bad
			if _, _, _, e = newMixedProfile(q); e == nil {
				t.Fatal("invalid inner capsule accepted", bad)
			}
		}
	}
}

func TestMixedSemicircleExactBoundsAndCap(t *testing.T) {
	b := New()
	s := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
		"kind": "semicircle", "origin": vector{0, 0, 13}, "normal": vector{0, 0, 1},
		"xDirection": vector{1, 0, 0}, "center": []float64{0, 0}, "radius": 10, "height": 5,
	}})
	if s.Bounds.Min != (vector{-10, 0, 13}) || s.Bounds.Max != (vector{10, 10, 18}) {
		t.Fatal(s.Bounds)
	}
	f, err := b.shapes[s.Handle].analytic.planarFace(vector{0, 0, 18}.vec(), vector{0, 0, 1}.vec())
	if err != nil {
		t.Fatal(err)
	}
	near(t, f.Area, 50*math.Pi)
}

func TestMixedCircularSegmentRetainsSolverArc(t *testing.T) {
	const radius, height = 20.0, 5.0
	const start, sweep = 2.5002222596456e-7, 3.141592153545341
	b := New()
	s := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
		"kind": "circularSegment", "origin": vector{0, 0, 13}, "normal": vector{0, 0, 1},
		"xDirection": vector{1, 0, 0}, "center": []float64{0, 0}, "radius": radius, "height": height,
		"startAngle": start, "sweep": sweep,
	}})
	near(t, s.Bounds.Min[1], radius*math.Sin(start))
	near(t, s.Bounds.Max[1], radius)
	want := radius * radius * (sweep - math.Sin(sweep)) * height / 2
	near(t, s.GeometryAssertions.Volume, want)
	m, err := b.shapes[s.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}
