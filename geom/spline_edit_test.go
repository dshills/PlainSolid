package geom

import (
	"errors"
	"math"
	"testing"
)

func TestZeroSplineDomain(t *testing.T) {
	for _, c := range []*BSpline{nil, {}} {
		a, b := c.Domain()
		if !math.IsNaN(a) || !math.IsNaN(b) {
			t.Fatal(a, b)
		}
		if _, e := c.Eval(0); e == nil {
			t.Fatal("zero eval")
		}
	}
	for _, s := range []*BSplineSurface{nil, {}} {
		a, b, c, d := s.Domain()
		if !math.IsNaN(a) || !math.IsNaN(b) || !math.IsNaN(c) || !math.IsNaN(d) {
			t.Fatal(a, b, c, d)
		}
	}
}
func TestSplineNonclampedRepeatedEnd(t *testing.T) {
	c, e := NewBSpline(2, []Vec3{{0, 0, 0}, {1, 1, 0}, {2, 0, 0}, {3, 4, 0}, {4, 0, 0}}, []float64{-1, 0, 0, 1, 1, 1, 2, 3}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := c.Eval(1)
	if e != nil {
		t.Fatal(e)
	}
	near(t, p, Vec3{2, 0, 0}, 0)
	d, e := c.Derivative(1)
	if e != nil {
		t.Fatal(e)
	}
	near(t, d, Vec3{2, -2, 0}, 0)
}
func TestSplineDiscontinuousKnot(t *testing.T) {
	c, e := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {5, 0, 0}, {8, 0, 0}}, []float64{0, 0, 1, 1, 2, 2}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, _ := c.Eval(1)
	near(t, p, Vec3{5, 0, 0}, 0)
	d, _ := c.Derivative(1)
	near(t, d, Vec3{3, 0, 0}, 0)
	left, right, e := c.Split(1)
	if e != nil {
		t.Fatal(e)
	}
	p, _ = left.Eval(1)
	near(t, p, Vec3{1, 0, 0}, 0)
	p, _ = right.Eval(1)
	near(t, p, Vec3{5, 0, 0}, 0)
	r, e := ArcLength(c, 0, 2, LengthOptions{})
	if e != nil || math.Abs(r.Length-4) > 1e-14 {
		t.Fatal(r, e)
	}
}
func TestKnotInsertionAndSplit(t *testing.T) {
	circle, _ := NewCircle(Vec3{2, -3, 4}, Vec3{0, 0, 1}, 5)
	c, _ := circle.RationalBSpline()
	original := c.Poles()
	refined, e := c.InsertKnot(.37, 3)
	if e != nil {
		t.Fatal(e)
	}
	if len(refined.Poles()) != len(original)+3 {
		t.Fatal("pole count")
	}
	for i := 0; i <= 200; i++ {
		u := float64(i) / 50
		p, _ := c.Eval(u)
		q, e := refined.Eval(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, p, q, 5e-15)
	}
	left, right, e := c.Split(2.3)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i <= 100; i++ {
		u := float64(i) / 25
		p, _ := c.Eval(u)
		piece := left
		if u >= 2.3 {
			piece = right
		}
		q, e := piece.Eval(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, p, q, 5e-15)
	}
	if _, e = c.InsertKnot(1, 2); e == nil {
		t.Fatal("excess multiplicity")
	}
	if _, e = c.InsertKnot(0, 1); e == nil {
		t.Fatal("endpoint")
	}
	if _, _, e = c.Split(4); e == nil {
		t.Fatal("endpoint split")
	}
	for i, p := range original {
		near(t, c.Poles()[i], p, 0)
	}
}
func TestUnclampedSplit(t *testing.T) {
	c, e := NewBSpline(2, []Vec3{{0, 0, 0}, {2, 1, 0}, {4, 0, 0}}, []float64{-2, -1, 0, 1, 2, 3}, nil)
	if e != nil {
		t.Fatal(e)
	}
	l, r, e := c.Split(.4)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i <= 100; i++ {
		u := float64(i) / 100
		p, _ := c.Eval(u)
		piece := l
		if u >= .4 {
			piece = r
		}
		q, e := piece.Eval(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, p, q, 2e-15)
	}
}
func TestBezierAndArcLength(t *testing.T) {
	b, e := NewBezier([]Vec3{{0, 0, 0}, {3, 4, 0}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	r, e := ArcLength(b, 0, 1, LengthOptions{})
	if e != nil || math.Abs(r.Length-5) > 1e-14 {
		t.Fatal(r, e)
	}
	circle, _ := NewCircle(Vec3{}, Vec3{0, 0, 1}, 3)
	c, _ := circle.RationalBSpline()
	r, e = ArcLength(c, 0, 4, LengthOptions{AbsoluteTolerance: 1e-11, RelativeTolerance: 1e-11})
	if e != nil || math.Abs(r.Length-6*math.Pi) > 1e-10 {
		t.Fatal(r, e)
	}
	r, e = ArcLength(circle, 0, 2*math.Pi, LengthOptions{})
	if e != nil || math.Abs(r.Length-6*math.Pi) > 1e-12 {
		t.Fatal(r, e)
	}
	_, e = ArcLength(c, 0, 4, LengthOptions{AbsoluteTolerance: 1e-15, RelativeTolerance: 1e-15, MaxEvaluations: 7})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	if _, e = ArcLength(c, -1, 1, LengthOptions{}); e == nil {
		t.Fatal("domain")
	}
	if _, e = NewBezier(nil, nil); e == nil {
		t.Fatal("empty")
	}
}
