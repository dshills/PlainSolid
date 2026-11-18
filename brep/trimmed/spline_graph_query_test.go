package trimmed

import (
	"context"
	"errors"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestSplineGraphQuery(t *testing.T) {
	slowtest.Skip(t)
	k := []float64{0, 0, 0, 1, 1, 1}
	q := [3]float64{.25, -.25, .25}
	const rr = .09765625 // (5/16)^2, with exactly representable section points
	var a, b []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			p := geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: q[j]/4 + 1}
			b = append(b, p)
			p.Z += q[i] + q[j] - rr
			a = append(a, p)
		}
	}
	makeSpline := func(p []geom.Vec3) *geom.BSplineSurface {
		t.Helper()
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	sa, sb := makeSpline(a), makeSpline(b)
	ctx := context.Background()
	for _, order := range [][2]*geom.BSplineSurface{{sa, sb}, {sb, sa}} {
		r, e := NewSplineGraphQuery(ctx, order[0], order[1], geom.ParameterRectangle{}, geom.SurfacePatchOptions{}, geom.SurfaceXYInverseOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, tc := range []struct {
			p    geom.Vec3
			want parametric.Location
		}{
			{geom.Vec3{X: .5, Y: .5, Z: 1 - rr/2}, parametric.Inside},
			{geom.Vec3{X: .5, Y: .5, Z: 1 - rr}, parametric.Boundary},
			{geom.Vec3{X: .5, Y: .5, Z: 1}, parametric.Boundary},
			{geom.Vec3{X: .5 + 5./16, Y: .5, Z: 1}, parametric.Boundary},
			{geom.Vec3{X: .99, Y: .5, Z: 1}, parametric.Outside},
			{geom.Vec3{X: 2, Y: .5, Z: 1}, parametric.Outside},
			{geom.Vec3{X: .5, Y: .5, Z: 1.1}, parametric.Outside},
		} {
			got, e := r.Classify(ctx, tc.p)
			if e != nil || got != tc.want {
				t.Fatal(tc, got, e)
			}
		}
		cc, cancel := context.WithCancel(ctx)
		cancel()
		if _, e := r.Classify(cc, geom.Vec3{}); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	}
}

func TestUnitSplineGraphQuery(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	build := func(h0, h1 float64) *geom.BSplineSurface {
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{Z: h0}, {Y: 1, Z: h0}, {X: 1, Z: h1}, {X: 1, Y: 1, Z: h1}}, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	ctx := context.Background()
	for _, x := range []struct {
		selection UnitGraphSelection
		a, b      *geom.BSplineSurface
		x         float64
		z         float64
	}{
		{UnitGraphFull, build(0, 0), build(1, 1), .125, .5},
		{UnitGraphAbove, build(-.5, .5), build(0, 0), .75, .125},
		{UnitGraphBelow, build(-.5, .5), build(0, 0), .25, -.125},
	} {
		q, e := NewUnitSplineGraphQuery(ctx, x.a, x.b, x.selection, intersect.SplinePlaneOptions{})
		if e != nil {
			t.Fatal(x.selection, e)
		}
		for _, tc := range []struct {
			p    geom.Vec3
			want parametric.Location
		}{
			{geom.Vec3{X: x.x, Y: .5, Z: x.z}, parametric.Inside},
			{geom.Vec3{X: x.x, Y: 1, Z: x.z}, parametric.Boundary},
			{geom.Vec3{X: x.x, Y: 1.001, Z: x.z}, parametric.Outside},
			{geom.Vec3{X: x.x, Y: .5, Z: 0}, parametric.Boundary},
			{geom.Vec3{X: x.x, Y: .5, Z: 2}, parametric.Outside},
		} {
			got, e := q.Classify(ctx, tc.p)
			if e != nil || got != tc.want {
				t.Fatal(x.selection, tc, got, e)
			}
		}
	}
	if _, e := NewUnitSplineGraphQuery(ctx, build(-.5, .5), build(0, 0), UnitGraphFull, intersect.SplinePlaneOptions{}); e == nil {
		t.Fatal("accepted crossing graphs as whole-chart material")
	}
}
