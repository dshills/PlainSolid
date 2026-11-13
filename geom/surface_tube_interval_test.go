package geom

import (
	"context"
	"errors"
	"testing"
)

func TestVerifiedTubeIntervalAffine(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	a, _ = a.Trim(ParameterRectangle{UMin: .2, UMax: .8, VMin: 0, VMax: 1})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .4, t - .3} })
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := tube.EncloseInterval(context.Background(), .3, .6, SurfaceTubeIntervalOptions{})
	if e != nil || !r.HasBounds || !r.Complete || r.ParametersA.Min.X != .3 || r.ParametersA.Max.X != .6 {
		t.Fatal(r, e)
	}
	for _, x := range []float64{.3, .4, .5, .6} {
		if !r.Bounds.Contains(Vec3{x, .4, 0}) || r.ParametersB.Min.X > x || r.ParametersB.Max.X < x || r.ParametersA.Min.Y > .4 || r.ParametersA.Max.Y < .4 {
			t.Fatal(r, x)
		}
	}
	swapped, e := NewVerifiedSurfaceTube(b, a, SurfacePairTubeOptions{FreeAxis: 2})
	if e != nil {
		t.Fatal(e)
	}
	q, e := swapped.EncloseInterval(context.Background(), .3, .6, SurfaceTubeIntervalOptions{})
	if e != nil || q.Bounds != r.Bounds || q.ParametersA != r.ParametersB || q.ParametersB != r.ParametersA {
		t.Fatal(q, r, e)
	}
}
func TestVerifiedTubeIntervalRationalAndOneIteration(t *testing.T) {
	source := quarterCylinder(t)
	a, _ := NewSurfacePatch(source, SurfacePatchOptions{})
	a, _ = a.Trim(ParameterRectangle{UMin: .25, UMax: .35, VMin: 0, VMax: 1})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{2 * s, 2 * t, .75} })
	tube, e := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := tube.EncloseInterval(context.Background(), .28, .32, SurfaceTubeIntervalOptions{MaxIterations: 1, ParameterTolerance: 1e-12, BoxTolerance: 1e-12})
	if !errors.Is(e, ErrConvergence) || !r.HasBounds || r.Complete || r.Iterations != 1 {
		t.Fatal(r, e)
	}
	for _, u := range []float64{.28, .29, .3, .31, .32} {
		point, _ := source.Eval(u, .375)
		point.Z = .75 // exact section height; ordinary Eval has its own rounding error
		if !r.Bounds.Contains(point) || r.ParametersA.Min.Y > .375 || r.ParametersA.Max.Y < .375 || r.ParametersB.Min.X > point.X/2 || r.ParametersB.Max.X < point.X/2 || r.ParametersB.Min.Y > point.Y/2 || r.ParametersB.Max.Y < point.Y/2 {
			t.Fatal(r, u, point)
		}
	}
	if r.ParametersB.Max.X-r.ParametersB.Min.X >= 1 {
		t.Fatal("failed to contract", r)
	}
}
func TestVerifiedTubeIntervalLimitsAndCancellation(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	a, _ = a.Trim(ParameterRectangle{UMin: .2, UMax: .8, VMin: 0, VMax: 1})
	b := arcTestPatch(t, func(s, t float64) Vec3 { return Vec3{s, .4, t - .3} })
	tube, _ := NewVerifiedSurfaceTube(a, b, SurfacePairTubeOptions{})
	r, e := tube.EncloseInterval(distanceCancelContext(2), .3, .6, SurfaceTubeIntervalOptions{})
	if !errors.Is(e, context.Canceled) || !r.HasBounds || r.Complete {
		t.Fatal(r, e)
	}
	_, e = tube.EncloseInterval(context.Background(), .1, .6, SurfaceTubeIntervalOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	_, e = tube.EncloseInterval(context.Background(), .4, .4, SurfaceTubeIntervalOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	r, e = tube.EncloseInterval(context.Background(), .3, .6, SurfaceTubeIntervalOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	var empty VerifiedSurfaceTube
	_, e = empty.EncloseInterval(context.Background(), 0, 1, SurfaceTubeIntervalOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
