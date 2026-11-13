package geom

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestExactSplineCurveRestriction(t *testing.T) {
	source, _ := NewBezier([]Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{1, math.Sqrt(.5), 1})
	source, _ = source.InsertKnot(.4, 1)
	c, e := NewExactSplineCurve(source, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	restricted, e := c.Restrict(.2, .8, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	lo, hi := restricted.Domain()
	if lo != .2 || hi != .8 {
		t.Fatal(lo, hi)
	}
	axis, fixed := restricted.FixedParameter()
	if axis != -1 || !math.IsNaN(fixed) {
		t.Fatal(axis, fixed)
	}
	for _, u := range []float64{.2, .3, .4, .5, .8} {
		budget := &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
		a, e := exactPatchPointAt(c.patch, u, 0, budget)
		if e != nil {
			t.Fatal(e)
		}
		b, e := exactPatchPointAt(restricted.patch, u, 0, budget)
		if e != nil {
			t.Fatal(e)
		}
		for i := range a {
			if a[i].Cmp(b[i]) != 0 {
				t.Fatal(u, i, a, b)
			}
		}
		if u > .2 && u < .8 {
			d, _ := c.Derivative(u)
			q, e := restricted.Derivative(u)
			if e != nil {
				t.Fatal(e)
			}
			near(t, d, q, 1e-14)
		}
	}
	containsKnot := false
	for _, k := range restricted.patch.knotsU {
		if k == .4 {
			containsKnot = true
		}
	}
	if !containsKnot {
		t.Fatal("lost knot")
	}
	// The new rational coefficients include nonbinary values rather than rounded poles.
	nonbinary := false
	for _, h := range restricted.patch.net {
		for _, x := range h {
			den := x.Denom()
			if new(big.Int).And(new(big.Int).Set(den), new(big.Int).Sub(new(big.Int).Set(den), big.NewInt(1))).Sign() != 0 {
				nonbinary = true
			}
		}
	}
	if !nonbinary {
		t.Fatal("expected exact nonbinary knot-refinement coefficients")
	}
}
func TestExactSplineCurveSplitAndLimits(t *testing.T) {
	source, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {3, 1, 0}}, []float64{1, 2, 1})
	c, _ := NewExactSplineCurve(source, SurfacePatchOptions{})
	a, b, e := c.Split(.37, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	left, right := a.patch.net[(a.patch.countU-1)*2], b.patch.net[0]
	for i := 0; i < 3; i++ {
		if new(big.Rat).Mul(left[i], right[3]).Cmp(new(big.Rat).Mul(right[i], left[3])) != 0 {
			t.Fatal(left, right)
		}
	}
	if a.ExactOperations() != b.ExactOperations() {
		t.Fatal("shared budget")
	}
	_, _, e = c.Split(0, SurfacePatchOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	_, e = c.Restrict(-1, .5, SurfacePatchOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	_, e = c.Restrict(.5, .5, SurfacePatchOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	_, e = c.Restrict(.1, .9, SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	_, e = NewExactSplineCurve(source, SurfacePatchOptions{MaxPoles: 4})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	p, e := c.Eval(.5)
	if e != nil {
		t.Fatal(e)
	}
	expected, _ := source.Eval(.5)
	near(t, p, expected, 1e-14)
}
