package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactDegreeElevationRetainsContinuityOrders(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchOptions{}
	source, err := NewPeriodicBSplineWithExactWeights(2,
		[]Vec3{{1, 0, 0}, {3, 2, 1}, {2, -1, 3}, {-1, 1, 2}, {-2, 0, 1}}, []float64{1, 3, 2, 4, 5}, -2, 4)
	if err != nil {
		t.Fatal(err)
	}
	s, err := source.ExactCyclicSpline(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	for _, station := range []*big.Rat{big.NewRat(-2, 1), big.NewRat(-6, 5), big.NewRat(13, 7)} {
		s, err = s.InsertKnot(ctx, station, o)
		if err != nil {
			t.Fatal(station, err)
		}
	}
	if raised, err := s.ElevateDegree(ctx, SurfacePatchOptions{MaxPoles: 12}); !errors.Is(err, ErrConvergence) || raised != nil {
		t.Fatal("temporary net cap must be enforced before degree elevation", err)
	}
	for wantDegree := 3; wantDegree <= 4; wantDegree++ {
		r, err := s.ElevateDegree(ctx, o)
		if err != nil || r.Degree() != wantDegree {
			t.Fatal("exact degree elevation", wantDegree, err)
		}
		assertPeriodicRaisedAxis(t, s.Knots(), r.Knots())
		ks := r.Knots()
		shift := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(-1), 102))
		for i, x := range ks {
			checkExactPeriodicCurveAgainstBezier(t, ctx, source, r, x)
			checkExactPeriodicCurveAgainstBezier(t, ctx, source, r, new(big.Rat).Add(x, shift))
			if i > 0 && ks[i-1].Cmp(x) != 0 {
				middle := new(big.Rat).Quo(new(big.Rat).Add(x, ks[i-1]), big.NewRat(2, 1))
				checkExactPeriodicCurveAgainstBezier(t, ctx, source, r, middle)
			}
		}
		s = r
	}
}

func TestPeriodicExactTensorDegreeElevationBothAxes(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchOptions{}
	d := BSplineSurfaceData{DegreeU: 1, DegreeV: 2, CountU: 4, CountV: 4}
	for u := range d.CountU {
		for v := range d.CountV {
			d.Poles = append(d.Poles, Vec3{float64(u), float64(v), float64((u + v*v) % 5)})
			d.Weights = append(d.Weights, float64(1+(u+v)%5))
		}
	}
	original, err := NewPeriodicBSplineSurface(d, &PeriodicSplineAxis{Start: -1, Period: 3}, &PeriodicSplineAxis{Start: 2, Period: 1})
	if err != nil {
		t.Fatal(err)
	}
	s, err := original.ExactCyclicSurface(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.InsertPeriodicKnot(ctx, 1, big.NewRat(29, 11), o)
	if err != nil {
		t.Fatal(err)
	}
	ou, ov, _ := s.Data()
	for dir := range 2 {
		if out, err := s.ElevatePeriodicDegree(ctx, dir, SurfacePatchOptions{MaxOperations: 90}); out != nil || !errors.Is(err, ErrConvergence) {
			t.Fatal("too little work published an elevated tensor", dir, err)
		}
		s, err = s.ElevatePeriodicDegree(ctx, dir, o)
		if err != nil {
			t.Fatal(dir, err)
		}
	}
	u, v, _ := s.Data()
	if u.Degree != 2 || v.Degree != 3 {
		t.Fatal("wrong elevated degrees", u.Degree, v.Degree)
	}
	assertPeriodicRaisedAxis(t, ou.Knots, u.Knots)
	assertPeriodicRaisedAxis(t, ov.Knots, v.Knots)
	for _, x := range []*big.Rat{big.NewRat(-4, 1), big.NewRat(-1, 1), big.NewRat(1, 7), big.NewRat(5, 4)} {
		for _, y := range []*big.Rat{big.NewRat(-4, 1), big.NewRat(2, 1), big.NewRat(29, 11), big.NewRat(19, 7)} {
			checkPeriodicExactTensorJet(t, ctx, original, s, x, y)
		}
	}
}

func assertPeriodicRaisedAxis(t *testing.T, original, elevated []*big.Rat) {
	t.Helper()
	j := 0
	for i := 0; i < len(original)-1; {
		next := i + 1
		for next < len(original)-1 && original[i].Cmp(original[next]) == 0 {
			next++
		}
		for range next - i + 1 {
			if j >= len(elevated)-1 || elevated[j].Cmp(original[i]) != 0 {
				t.Fatal("multiplicity did not increase by exactly one", i, original, elevated)
			}
			j++
		}
		i = next
	}
	if j != len(elevated)-1 || elevated[j].Cmp(original[len(original)-1]) != 0 {
		t.Fatal("degree elevation changed a closing endpoint", original, elevated)
	}
}
