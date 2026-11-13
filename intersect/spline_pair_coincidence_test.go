package intersect

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestSplinePairCoincidenceAffineCharts(t *testing.T) {
	ctx := context.Background()
	unit := []float64{0, 0, 1, 1}
	a, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, unit, unit, nil)
	if e != nil {
		t.Fatal(e)
	}
	makeB := func(perturb bool) Patch {
		t.Helper()
		var poles []geom.Vec3
		for i := range 3 {
			y := 1 - float64(i)/2
			poles = append(poles, geom.Vec3{Y: y}, geom.Vec3{X: 1, Y: y, Z: y})
		}
		if perturb {
			poles[3].Z = math.Nextafter(poles[3].Z, 1)
		}
		b, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, []float64{2, 2, 2, 6, 6, 6}, []float64{10, 10, 12, 12}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return Patch{b, geom.Vec2{X: 2, Y: 10}, geom.Vec2{X: 6, Y: 12}}
	}
	pa, pb := Patch{a, geom.Vec2{}, geom.Vec2{X: 1, Y: 1}}, makeB(false)
	r, e := TraceSplinePair(ctx, pa, pb, SplinePairTraceOptions{})
	m := r.Coincidence.Map
	if !errors.Is(e, ErrUnresolved) || r.Complete || !r.Coincidence.Found || !r.Coincidence.Complete || len(r.Curves) != 0 ||
		!m.Transposed || !m.ReverseU || m.ReverseV || m.Orientation() != 1 {
		t.Fatal(r, e)
	}
	q := geom.Vec2{X: .25, Y: .75}
	p, e := m.MapExact(q)
	if e != nil || p != (geom.Vec2{X: 3, Y: 10.5}) {
		t.Fatal(p, e)
	}
	back, e := m.Inverse().MapExact(p)
	if e != nil || back != q {
		t.Fatal(back, e)
	}
	pointA, e := pa.Surface.Eval(q.X, q.Y)
	pointB, f := pb.Surface.Eval(p.X, p.Y)
	if e != nil || f != nil || pointA != pointB {
		t.Fatal(pointA, pointB, e, f)
	}
	c, e := FindSplineCoincidence(ctx, pa, makeB(true), SplineCoincidenceOptions{})
	if e != nil || c.Found || !c.Complete || c.Attempted != 8 {
		t.Fatal(c, e)
	}
	c, e = FindSplineCoincidence(ctx, pa, pb, SplineCoincidenceOptions{MaxOperations: 1})
	if !errors.Is(e, geom.ErrConvergence) || c.Found || c.Complete {
		t.Fatal(c, e)
	}
}

func TestSplineChartMapDoesNotSnap(t *testing.T) {
	a := geom.ParameterRectangle{UMax: 3, VMax: 1}
	b := geom.ParameterRectangle{UMax: 1, VMax: 1}
	m := SplineChartMap{DomainA: a, DomainB: b, ReverseU: true}
	if m.Orientation() != -1 {
		t.Fatal(m)
	}
	x := geom.Vec2{X: 1, Y: .5}
	if _, e := m.MapExact(x); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal(e)
	}
	if q, e := m.Map(x); e != nil || q != (geom.Vec2{X: 2.0 / 3, Y: .5}) {
		t.Fatal(q, e)
	}
	if _, e := m.Map(geom.Vec2{X: 4}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
