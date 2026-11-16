package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactNonuniformTensorKnotInsertion(t *testing.T) {
	ctx := context.Background()
	data := BSplineSurfaceData{DegreeU: 2, DegreeV: 2, CountU: 5, CountV: 4}
	for u := range data.CountU {
		for v := range data.CountV {
			data.Poles = append(data.Poles, Vec3{float64(u), float64(v), float64((u*u+v*u+v)%7) / 8})
			data.Weights = append(data.Weights, float64(1+(u+2*v)%5))
		}
	}
	s, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Start: -1, Period: 3}, &PeriodicSplineAxis{Start: 2, Period: 1.25})
	if err != nil {
		t.Fatal(err)
	}
	base, err := s.ExactCyclicSurface(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	shift := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 100))
	farU := new(big.Rat).Mul(shift, big.NewRat(-3, 1))
	farV := new(big.Rat).Mul(shift, big.NewRat(5, 4))
	nearU, nearV := big.NewRat(-16, 17), big.NewRat(55, 17)
	newU, newV := new(big.Rat).Add(farU, nearU), new(big.Rat).Add(farV, nearV)
	refined := base
	for _, edit := range []struct {
		dir int
		t   *big.Rat
	}{{0, newU}, {1, newV}, {0, big.NewRat(-2, 5)}, {1, big.NewRat(37, 16)}} {
		refined, err = refined.InsertPeriodicKnot(ctx, edit.dir, edit.t, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(edit, err)
		}
	}
	uAxis, vAxis, controls := refined.Data()
	oldU, oldV, _ := base.Data()
	if oldU.Count != 5 || oldV.Count != 4 || uAxis.Count != 7 || vAxis.Count != 6 || len(controls) != 42 ||
		uAxis.Knots[1].Cmp(nearU) != 0 || vAxis.Knots[len(vAxis.Knots)-2].Cmp(nearV) != 0 {
		t.Fatal("edited tensor lost the original or inserted axes", uAxis, vAxis)
	}
	for _, u := range []*big.Rat{uAxis.Knots[0], nearU, big.NewRat(-2, 5), big.NewRat(5, 7), uAxis.Knots[7], newU} {
		for _, v := range []*big.Rat{vAxis.Knots[0], big.NewRat(37, 16), nearV, big.NewRat(21, 8), vAxis.Knots[6], newV} {
			checkPeriodicExactTensorJet(t, ctx, s, refined, u, v)
		}
	}
	// Reconstruct from the genuinely nonuniform axes, and check that
	// neither constructor inputs nor detached output data are aliased.
	copyOfRefined, err := NewExactPeriodicSplineSurface(ctx, uAxis, vAxis, controls, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	uAxis.Knots[0].SetInt64(900)
	vAxis.Knots[0].SetInt64(-900)
	controls[0][3].SetInt64(0)
	checkPeriodicExactTensorJet(t, ctx, s, copyOfRefined, newU, newV)
	checkPeriodicExactTensorJet(t, ctx, s, refined, newU, newV)
	for _, o := range []SurfacePatchOptions{{MaxOperations: 60}, {MaxPoles: 42}} {
		if got, err := refined.InsertPeriodicKnot(ctx, 0, big.NewRat(1, 7), o); got != nil || !errors.Is(err, ErrConvergence) {
			t.Fatal("short tensor insertion published geometry", err)
		}
	}
	if got, err := refined.InsertPeriodicKnot(ctx, 1, big.NewRat(37, 16), SurfacePatchOptions{}); got != nil || !errors.Is(err, ErrDomain) {
		t.Fatal("tensor insertion exceeded cyclic knot multiplicity", err)
	}
}

func TestPeriodicExactNonuniformTensorPreservesOrdinaryDiscontinuity(t *testing.T) {
	ctx := context.Background()
	data := BSplineSurfaceData{DegreeU: 1, DegreeV: 1, CountU: 4, CountV: 3, KnotsU: []float64{-1, -1, .25, .25, 3, 3}}
	for u := range data.CountU {
		for v := range data.CountV {
			data.Poles = append(data.Poles, Vec3{float64(u), float64(v), float64((u + v) % 3)})
			data.Weights = append(data.Weights, float64(1+(u+v)%4))
		}
	}
	s, err := NewPeriodicBSplineSurface(data, nil, &PeriodicSplineAxis{Start: -2, Period: 5})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.ExactCyclicSurface(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c, err = c.InsertPeriodicKnot(ctx, 1, big.NewRat(25, 13), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 4), big.NewRat(3, 1)} {
		checkPeriodicExactTensorJet(t, ctx, s, c, u, big.NewRat(25, 13))
	}
	for _, u := range []*big.Rat{big.NewRat(-2, 1), big.NewRat(4, 1)} {
		if got, err := c.EvaluateRational(ctx, u, new(big.Rat), SurfacePatchOptions{}); !errors.Is(err, ErrDomain) || got.Point[0] != nil {
			t.Fatal("outside ordinary tensor domain", err)
		}
	}
	if got, err := c.InsertPeriodicKnot(ctx, 0, new(big.Rat), SurfacePatchOptions{}); got != nil || !errors.Is(err, ErrDomain) {
		t.Fatal("ordinary axis admitted a cyclic insertion", err)
	}
	jet, err := c.EvaluateRational(ctx, big.NewRat(1, 4), big.NewRat(25, 13), SurfacePatchOptions{})
	if err != nil || jet.U.Span != 2 || jet.U.Cycle != nil || jet.Point[0].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("right-hand discontinuity was lost", jet, err)
	}
	short := SurfacePatchOptions{MaxOperations: jet.ExactOperations - 1}
	if got, err := c.EvaluateRational(ctx, big.NewRat(1, 4), big.NewRat(25, 13), short); !errors.Is(err, ErrConvergence) || got.Point[0] != nil || got.V.Cycle != nil {
		t.Fatal("limited tensor evaluation published an uncertified jet", err)
	}
}

func checkPeriodicExactTensorJet(t *testing.T, ctx context.Context, original *PeriodicBSplineSurface, edited *ExactPeriodicSplineSurface, u, v *big.Rat) {
	t.Helper()
	want, err := original.EvaluateRational(ctx, u, v, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(u, v, err)
	}
	got, err := edited.EvaluateRational(ctx, u, v, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(u, v, err)
	}
	for d := range got.Point {
		if got.Point[d].Cmp(want.Point[d]) != 0 || got.Du[d].Cmp(want.Du[d]) != 0 || got.Dv[d].Cmp(want.Dv[d]) != 0 {
			t.Fatalf("parameter (%s,%s), coordinate %d: want %s, %s, %s; got %s, %s, %s", u, v, d,
				want.Point[d], want.Du[d], want.Dv[d], got.Point[d], got.Du[d], got.Dv[d])
		}
	}
	assertExactPointInBox(t, got.Point, got.PointBounds)
	assertExactPointInBox(t, got.Du, got.DuBounds)
	assertExactPointInBox(t, got.Dv, got.DvBounds)
	for _, o := range []struct {
		t    *big.Rat
		base ExactSurfaceKnotSpan
		got  ExactSurfaceKnotSpan
	}{{u, want.U, got.U}, {v, want.V, got.V}} {
		if o.got.Domain.Lower.Cmp(o.t) > 0 || o.got.Domain.Upper.Cmp(o.t) < 0 || (o.got.Cycle == nil) != (o.base.Cycle == nil) ||
			o.got.Cycle != nil && (o.got.Cycle.Cmp(o.base.Cycle) != 0 || o.got.Domain.Upper.Cmp(o.t) == 0) {
			t.Fatal("changed original cycle or misassigned refined owner", o)
		}
	}
}
