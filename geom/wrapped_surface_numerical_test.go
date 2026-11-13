package geom

import (
	"context"
	"math"
	"sync"
	"testing"
)

func TestWrappedSurfaceNumericalCacheAndFallback(t *testing.T) {
	w, _, e := NewWrappedSurfacePatch(context.Background(), wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []float64{-100.25, -.25, 0, .125, 3.999, 4, 8.25, math.MaxFloat64, -math.MaxFloat64} {
		point, e := w.EvalApprox(u, .2)
		exact, f := w.Eval(u, .2)
		if e != nil || f != nil || point.Distance(exact) > 1e-12 {
			t.Fatal(u, point, exact, e, f)
		}
		du, dv, e := w.PartialsApprox(u, .2)
		eu, ev, f := w.Partials(u, .2)
		if e != nil || f != nil || du.Distance(eu) > 1e-12 || dv.Distance(ev) > 1e-12 {
			t.Fatal(u, du, eu, e, f)
		}
	}
	if w.numerical.support == nil {
		t.Fatal("lossless cache unavailable")
	}
	shifted, _, e := w.Transformed(context.Background(), Translation(Vec3{X: math.Ldexp(1, -80)}), WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point, e := shifted.EvalApprox(.125, .2)
	exact, f := shifted.Eval(.125, .2)
	if e != nil || f != nil || point != exact || shifted.numerical.support != nil {
		t.Fatal(point, exact, e, f)
	}
	// A nonbinary-period rectangle can yield an unrepresentable exact phase.
	var poles []Vec3
	for _, x := range []float64{0, 1, -1, 0} {
		for _, v := range []float64{0, 1} {
			poles = append(poles, Vec3{x, v, 0})
		}
	}
	s, _ := NewBSplineSurface(3, 1, 4, 2, poles, []float64{.1, .1, .1, .1, 1.1, 1.1, 1.1, 1.1}, []float64{0, 0, 1, 1}, nil)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	phase, _, e := NewWrappedSurfacePatch(context.Background(), p, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	_, represented, e := phase.fastCanonicalUV(2.1, .25)
	if e != nil || represented {
		t.Fatal(represented, e)
	}
	point, e = phase.EvalApprox(2.1, .25)
	exact, f = phase.Eval(2.1, .25)
	if e != nil || f != nil || point != exact {
		t.Fatal(point, exact, e, f)
	}
}
func TestWrappedSurfaceNumericalConcurrentInitialization(t *testing.T) {
	w, _, e := NewWrappedSurfacePatch(context.Background(), wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			surface := w.NumericalSurface()
			for j := 0; j < 8; j++ {
				if _, e := surface.Eval(float64(i)+.125, .25); e != nil {
					t.Error(e)
				}
				if _, _, e := surface.Partials(float64(j)-.125, .25); e != nil {
					t.Error(e)
				}
			}
		}(i)
	}
	workers.Wait()
}
