package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func queryCylinder(t *testing.T, circular bool) *WrappedSurfacePatch {
	t.Helper()
	q := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	weights := []float64{1, .75, 1, .75, 1, .75, 1, .75, 1}
	if circular {
		weights = []float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	}
	var poles []Vec3
	var ws []float64
	for i, p := range q {
		for _, z := range []float64{-1, 2} {
			poles = append(poles, Vec3{X: p[0], Y: p[1], Z: z})
			ws = append(ws, weights[i])
		}
	}
	s, e := NewBSplineSurface(2, 1, 9, 2, poles, []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}, []float64{0, 0, 1, 1}, ws)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := NewWrappedSurfacePatch(context.Background(), p, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return w
}
func TestWrappedCylinderExactQueries(t *testing.T) {
	ctx := context.Background()
	w := queryCylinder(t, true)
	for _, tc := range []struct {
		p    Vec3
		want CylinderLocation
	}{{Vec3{}, CylinderInside}, {Vec3{Z: -1}, CylinderBoundary}, {Vec3{X: 1}, CylinderBoundary}, {Vec3{Z: 3}, CylinderOutside}, {Vec3{X: 1, Y: .125}, CylinderOutside}} {
		got, e := LocateWrappedCylinder(ctx, w, tc.p, SurfacePatchOptions{})
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	rational := queryCylinder(t, false)
	for _, q := range []float64{.6875, .75} {
		got, e := LocateWrappedCylinder(ctx, rational, Vec3{X: q, Y: q}, SurfacePatchOptions{})
		want := CylinderInside
		if q == .75 {
			want = CylinderOutside
		}
		if e != nil || got != want {
			t.Fatal(got, e)
		}
	}
	o := ToroidLineOptions{ParameterTolerance: 1e-10}
	for _, tc := range []struct {
		p, d           Vec3
		hits, overlaps int
	}{{Vec3{}, Vec3{X: 1}, 2, 0}, {Vec3{}, Vec3{Z: 1}, 2, 0}, {Vec3{X: 1}, Vec3{Z: 1}, 2, 1}, {Vec3{Z: 2}, Vec3{X: 1}, 2, 1}, {Vec3{X: 1, Z: 2}, Vec3{Y: 1}, 1, 0}, {Vec3{X: 2}, Vec3{Z: 1}, 0, 0}} {
		r, e := IntersectWrappedCircularCylinderLine(ctx, w, tc.p, tc.d, -3, 3, o)
		if e != nil || len(r.Hits) != tc.hits || len(r.Overlaps) != tc.overlaps {
			t.Fatal(tc, r, e)
		}
	}
	r, e := IntersectWrappedCircularCylinderLine(ctx, w, Vec3{}, Vec3{X: 1, Z: 2}, -3, 3, o)
	if e != nil {
		t.Fatal(e)
	}
	// lower cap at t=-1/2, upper rim at t=1 combines cap and side identity.
	if len(r.Hits) != 2 || r.Hits[0].Parameter.Min.Cmp(big.NewRat(-1, 2)) != 0 || r.Hits[1].Parts != (CylinderLateral|CylinderUpperCap) {
		t.Fatal("rim ancestry", r)
	}
	r, e = IntersectWrappedCircularCylinderLine(ctx, w, Vec3{Z: 2}, Vec3{X: 1}, -3, 3, o)
	if e != nil || r.Overlaps[0].Start.Min.Cmp(big.NewRat(-1, 1)) > 0 || r.Overlaps[0].End.Max.Cmp(big.NewRat(1, 1)) < 0 {
		t.Fatal("cap overlap", r, e)
	}
	bad := o
	bad.Surface.MaxOperations = 1
	r, e = IntersectWrappedCircularCylinderLine(ctx, w, Vec3{}, Vec3{X: 1}, -3, 3, bad)
	if e == nil || len(r.Hits) != 0 || len(r.Overlaps) != 0 {
		t.Fatal("budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = IntersectWrappedCircularCylinderLine(canceled, w, Vec3{}, Vec3{X: 1}, -3, 3, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
