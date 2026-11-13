package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestApproximateScatteredBSplineSurfaceRational(t *testing.T) {
	ctx := context.Background()
	knots := []float64{0, 0, 1, 1}
	weights := []float64{1, .5, .25, 1}
	poles := []Vec3{{X: 1000, Y: -500, Z: 20}, {X: 1000, Y: -498, Z: 21}, {X: 1003, Y: -500, Z: 19}, {X: 1003, Y: -498, Z: 22}}
	want, e := NewBSplineSurface(1, 1, 2, 2, poles, knots, knots, weights)
	if e != nil {
		t.Fatal(e)
	}
	var samples []SurfaceSample
	for i, uv := range []Vec2{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {.3, .4}, {.9, .2}, {.05, .65}, {.45, .9}, {.73, .51}} {
		p, e := want.Eval(uv.X, uv.Y)
		if e != nil {
			t.Fatal(e)
		}
		samples = append(samples, SurfaceSample{UV: uv, Point: p, Weight: float64(i + 1)})
	}
	o := ScatteredSurfaceOptions{DegreeU: 1, DegreeV: 1, CountU: 2, CountV: 2, KnotsU: knots, KnotsV: knots, Weights: weights, MaxResidual: 1e-10}
	s, d, e := ApproximateScatteredBSplineSurface(ctx, samples, o)
	if e != nil || d.Rank != 4 || d.Samples != 9 || d.MaxResidual > 1e-10 {
		t.Fatal(d, e)
	}
	for _, uv := range []Vec2{{.1, .3}, {.7, .8}, {.65, .25}} {
		a, _ := s.Eval(uv.X, uv.Y)
		b, _ := want.Eval(uv.X, uv.Y)
		if a.Distance(b) > 1e-10 {
			t.Fatal(uv, a, b)
		}
	}
	// A zero-weight outlier does not pull the fit, but the residual cap
	// still rejects the complete data set if requested.
	samples = append(samples, SurfaceSample{UV: Vec2{.3, .4}, Point: Vec3{Z: 99}})
	if s, d, e := ApproximateScatteredBSplineSurface(ctx, samples, o); s != nil || !errors.Is(e, ErrConvergence) || d.MaxResidual < 1 || d.WeightedRMS > 1e-10 {
		t.Fatal(s, d, e)
	}
	o.MaxResidual = 0
	if _, d, e := ApproximateScatteredBSplineSurface(ctx, samples, o); e != nil || d.WeightedSamples != 9 || d.MaxResidual < 1 {
		t.Fatal(d, e)
	}
}

func TestApproximateScatteredBSplineSurfaceLimits(t *testing.T) {
	ctx := context.Background()
	knots := []float64{0, 0, 1, 1}
	o := ScatteredSurfaceOptions{DegreeU: 1, DegreeV: 1, CountU: 2, CountV: 2, KnotsU: knots, KnotsV: knots}
	s := []SurfaceSample{{UV: Vec2{0, 0}}, {UV: Vec2{1, 0}}, {UV: Vec2{0, 1}}, {UV: Vec2{1, 1}}}
	bad := o
	bad.KnotsV = nil
	if _, _, e := ApproximateScatteredBSplineSurface(ctx, s, bad); e == nil {
		t.Fatal("accepted missing knots")
	}
	bad = o
	bad.MaxWork = 1
	if _, _, e := ApproximateScatteredBSplineSurface(ctx, s, bad); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	bad = o
	bad.MaxResidual = math.Inf(1)
	if _, _, e := ApproximateScatteredBSplineSurface(ctx, s, bad); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	for i := range s {
		s[i].UV = Vec2{.5, .5}
	}
	if _, _, e := ApproximateScatteredBSplineSurface(ctx, s, o); !errors.Is(e, ErrIllConditioned) {
		t.Fatal("rank deficiency not rejected", e)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e := ApproximateScatteredBSplineSurface(ctx, s, o); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
