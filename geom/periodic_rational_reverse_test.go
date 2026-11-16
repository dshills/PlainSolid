package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactReverseRetainsOriginalLeftJet(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchOptions{}
	knots := []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 1)}
	net := make([][4]*big.Rat, 3)
	for i, p := range [][3]int64{{0, 0, 0}, {2, 0, 0}, {2, 2, 0}} {
		net[i] = [4]*big.Rat{big.NewRat(p[0], 1), big.NewRat(p[1], 1), big.NewRat(p[2], 1), big.NewRat(1, 1)}
	}
	c, err := NewExactPeriodicSplineCurve(ctx, 1, knots, net, o)
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.Reverse(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		x      *big.Rat
		px, py int64
		dx, dy int64
	}{{new(big.Rat), 0, 0, 4, 4}, {big.NewRat(2, 3), 2, 0, -6, 0}} {
		j, err := r.EvaluateRational(ctx, tc.x, o)
		if err != nil || j.Point[0].Cmp(big.NewRat(tc.px, 1)) != 0 || j.Point[1].Cmp(big.NewRat(tc.py, 1)) != 0 ||
			j.Derivative[0].Cmp(big.NewRat(tc.dx, 1)) != 0 || j.Derivative[1].Cmp(big.NewRat(tc.dy, 1)) != 0 {
			t.Fatal("reversal used the old right derivative at a crease", tc, j, err)
		}
	}
	twice, err := r.Reverse(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	assertCyclicRationalNet(t, c.Knots(), twice.Knots(), c.HomogeneousControls(), twice.HomogeneousControls())
}

func TestPeriodicExactTensorReverseRepeatedSeam(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchOptions{}
	d := BSplineSurfaceData{DegreeU: 2, DegreeV: 1, CountU: 4, CountV: 3, KnotsV: []float64{0, 0, .5, 1, 1}}
	for u := range d.CountU {
		for v := range d.CountV {
			d.Poles = append(d.Poles, Vec3{float64(u), float64(v), float64((u + v*v) % 5)})
			d.Weights = append(d.Weights, float64(1+(u+v)%5))
		}
	}
	base, err := NewPeriodicBSplineSurface(d, &PeriodicSplineAxis{Start: -1, Period: 3}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := base.ExactCyclicSurface(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.InsertPeriodicKnot(ctx, 0, big.NewRat(-1, 1), o)
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.InsertPeriodicKnot(ctx, 0, big.NewRat(1, 7), o)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.ReversePeriodicDirection(ctx, 0, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []*big.Rat{big.NewRat(-1, 1), big.NewRat(0, 1), big.NewRat(6, 7), big.NewRat(-49, 11)} {
		v := big.NewRat(9, 16)
		original := new(big.Rat).Sub(big.NewRat(1, 1), u)
		a, err := s.EvaluateRational(ctx, original, v, o)
		if err != nil {
			t.Fatal(err)
		}
		z, err := r.EvaluateRational(ctx, u, v, o)
		if err != nil {
			t.Fatal(err)
		}
		for i := range a.Point {
			if z.Point[i].Cmp(a.Point[i]) != 0 || z.Du[i].Cmp(new(big.Rat).Neg(a.Du[i])) != 0 || z.Dv[i].Cmp(a.Dv[i]) != 0 {
				t.Fatal("reflected periodic parameter changed the physical law", u, i)
			}
		}
	}
	twice, err := r.ReversePeriodicDirection(ctx, 0, o)
	if err != nil {
		t.Fatal(err)
	}
	a, b, h := s.Data()
	ua, va, controls := twice.Data()
	assertCyclicRationalNet(t, a.Knots, ua.Knots, h, controls)
	assertCyclicRationalNet(t, b.Knots, va.Knots, h, controls)
	if out, err := r.ReversePeriodicDirection(ctx, 1, o); out != nil || !errors.Is(err, ErrDomain) {
		t.Fatal("ordinary axis admitted periodic edit", err)
	}
}
