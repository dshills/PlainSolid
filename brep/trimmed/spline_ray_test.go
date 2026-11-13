package trimmed

import (
	"context"
	"errors"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestTrimmedSplineRay(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	q := [3]float64{.25, -.25, .25}
	const rr = .1171875
	build := func(lower bool) *geom.BSplineSurface {
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				z := 1.
				if lower {
					z += q[i] + q[j] - rr
				}
				p = append(p, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	ctx := context.Background()
	a, b := build(true), build(false)
	s, _, e := SealSplineGraphPair(ctx, a, b, 1e-7, intersect.SplinePlaneOptions{}, geom.SurfaceInjectivityOptions{})
	if e != nil {
		t.Fatal(e)
	}
	opt := SplineRayOptions{Line: geom.LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 2}}
	h, e := s.SplineRay(ctx, geom.Vec3{X: .5, Y: .5}, geom.Vec3{Z: 1}, opt)
	if e != nil || !h.Complete || len(h.Hits) != 2 || h.Hits[0].Direction != -1 || h.Hits[1].Direction != 1 {
		t.Fatal(h, e)
	}
	for i, z := range []float64{1 - rr, 1} {
		if h.Hits[i].ParameterMin > z || h.Hits[i].ParameterMax < z {
			t.Fatal(h)
		}
	}
	h, e = s.SplineRay(ctx, geom.Vec3{X: .2, Y: .2}, geom.Vec3{Z: 1}, opt)
	if e != nil || !h.Complete || len(h.Hits) != 0 {
		t.Fatal(h, e)
	}
	transform := geom.Affine{{-2, .5, 0, 3}, {0, 2, 0, 1}, {0, 0, 2, 1}}
	moved, e := s.Transformed(transform)
	if e != nil {
		t.Fatal(e)
	}
	worldOpt := opt
	worldOpt.Line.MaxParameter = 4
	h, e = moved.SplineRay(ctx, transform.Apply(geom.Vec3{X: .5, Y: .5}), geom.Vec3{Z: 1}, worldOpt)
	if e != nil || len(h.Hits) != 2 || h.Hits[0].Direction != -1 || h.Hits[1].Direction != 1 {
		t.Fatal(h, e)
	}
	for i, z := range []float64{2 * (1 - rr), 2} {
		if h.Hits[i].ParameterMin > z || h.Hits[i].ParameterMax < z {
			t.Fatal(h)
		}
	}
	h, e = s.SplineRay(ctx, geom.Vec3{X: .5, Y: .5}, geom.Vec3{Z: 1}, SplineRayOptions{
		Line: geom.LineSplineSurfaceOptions{MinParameter: 0, MaxParameter: 2, MaxCells: 1}})
	if !errors.Is(e, geom.ErrConvergence) || h.Complete || len(h.Issues) == 0 {
		t.Fatal(h, e)
	}
}

func TestSplineTrimBoxRefinement(t *testing.T) {
	// A parabolic bottom trim bows upward. Its control hull contains the
	// query, but the exact curve lies strictly below it. The chord-only
	// winding also differs for a point below the true curved trim.
	a, b := geom.Vec2{}, geom.Vec2{X: 1}
	c, e := geom.NewBezier([]geom.Vec3{{}, {X: .5, Y: .5}, {X: 1}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	top, left := geom.Vec2{X: 1, Y: 1}, geom.Vec2{Y: 1}
	f := Face{Outer: []Use{
		{UVStart: a, UVEnd: b, Curve: SplineUV{Curve: c, Range: UVInterval{Max: 1}}},
		{UVStart: b, UVEnd: top}, {UVStart: top, UVEnd: left}, {UVStart: left, UVEnd: a},
	}}
	for _, tc := range []struct {
		y    float64
		want parametric.Location
	}{{.375, parametric.Inside}, {.125, parametric.Outside}} {
		q := geom.UVBox{Min: geom.Vec2{X: .499, Y: tc.y - .001}, Max: geom.Vec2{X: .501, Y: tc.y + .001}}
		got, known, e := classifyTrimBox(context.Background(), f, q, 64)
		if e != nil || !known || got != tc.want {
			t.Fatal(tc, got, known, e)
		}
		if _, known, e = classifyTrimBox(context.Background(), f, q, 1); e != nil || known {
			t.Fatal(known, e)
		}
	}
}
