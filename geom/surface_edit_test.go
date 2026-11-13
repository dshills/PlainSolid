package geom

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

func TestRandomRationalSurfaceSplits(t *testing.T) {
	random := rand.New(rand.NewSource(97291))
	for trial := 0; trial < 30; trial++ {
		poles := make([]Vec3, 9)
		weights := make([]float64, 9)
		for i := range poles {
			poles[i] = Vec3{random.Float64() * 10, random.Float64() * 10, random.Float64() * 10}
			weights[i] = math.Pow(10, random.Float64()*4-2)
		}
		surface, e := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, weights)
		if e != nil {
			t.Fatal(e)
		}
		left, right, e := surface.SplitU(.37)
		if e != nil {
			t.Fatal(e)
		}
		bottom, top, e := surface.SplitV(.64)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i <= 10; i++ {
			for j := 0; j <= 10; j++ {
				u, v := float64(i)/10, float64(j)/10
				want, _ := surface.Eval(u, v)
				us := left
				if u >= .37 {
					us = right
				}
				vs := bottom
				if v >= .64 {
					vs = top
				}
				a, e := us.Eval(u, v)
				if e != nil {
					t.Fatal(e)
				}
				b, e := vs.Eval(u, v)
				if e != nil {
					t.Fatal(e)
				}
				near(t, a, want, 1e-12)
				near(t, b, want, 1e-12)
			}
		}
	}
}
func quarterCylinder(t *testing.T) *BSplineSurface {
	t.Helper()
	w := math.Sqrt(.5)
	s, e := NewBSplineSurface(2, 1, 3, 2, []Vec3{{2, 0, 0}, {2, 0, 2}, {2, 2, 0}, {2, 2, 2}, {0, 2, 0}, {0, 2, 2}}, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, w, w, 1, 1})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestSplineSurfaceProjection(t *testing.T) {
	surface, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 3, 0}, {4, 0, 0}, {4, 3, 0}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	result, e := surface.Project(Vec3{1.2, 2.1, 2}, SplineSurfaceProjectionOptions{})
	if e != nil || math.Abs(result.Distance-2) > 1e-6 || result.ErrorBound > 1e-6 || result.LowerBound > 2+1e-12 {
		t.Fatal(result, e)
	}
	surface = quarterCylinder(t)
	for _, tt := range []struct {
		p        Vec3
		distance float64
	}{{Vec3{3, 4, 1}, 3}, {Vec3{3, 4, 3}, math.Sqrt(10)}, {Vec3{1, 0, 1}, 1}} {
		result, e = surface.Project(tt.p, SplineSurfaceProjectionOptions{})
		if e != nil || math.Abs(result.Distance-tt.distance) > 1e-6 || result.ErrorBound > 1e-6 || result.LowerBound > tt.distance+1e-12 {
			t.Fatal(tt, result, e)
		}
		point, _ := surface.Eval(result.U, result.V)
		near(t, result.Point, point, 1e-12)
	}
	result, e = surface.Project(Vec3{3, 4, 1}, SplineSurfaceProjectionOptions{DistanceTolerance: 1e-14, MaxSplits: 1})
	if !errors.Is(e, ErrConvergence) || !finite(result.Distance) {
		t.Fatal(result, e)
	}
	if _, e = surface.Project(Vec3{}, SplineSurfaceProjectionOptions{MaxQueuedPoles: 4}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
func TestSurfaceSplitErrors(t *testing.T) {
	surface := quarterCylinder(t)
	for _, parameter := range []float64{-1, 0, 1, 2, math.NaN()} {
		if _, _, e := surface.SplitU(parameter); e == nil {
			t.Fatal(parameter)
		}
		if _, _, e := surface.SplitV(parameter); e == nil {
			t.Fatal(parameter)
		}
	}
	var zero BSplineSurface
	if _, _, e := zero.SplitU(.5); e == nil {
		t.Fatal("zero")
	}
}
