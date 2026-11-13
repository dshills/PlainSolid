package geom

import (
	"errors"
	"math/rand"
	"testing"
)

func TestSurfaceKnotRemovalRationalRoundTrip(t *testing.T) {
	source := quarterCylinder(t)
	refined, e := source.InsertUKnot(.37, 2)
	if e != nil {
		t.Fatal(e)
	}
	refined, e = refined.InsertVKnot(.61, 1)
	if e != nil {
		t.Fatal(e)
	}
	a, d, e := refined.RemoveUKnot(.37, 2, KnotRemovalOptions{})
	if e != nil || d.Removed != 2 || d.GlobalErrorBound > 1e-9 {
		t.Fatal(d, e)
	}
	b, dv, e := a.RemoveVKnot(.61, 1, KnotRemovalOptions{})
	if e != nil || dv.Removed != 1 || dv.GlobalErrorBound > 1e-9 {
		t.Fatal(dv, e)
	}
	rng := rand.New(rand.NewSource(710))
	for i := 0; i < 200; i++ {
		u, v := rng.Float64(), rng.Float64()
		p, _ := source.Eval(u, v)
		q, _ := b.Eval(u, v)
		r, _ := refined.Eval(u, v)
		if p.Distance(q) > 1e-12 || r.Distance(q) > d.GlobalErrorBound+dv.GlobalErrorBound+1e-14 {
			t.Fatal(p, q, r, d, dv)
		}
	}
}
func TestSurfaceKnotRemovalIndependentWeights(t *testing.T) {
	rng := rand.New(rand.NewSource(221))
	poles := make([]Vec3, 9)
	weights := make([]float64, 9)
	for i := range poles {
		poles[i] = Vec3{rng.Float64() * 3, rng.Float64(), rng.Float64()}
		weights[i] = .4 + rng.Float64()*2
	}
	s, e := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, weights)
	if e != nil {
		t.Fatal(e)
	}
	r, _ := s.InsertVKnot(.5, 2)
	q, d, e := r.RemoveVKnot(.5, 2, KnotRemovalOptions{})
	if e != nil {
		t.Fatal(d, e)
	}
	for i := 0; i < 100; i++ {
		u, v := rng.Float64(), rng.Float64()
		p, _ := r.Eval(u, v)
		point, _ := q.Eval(u, v)
		if p.Distance(point) > d.GlobalErrorBound+1e-14 {
			t.Fatal(p, point, d)
		}
	}
	// Independent exact tensor Bernstein oracle on the pre-insertion surface.
	for _, uv := range [][2]float64{{.125, .25}, {.5, .75}, {.875, .125}} {
		exact := exactBezierSurfacePoint(s, uv[0], uv[1])
		point, _ := q.Eval(uv[0], uv[1])
		for axis := 0; axis < 3; axis++ {
			value, _ := exact[axis].Float64()
			delta := coordinate(point, axis) - value
			if delta > 1e-12 || delta < -1e-12 {
				t.Fatal(point, exact)
			}
		}
	}
}
func TestSurfaceKnotRemovalRejectsShapeAndLimits(t *testing.T) {
	poles := []Vec3{{0, 0, 0}, {0, 1, 0}, {.5, 0, 1}, {.5, 1, 0}, {1, 0, 0}, {1, 1, 0}}
	s, _ := NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	before, _ := s.Eval(.5, 0)
	q, d, e := s.RemoveUKnot(.5, 1, KnotRemovalOptions{})
	if !errors.Is(e, ErrKnotRemoval) || q != nil || d.Removed != 0 {
		t.Fatal(q, d, e)
	}
	after, _ := s.Eval(.5, 0)
	if before != after {
		t.Fatal("source mutated")
	}
	s, _ = quarterCylinder(t).InsertUKnot(.5, 1)
	for _, o := range []KnotRemovalOptions{{MaxExactOperations: 1}, {MaxWork: 1}, {MaxPoles: 4}} {
		q, d, e = s.RemoveUKnot(.5, 1, o)
		if !errors.Is(e, ErrConvergence) || q != nil || d.Removed != 0 {
			t.Fatal(q, d, e)
		}
	}
}
