package geom

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSurfaceIsoCurveExactRational(t *testing.T) {
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}, {1, 1, 0}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, 2, 2})
	if e != nil {
		t.Fatal(e)
	}
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	c, e := p.IsoCurve(0, .5, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, h := range c.patch.net {
		if new(big.Rat).Quo(h[0], h[3]).Cmp(big.NewRat(2, 3)) != 0 {
			t.Fatal(h)
		}
	}
	plane, _ := NewPlane(Vec3{2.0 / 3, 0, 0}, Vec3{1, 0, 0})
	r, e := c.BoundPlaneRange(plane, SurfacePlaneRangeOptions{})
	if e != nil || !r.Complete || r.StrictSign != 1 || r.Minimum.Min <= 0 {
		t.Fatal(r, e)
	}
	for _, v := range []float64{0, .3, 1} {
		j, e := c.EvaluateJet(v)
		if e != nil {
			t.Fatal(e)
		}
		near(t, j.Point, Vec3{2.0 / 3, v, 0}, 1e-15)
		near(t, j.Derivative, Vec3{0, 1, 0}, 1e-15)
	}
}
func TestSurfaceIsoCurveDirectionsEndpointsKnots(t *testing.T) {
	s := quarterCylinder(t)
	s, e := s.InsertUKnot(.4, 1)
	if e != nil {
		t.Fatal(e)
	}
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	for axis := 0; axis < 2; axis++ {
		for _, fixed := range []float64{0, .4, .7, 1} {
			c, e := p.IsoCurve(axis, fixed, SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for _, q := range []float64{0, .2, .4, .8, 1} {
				u, v := fixed, q
				if axis == 1 {
					u, v = q, fixed
				}
				point, e := c.Eval(q)
				if e != nil {
					t.Fatal(e)
				}
				expected, e := p.Eval(u, v)
				if e != nil {
					t.Fatal(e)
				}
				near(t, point, expected, 1e-14)
				derivative, e := c.Derivative(q)
				if e != nil {
					t.Fatal(e)
				}
				du, dv, e := p.Partials(u, v)
				if e != nil {
					t.Fatal(e)
				}
				expected = dv
				if axis == 1 {
					expected = du
				}
				near(t, derivative, expected, 1e-13)
			}
			lo, hi := c.Domain()
			if lo != 0 || hi != 1 {
				t.Fatal(lo, hi)
			}
			a, f := c.FixedParameter()
			if a != axis || f != fixed {
				t.Fatal(a, f)
			}
		}
	}
	if _, e := p.IsoCurve(2, .5, SurfacePatchOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := p.IsoCurve(0, -1, SurfacePatchOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	if _, e := p.IsoCurve(0, .5, SurfacePatchOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	var c *SurfaceIsoCurve
	lo, _ := c.Domain()
	if !math.IsNaN(lo) {
		t.Fatal(lo)
	}
}
