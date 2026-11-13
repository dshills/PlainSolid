package geom

import (
	"errors"
	"math"
	"testing"
)

func TestParabolaProjectionUniqueAndNonunique(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewParabola(f, 1)
	o := ConicProjectionOptions{Start: -2, End: 2}
	r, e := p.Project(Vec3{}, o)
	if e != nil || !r.Complete || !r.UniqueMinimum || len(r.Candidates) != 1 || r.Candidates[0].Parameter != 0 || r.SquaredDistanceMax != 0 {
		t.Fatal(r, e)
	}
	r, e = p.Project(Vec3{X: 3}, o)
	if e != nil || !r.Complete || r.UniqueMinimum || len(r.Candidates) != 2 || r.SquaredDistanceMin != 8 || r.SquaredDistanceMax != 8 {
		t.Fatal(r, e)
	}
	for _, c := range r.Candidates {
		if math.Abs(c.Parameter) != 1 {
			t.Fatal(c)
		}
	}
	r, e = p.Project(Vec3{}, ConicProjectionOptions{Start: .5, End: 2})
	if e != nil || !r.UniqueMinimum || !r.Candidates[0].Endpoint || r.Candidates[0].Parameter != .5 {
		t.Fatal(r, e)
	}
}
func TestHyperbolaProjectionBranches(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	for _, branch := range []int{-1, 1} {
		h, _ := NewHyperbolaBranch(f, 2, 1, branch)
		r, e := h.Project(Vec3{}, ConicProjectionOptions{Start: -.8, End: .8})
		if e != nil || !r.UniqueMinimum || r.Candidates[0].Parameter != 0 || r.SquaredDistanceMin != 4 || r.SquaredDistanceMax != 4 {
			t.Fatal(r, e)
		}
	}
}
func TestOpenConicProjectionIndependentDenseOracle(t *testing.T) {
	f, _ := NewFrame(Vec3{2, -1, 3}, Vec3{1, 2, 0}, Vec3{0, 0, 1})
	p, _ := NewParabola(f, .7)
	h, _ := NewHyperbola(f, 1.5, .8)
	query := Vec3{3, .2, 4}
	o := ConicProjectionOptions{Start: -.7, End: .75}
	for _, c := range []interface {
		Curve
		Project(Vec3, ConicProjectionOptions) (ConicProjection, error)
	}{p, h} {
		r, e := c.Project(query, o)
		if e != nil || !r.Complete || len(r.Candidates) == 0 {
			t.Fatal(r, e)
		}
		best := math.Inf(1)
		for i := 0; i <= 20000; i++ {
			u := o.Start + (o.End-o.Start)*float64(i)/20000
			point, _ := c.Eval(u)
			d := point.Sub(query).Dot(point.Sub(query))
			best = min(best, d)
		}
		if r.SquaredDistanceMin > best+1e-12 || best-r.SquaredDistanceMax > 1e-6 {
			t.Fatal(r, best)
		}
		if r.SquaredDistanceMax-r.SquaredDistanceMin > 1e-7 {
			t.Fatal(r)
		}
	}
}
func TestOpenConicProjectionLimits(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewParabola(f, 1)
	r, e := p.Project(Vec3{1, 1, 0}, ConicProjectionOptions{Start: -2, End: 2, MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	h, _ := NewHyperbola(f, 1, 1)
	if _, e := h.Project(Vec3{}, ConicProjectionOptions{Start: -1, End: .5}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
