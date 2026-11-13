package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestExactSurfaceComparisonDegreeKnotsAndDomain(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	// Bilinear surface represented as biquadratic Bernstein coefficients.
	poles := make([]Vec3, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := float64(i)/2, float64(j)/2
			poles[i*3+j] = Vec3{u, v, u * v}
		}
	}
	s, _ := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	s, _ = s.InsertUKnot(.25, 1)
	s, _ = s.InsertVKnot(.5, 1)
	b, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	r, e := CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{})
	if e != nil || !r.Equal || r.ComparedCells != 4 {
		t.Fatal(r, e)
	}
	b, e = b.Reparameterize(ParameterRectangle{UMin: 2, UMax: 6, VMin: -4, VMax: 2}, false, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true})
	if e != nil || !r.Equal {
		t.Fatal(r, e)
	}
	_, e = CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
func TestExactSurfaceComparisonChartSymmetries(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, 2 * v, u * v} })
	for _, swap := range []bool{false, true} {
		for _, ru := range []bool{false, true} {
			for _, rv := range []bool{false, true} {
				b := arcTestPatch(t, func(u, v float64) Vec3 {
					// B's original coordinates map to aligned U,V after swapping and reversal.
					if swap {
						u, v = v, u
					}
					if ru {
						u = 1 - u
					}
					if rv {
						v = 1 - v
					}
					return Vec3{u, 2 * v, u * v}
				})
				r, e := CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{SwapB: swap, ReverseBU: ru, ReverseBV: rv})
				if e != nil || !r.Equal {
					t.Fatal(swap, ru, rv, r, e)
				}
			}
		}
	}
}
func TestExactSurfaceComparisonNegativeAndLimits(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, math.Ldexp(1, -100) * u * v} })
	r, e := CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{})
	if e != nil || !r.Complete || r.Equal || r.DifferentAxis != 2 {
		t.Fatal(r, e)
	}
	r, e = CompareExactSurfaces(context.Background(), a, a, ExactSurfaceComparisonOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = CompareExactSurfaces(ctx, a, a, ExactSurfaceComparisonOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}

func TestExactSurfaceComparisonRationalMultiplierAndNonbinaryGrid(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	s := &BSplineSurface{degreeU: 2, degreeV: 1, countU: 3, countV: 2, knotsU: []float64{0, 0, 0, 1, 1, 1}, knotsV: []float64{0, 0, 1, 1}, weights: []float64{1, 1, 2, 2, 3, 3}, poles: []Vec3{{0, 0, 0}, {0, 1, 0}, {.25, 0, 0}, {.25, 1, .25}, {1, 0, 0}, {1, 1, 1}}}
	b, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{})
	if e != nil || !r.Equal {
		t.Fatal(r, e)
	}
	a = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{3 * u, v, 3 * u * v} })
	s, e = NewBSplineSurface(1, 1, 3, 2, []Vec3{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}, {1, 1, 1}, {3, 0, 0}, {3, 1, 3}}, []float64{2, 2, 3, 5, 5}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = NewSurfacePatch(s, SurfacePatchOptions{})
	r, e = CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true})
	if e != nil || !r.Equal || r.ComparedCells != 2 {
		t.Fatal(r, e)
	}
	r, e = CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true, MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
}
