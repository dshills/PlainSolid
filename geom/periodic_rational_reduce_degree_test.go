package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactCurveDegreeReductionRecoversRaisedNet(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchOptions{}
	c, err := NewPeriodicBSplineWithExactWeights(2,
		[]Vec3{{1, 0, 0}, {3, 2, 1}, {2, -1, 3}, {-1, 1, 2}, {-2, 0, 1}}, []float64{1, 3, 2, 4, 5}, -2, 4)
	if err != nil {
		t.Fatal(err)
	}
	base, err := c.ExactCyclicSpline(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	base, err = base.InsertKnot(ctx, big.NewRat(-2, 1), o)
	if err != nil {
		t.Fatal(err)
	}
	base, err = base.InsertKnot(ctx, big.NewRat(13, 7), o)
	if err != nil {
		t.Fatal(err)
	}
	high := base
	for range 2 {
		high, err = high.ElevateDegree(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
	}
	if out, err := high.ReduceDegreeExact(ctx, SurfacePatchOptions{MaxOperations: 10}); out != nil || !errors.Is(err, ErrConvergence) {
		t.Fatal("short reduction published an approximate net", err)
	}
	low := high
	for range 2 {
		low, err = low.ReduceDegreeExact(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
	}
	assertCyclicRationalNet(t, base.Knots(), low.Knots(), base.HomogeneousControls(), low.HomogeneousControls())
	if out, err := base.ReduceDegreeExact(ctx, o); out != nil || !errors.Is(err, ErrUnrepresentable) {
		t.Fatal("genuine rational quadratic has no exact degree-one homogeneous net", err)
	}
}

func TestPeriodicExactTensorDegreeReductionRefusesOneChangedRow(t *testing.T) {
	ctx := context.Background()
	o := SurfacePatchOptions{}
	d := BSplineSurfaceData{DegreeU: 2, DegreeV: 1, CountU: 4, CountV: 3, KnotsV: []float64{0, 0, .5, 1, 1}}
	for u := range d.CountU {
		for v := range d.CountV {
			d.Poles = append(d.Poles, Vec3{float64(u), float64(v), float64((u + v*v) % 5)})
			d.Weights = append(d.Weights, float64(1+(u+v)%5))
		}
	}
	origin, err := NewPeriodicBSplineSurface(d, &PeriodicSplineAxis{Start: -1, Period: 3}, nil)
	if err != nil {
		t.Fatal(err)
	}
	base, err := origin.ExactCyclicSurface(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	high, err := base.ElevatePeriodicDegree(ctx, 0, o)
	if err != nil {
		t.Fatal(err)
	}
	low, err := high.ReducePeriodicDegreeExact(ctx, 0, o)
	if err != nil {
		t.Fatal(err)
	}
	u, v, h := base.Data()
	lu, lv, lh := low.Data()
	assertCyclicRationalNet(t, u.Knots, lu.Knots, h, lh)
	assertCyclicRationalNet(t, v.Knots, lv.Knots, h, lh)
	if out, err := high.ReducePeriodicDegreeExact(ctx, 1, o); out != nil || !errors.Is(err, ErrDomain) {
		t.Fatal("ordinary tensor direction admitted periodic edit", err)
	}
	u, v, h = high.Data()
	h[len(h)-1][2].Add(h[len(h)-1][2], big.NewRat(1, 4096))
	changed, err := NewExactPeriodicSplineSurface(ctx, u, v, h, o)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := changed.ReducePeriodicDegreeExact(ctx, 0, o); out != nil || !errors.Is(err, ErrUnrepresentable) {
		t.Fatal("one changed tensor row published a partly reduced object", err)
	}
}
