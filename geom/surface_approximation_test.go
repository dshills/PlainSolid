package geom

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

func TestSurfaceApproximationPolynomialGrid(t *testing.T) {
	params := []float64{0, .1, .3, .6, .8, 1}
	points := []Vec3{}
	for _, u := range params {
		for _, v := range params {
			points = append(points, Vec3{u, v, u*u + 2*u*v + 3*v*v})
		}
	}
	o := SurfaceApproximationOptions{U: ApproximationOptions{Degree: 2, PoleCount: 3, Parameters: params}, V: ApproximationOptions{Degree: 2, PoleCount: 3, Parameters: params}}
	s, d, e := ApproximateBSplineSurface(points, 6, 6, o)
	if e != nil || d.MaxResidual > 1e-13 {
		t.Fatal(d, e)
	}
	for i := 0; i <= 20; i++ {
		u, v := float64(i)/20, float64(20-i)/20
		p, e := s.Eval(u, v)
		if e != nil || p.Distance(Vec3{u, v, u*u + 2*u*v + 3*v*v}) > 1e-13 {
			t.Fatal(p, e)
		}
	}
}
func TestSurfaceApproximationInterpolationAndNoise(t *testing.T) {
	params := []float64{0, .3, .7, 1}
	points := []Vec3{}
	rng := rand.New(rand.NewSource(17))
	for _, u := range params {
		for _, v := range params {
			points = append(points, Vec3{u, v, u*v + .001*rng.NormFloat64()})
		}
	}
	o := SurfaceApproximationOptions{U: ApproximationOptions{Degree: 3, PoleCount: 4, Parameters: params}, V: ApproximationOptions{Degree: 3, PoleCount: 4, Parameters: params}}
	_, d, e := ApproximateBSplineSurface(points, 4, 4, o)
	if e != nil || d.MaxResidual > 1e-13 {
		t.Fatal(d, e)
	}
	o.U.Degree, o.V.Degree, o.U.PoleCount, o.V.PoleCount = 1, 1, 2, 2
	_, d, e = ApproximateBSplineSurface(points, 4, 4, o)
	if e != nil || d.RMSResidual <= 0 || d.MaxResidual > .01 {
		t.Fatal(d, e)
	}
	o.MaxResidual = 1e-8
	s, d, e := ApproximateBSplineSurface(points, 4, 4, o)
	if !errors.Is(e, ErrConvergence) || s != nil || d.MaxResidual <= o.MaxResidual {
		t.Fatal(s, d, e)
	}
}
func TestSurfaceApproximationRationalAndLimits(t *testing.T) {
	source := quarterCylinder(t)
	params := []float64{0, .2, .5, .8, 1}
	points := []Vec3{}
	for _, u := range params {
		for _, v := range params {
			p, _ := source.Eval(u, v)
			points = append(points, p)
		}
	}
	o := SurfaceApproximationOptions{U: ApproximationOptions{Degree: 2, PoleCount: 3, Parameters: params, Weights: []float64{1, math.Sqrt(.5), 1}}, V: ApproximationOptions{Degree: 1, PoleCount: 2, Parameters: params}}
	s, d, e := ApproximateBSplineSurface(points, 5, 5, o)
	if e != nil || d.MaxResidual > 1e-13 {
		t.Fatal(d, e)
	}
	p, _ := s.Eval(.37, .61)
	q, _ := source.Eval(.37, .61)
	if p.Distance(q) > 1e-13 {
		t.Fatal(p, q)
	}
	o.MaxWork = 1
	if s, _, e := ApproximateBSplineSurface(points, 5, 5, o); !errors.Is(e, ErrConvergence) || s != nil {
		t.Fatal(s, e)
	}
	o.MaxWork = 0
	o.U.Parameters = []float64{0, 0, 0, 0, 1}
	if s, _, e := ApproximateBSplineSurface(points, 5, 5, o); e == nil || s != nil {
		t.Fatal("singular fit accepted")
	}
}
