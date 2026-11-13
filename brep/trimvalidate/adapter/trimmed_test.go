package adapter

import (
	"context"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestAdapter(t *testing.T) {
	p := []geom.Vec2{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	f := trimmed.Face{}
	for i, a := range p {
		f.Outer = append(f.Outer, trimmed.Use{UVStart: a, UVEnd: p[(i+1)%4]})
	}
	q := trimmed.QuadraticUV{Start: p[0], Control: geom.Vec2{X: 2, Y: -1}, End: p[1]}
	f.Outer[0].Curve = q
	r, e := Validate(context.Background(), f, trimvalidate.Options{})
	if e != nil || r.Status != trimvalidate.Certified {
		t.Fatal(r, e)
	}
	q.End.X = 3
	f.Outer[0].Curve = q
	r, e = Validate(context.Background(), f, trimvalidate.Options{})
	if e != nil || r.Status != trimvalidate.Invalid {
		t.Fatal(r, e)
	}
	f.Outer[0].Curve = trimmed.EllipseUV{}
	r, e = Validate(context.Background(), f, trimvalidate.Options{})
	if e != nil || r.Status != trimvalidate.Unresolved {
		t.Fatal(r, e)
	}
}

func TestRationalSplineQuarterAdapter(t *testing.T) {
	points := []geom.Vec2{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}
	controls := []geom.Vec2{{X: 1, Y: 1}, {X: -1, Y: 1}, {X: -1, Y: -1}, {X: 1, Y: -1}}
	f := trimmed.Face{}
	for i, p := range points {
		q := points[(i+1)%4]
		c, e := geom.NewBSpline(2, []geom.Vec3{{X: p.X, Y: p.Y}, {X: controls[i].X, Y: controls[i].Y}, {X: q.X, Y: q.Y}}, []float64{0, 0, 0, 1, 1, 1}, []float64{1, 1, 2})
		if e != nil {
			t.Fatal(e)
		}
		f.Outer = append(f.Outer, trimmed.Use{UVStart: p, UVEnd: q, Curve: trimmed.SplineUV{Curve: c, Range: trimmed.UVInterval{Min: 0, Max: 1}}})
	}
	r, e := Validate(context.Background(), f, trimvalidate.Options{})
	if e != nil || r.Status != trimvalidate.Certified {
		t.Fatal(r, e)
	}
}
