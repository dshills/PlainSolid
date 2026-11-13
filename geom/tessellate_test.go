package geom

import (
	"errors"
	"math"
	"testing"
)

func TestTessellateCircle(t *testing.T) {
	c, _ := NewCircle(Vec3{}, Vec3{0, 0, 1}, 3)
	segments, e := TessellateCurve(c, 0, 2*math.Pi, CurveTessellationOptions{ChordTolerance: 1e-3})
	if e != nil || len(segments) < 100 {
		t.Fatal(len(segments), e)
	}
	for _, s := range segments {
		for i := 0; i <= 20; i++ {
			u := s.StartParameter + (s.EndParameter-s.StartParameter)*float64(i)/20
			p, _ := c.Eval(u)
			q, _, e := ClosestOnSegment(p, s.Start, s.End)
			if e != nil || p.Distance(q) > 1.001e-3 {
				t.Fatal(p.Distance(q), e)
			}
		}
	}
	near(t, segments[0].Start, segments[len(segments)-1].End, 1e-14)
	_, e = TessellateCurve(c, 0, 2*math.Pi, CurveTessellationOptions{ChordTolerance: 1e-12, InitialSegments: 1, MaxEvaluations: 7})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
func TestTessellateDiscontinuousSpline(t *testing.T) {
	c, _ := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {5, 0, 0}, {8, 0, 0}}, []float64{0, 0, 1, 1, 2, 2}, nil)
	segments, e := TessellateCurve(c, 0, 2, CurveTessellationOptions{InitialSegments: 1})
	if e != nil || len(segments) != 2 {
		t.Fatal(segments, e)
	}
	near(t, segments[0].End, Vec3{1, 0, 0}, 0)
	near(t, segments[1].Start, Vec3{5, 0, 0}, 0)
	if segments[0].EndParameter != 1 || segments[1].StartParameter != 1 {
		t.Fatal(segments)
	}
}
func TestTessellateRational(t *testing.T) {
	circle, _ := NewCircle(Vec3{}, Vec3{0, 0, 1}, 1)
	c, _ := circle.RationalBSpline()
	segments, e := TessellateCurve(c, 0, 4, CurveTessellationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range segments {
		if math.Abs(s.Start.Length()-1) > 1e-14 || math.Abs(s.End.Length()-1) > 1e-14 {
			t.Fatal(s)
		}
	}
	if _, e = TessellateCurve(c, -1, 4, CurveTessellationOptions{}); e == nil {
		t.Fatal("domain")
	}
	if _, e = TessellateCurve(c, 0, 4, CurveTessellationOptions{ChordTolerance: -1}); e == nil {
		t.Fatal("tolerance")
	}
}
