package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func locationToroid(t *testing.T, circular bool) *WrappedSurfacePatch {
	t.Helper()
	q := [][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	weights := []float64{1, .75, 1, .75, 1, .75, 1, .75, 1}
	if circular {
		weights = []float64{1, 1, 2, 2, 4, 2, 2, 1, 1}
	}
	var poles []Vec3
	var ws []float64
	for i, u := range q {
		for j, v := range q {
			poles = append(poles, Vec3{X: (2 + .5*v[0]) * u[0], Y: (2 + .5*v[0]) * u[1], Z: .5 * v[1]})
			ws = append(ws, weights[i]*weights[j])
		}
	}
	k := []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}
	s, e := NewBSplineSurface(2, 2, 9, 9, poles, k, k, ws)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := NewWrappedSurfacePatch(context.Background(), p, WrappedSurfaceOptions{WrapU: true, WrapV: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	return w
}

func TestExactToroidMeridianMaterial(t *testing.T) {
	ctx := context.Background()
	w := locationToroid(t, false)
	for _, tc := range []struct {
		p    Vec3
		want ToroidMeridianLocation
	}{
		{Vec3{}, ToroidMeridianOutside}, {Vec3{X: 2}, ToroidMeridianInside}, {Vec3{Y: -2}, ToroidMeridianInside},
		{Vec3{X: 2.5}, ToroidMeridianBoundary}, {Vec3{X: -1.5}, ToroidMeridianBoundary}, {Vec3{Y: 2, Z: .5}, ToroidMeridianBoundary},
		{Vec3{X: math.Nextafter(2.5, 2)}, ToroidMeridianInside}, {Vec3{X: math.Nextafter(2.5, 3)}, ToroidMeridianOutside},
		{Vec3{X: 2, Z: -.5}, ToroidMeridianBoundary}, {Vec3{X: 2, Z: .6}, ToroidMeridianOutside},
	} {
		got, e := LocateWrappedToroidMeridian(ctx, w, tc.p, SurfacePatchOptions{})
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}

	for _, sx := range []float64{-1, 1} {
		for _, sz := range []float64{-1, 1} {
			for _, tc := range []struct {
				q    float64
				want ToroidMeridianLocation
			}{{.75, ToroidMeridianOutside}, {.6875, ToroidMeridianInside}} {
				p := Vec3{X: 2 + sx*.5*tc.q, Z: sz * .5 * tc.q}
				got, e := LocateWrappedToroidMeridian(ctx, w, p, SurfacePatchOptions{})
				if e != nil || got != tc.want {
					t.Fatal("quadrant conic sign", p, got, e)
				}
			}
		}
	}
	if _, e := LocateWrappedToroidMeridian(ctx, w, Vec3{X: 1, Y: 1}, SurfacePatchOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	if got, e := LocateWrappedToroidMeridian(ctx, w, Vec3{X: 2}, SurfacePatchOptions{MaxOperations: 1}); e == nil || got != ToroidMeridianUnknown {
		t.Fatal("budget", got, e)
	}
}

func TestExactCircularToroidMaterialThreeDimensions(t *testing.T) {
	ctx := context.Background()
	w := locationToroid(t, true)
	for _, tc := range []struct {
		p    Vec3
		want ToroidMeridianLocation
	}{
		{Vec3{}, ToroidMeridianOutside}, {Vec3{X: 1.2, Y: 1.6}, ToroidMeridianInside},
		// Binary-exact 3-4-5 outer radius with nonaxial coordinates.
		{Vec3{X: 1.5, Y: 2}, ToroidMeridianBoundary},
		{Vec3{X: 1.5, Y: 2, Z: math.SmallestNonzeroFloat64}, ToroidMeridianOutside},
		{Vec3{X: 1, Y: 1}, ToroidMeridianOutside}, {Vec3{X: -1.5, Y: -2}, ToroidMeridianBoundary},
	} {
		got, e := LocateWrappedCircularToroid(ctx, w, tc.p, SurfacePatchOptions{})
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	if got, e := LocateWrappedCircularToroid(ctx, locationToroid(t, false), Vec3{X: 2}, SurfacePatchOptions{}); !errors.Is(e, ErrConvergence) || got != ToroidMeridianUnknown {
		t.Fatal("noncircle admitted", got, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := LocateWrappedCircularToroid(canceled, w, Vec3{}, SurfacePatchOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
