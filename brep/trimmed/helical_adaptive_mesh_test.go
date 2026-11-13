package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestAdaptiveHelicalM3(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	pitch, major := .5, 1.5
	inner := major - math.Sqrt(3)*17/48*pitch
	a := pitch / 12
	b := a + (major-inner)/math.Sqrt(3)
	margin := pitch * .55
	p := []geom.Vec2{{X: inner, Y: margin - a}, {X: major, Y: margin - b}, {X: major, Y: margin + b}, {X: inner, Y: margin + a}}
	s, _, e := GrooveWithBoundedChart(context.Background(), HelicalGrooveRecipe{Frame: f, OuterRadius: major, Height: 12, Profile: p, Lead: pitch, Start: 0, End: 6, Hand: geom.RightHanded, Starts: 1}, 1e-12)
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range []float64{.5, .05} {
		m, e := s.TessellateHelical(context.Background(), parametric.TessellationOptions{Deflection: d, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(d, e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		t.Log(d, len(m.Triangles))
		mass, e := m.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		integral := a*(major*major-inner*inner) + 2*(b-a)/(major-inner)*((major*major*major-inner*inner*inner)/3-inner*(major*major-inner*inner)/2)
		want := math.Pi*major*major*12 - 2*math.Pi*6*integral
		if math.Abs(mass.Volume-want) > want*.01 {
			t.Fatal(mass.Volume, want)
		}
	}
}

func TestAdaptiveHelicalLimitsAndCancellation(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	s, e := GrooveCylinderWithHelicalStarts(context.Background(), f, 2, 3, []geom.Vec2{{X: 1.5, Y: .5}, {X: 2, Y: .375}, {X: 2, Y: .625}}, 1, .125, 1.125, geom.RightHanded, 1)
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []parametric.TessellationOptions{{Deflection: .5, MaxTriangles: 1}, {Deflection: .5, MaxSubdivisions: 1}, {Deflection: .5, AnalyticBound: true}, {Deflection: 0}, {Deflection: .5, Angle: math.NaN()}} {
		m, e := s.TessellateHelical(context.Background(), o)
		if e == nil || len(m.Triangles) != 0 {
			t.Fatal("accepted invalid or unfulfillable request", o, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = s.TessellateHelical(ctx, parametric.TessellationOptions{Deflection: .5}); e != context.Canceled {
		t.Fatal(e)
	}
}
