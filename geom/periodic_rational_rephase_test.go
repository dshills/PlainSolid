package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactCurveRepeatedSeamAndRephase(t *testing.T) {
	ctx := context.Background()
	original, err := NewPeriodicBSplineWithExactWeights(2,
		[]Vec3{{1, 0, 0}, {3, 2, 1}, {2, -1, 3}, {-1, 1, 2}}, []float64{1, 3, 2, 4}, -.25, 1.75)
	if err != nil {
		t.Fatal(err)
	}
	c, err := original.ExactCyclicSpline(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Insert through the closing endpoint. The added station is stored
	// at the beginning only. Rephase below selects a repeated interior
	// station in a distant cycle while preserving its two incidences.
	c, err = c.InsertKnot(ctx, big.NewRat(3, 2), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	knots := c.Knots()
	if knots[0].Cmp(knots[1]) != 0 || knots[len(knots)-2].Cmp(knots[len(knots)-1]) >= 0 {
		t.Fatal("repeated seam was not stored canonically", knots)
	}
	if out, err := c.InsertKnot(ctx, knots[0], SurfacePatchOptions{}); out != nil || !errors.Is(err, ErrDomain) {
		t.Fatal("seam multiplicity exceeded degree", err)
	}
	inner := knots[2]
	c, err = c.InsertKnot(ctx, inner, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cycle := new(big.Int).Lsh(big.NewInt(1), 100)
	newSeam := new(big.Rat).Add(inner, new(big.Rat).Mul(new(big.Rat).SetInt(cycle), big.NewRat(7, 4)))
	r, err := c.RephaseAtKnot(ctx, newSeam, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []*big.Rat{newSeam, new(big.Rat).Sub(newSeam, big.NewRat(1, 37)), new(big.Rat).Add(newSeam, big.NewRat(1, 43)), inner, knots[0], knots[len(knots)-1]} {
		checkExactPeriodicCurveAgainstBezier(t, ctx, original, c, x)
		checkExactPeriodicCurveAgainstBezier(t, ctx, original, r, x)
	}
	lo, hi := new(big.Rat).Sub(newSeam, big.NewRat(1, 31)), new(big.Rat).Add(newSeam, big.NewRat(1, 31))
	p, _, err := r.ExactBezierSpans(ctx, lo, hi, ExactBezierSpanOptions{})
	if err != nil || len(p) != 2 || p[0].Cycle.Int64() != -1 || p[1].Cycle.Sign() != 0 || p[1].Span != 1 ||
		p[0].End.Cmp(newSeam) != 0 || p[1].Start.Cmp(newSeam) != 0 {
		t.Fatal("rephased extraction lost either original incidence", p, err)
	}
	if out, err := r.RephaseAtKnot(ctx, hi, SurfacePatchOptions{}); out != nil || !errors.Is(err, ErrDomain) {
		t.Fatal("rephase admitted an unauthored station", err)
	}
	if out, err := c.RephaseAtKnot(ctx, newSeam, SurfacePatchOptions{MaxCoefficientBits: 64}); out != nil || !errors.Is(err, ErrConvergence) {
		t.Fatal("rephase exceeded part of its bit budget", err)
	}
}

func TestPeriodicExactTensorRepeatedSeamAndRephase(t *testing.T) {
	ctx := context.Background()
	data := BSplineSurfaceData{DegreeU: 2, DegreeV: 2, CountU: 4, CountV: 3}
	for u := range data.CountU {
		for v := range data.CountV {
			data.Poles = append(data.Poles, Vec3{float64(u), float64(v), float64((u + v) % 3)})
			data.Weights = append(data.Weights, float64(1+(u+v)%4))
		}
	}
	original, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Start: -1, Period: 3}, &PeriodicSplineAxis{Start: 2, Period: 1})
	if err != nil {
		t.Fatal(err)
	}
	s, err := original.ExactCyclicSurface(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.InsertPeriodicKnot(ctx, 1, big.NewRat(16, 3), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.RephasePeriodicKnot(ctx, 1, big.NewRat(-2, 3), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.InsertPeriodicKnot(ctx, 0, big.NewRat(-4, 1), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	u, v, _ := s.Data()
	if u.Knots[0].Cmp(big.NewRat(-1, 1)) != 0 || v.Knots[0].Cmp(big.NewRat(-2, 3)) != 0 ||
		u.Knots[1].Cmp(u.Knots[0]) != 0 || v.Knots[1].Cmp(v.Knots[0]) != 0 {
		t.Fatal("expected repeated start of both edited axes", u, v)
	}
	for _, x := range []*big.Rat{big.NewRat(-4, 1), big.NewRat(-1, 1), big.NewRat(5, 7)} {
		for _, y := range []*big.Rat{big.NewRat(-2, 3), big.NewRat(7, 3), big.NewRat(9, 11)} {
			want, err := original.EvaluateRational(ctx, x, y, SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.EvaluateRational(ctx, x, y, SurfacePatchOptions{})
			if err != nil || got.V.Domain.Lower.Cmp(y) > 0 || got.V.Domain.Upper.Cmp(y) <= 0 || got.U.Domain.Lower.Cmp(x) > 0 || got.U.Domain.Upper.Cmp(x) <= 0 {
				t.Fatal("wrong rephased owners", x, y, got, err)
			}
			for d := range got.Point {
				if got.Point[d].Cmp(want.Point[d]) != 0 || got.Du[d].Cmp(want.Du[d]) != 0 || got.Dv[d].Cmp(want.Dv[d]) != 0 {
					t.Fatal("new periodic seam changed the original point or partials", x, y, d)
				}
			}
		}
	}
}
