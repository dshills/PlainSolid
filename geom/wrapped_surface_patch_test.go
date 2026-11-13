package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func wrappedCylinderPatch(t *testing.T) *SurfacePatch {
	t.Helper()
	circle, _ := NewCircle(Vec3{}, Vec3{Z: 1}, 2)
	c, _ := circle.RationalBSpline()
	var poles []Vec3
	var weights []float64
	for i, p := range c.poles {
		for _, v := range []float64{-2, 3} {
			poles = append(poles, Vec3{p.X, p.Y, v})
			weights = append(weights, c.weights[i])
		}
	}
	s, e := NewBSplineSurface(2, 1, len(c.poles), 2, poles, c.knots, []float64{-2, -2, 3, 3}, weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestWrappedSurfaceRationalCylinderSeamAndPhase(t *testing.T) {
	ctx := context.Background()
	p := wrappedCylinderPatch(t)
	var e error
	p, e = p.InsertUKnot(.13, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertVKnot(.7, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, seam, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil || !seam.PositionMatched[0] || !seam.FirstPartialsMatched[0] {
		t.Fatal(seam, e)
	}
	base, e := w.EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, k := range []*big.Int{big.NewInt(-100), big.NewInt(-1), big.NewInt(0), big.NewInt(23), new(big.Int).Lsh(big.NewInt(1), 1000)} {
		u := new(big.Rat).Add(big.NewRat(1, 3), new(big.Rat).Mul(new(big.Rat).SetInt(k), big.NewRat(4, 1)))
		actual, e := w.EvaluateRational(ctx, u, big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for axis := 0; axis < 3; axis++ {
			if actual.Point[axis].Cmp(base.Point[axis]) != 0 || actual.Du[axis].Cmp(base.Du[axis]) != 0 || actual.Dv[axis].Cmp(base.Dv[axis]) != 0 {
				t.Fatal(k, actual, base)
			}
		}
	}
	edited, editedSeam, e := w.InsertKnot(ctx, 0, .21, 1, SurfacePatchOptions{})
	if e != nil || !editedSeam.FirstPartialsMatched[0] {
		t.Fatal(editedSeam, e)
	}
	editedPoint, e := edited.EvaluateRational(ctx, big.NewRat(1, 3), big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for axis := 0; axis < 3; axis++ {
		if editedPoint.Point[axis].Cmp(base.Point[axis]) != 0 || editedPoint.Du[axis].Cmp(base.Du[axis]) != 0 {
			t.Fatal(editedPoint, base)
		}
	}
	_, _, e = w.InsertKnot(ctx, 0, .22, 1, SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	first, _ := w.Eval(0, .2)
	last, e := w.Eval(4, .2)
	if e != nil || first != last {
		t.Fatal(first, last, e)
	}
	_, e = w.Eval(.2, 4)
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	periods := w.Periods()
	periods[0].SetInt64(99)
	if w.Periods()[0].Cmp(big.NewRat(4, 1)) != 0 {
		t.Fatal("period alias")
	}
}
func TestWrappedSurfaceBothAxesAndRejectSeam(t *testing.T) {
	makePatch := func(badDerivative, badPosition bool) *SurfacePatch {
		var poles []Vec3
		controls := []float64{0, 1, -1, 0}
		if badDerivative {
			controls[2] = 1
		}
		if badPosition {
			controls[3] = .125
		}
		for _, x := range controls {
			for _, y := range []float64{0, 1, -1, 0} {
				poles = append(poles, Vec3{x, y, x * y})
			}
		}
		k := []float64{0, 0, 0, 0, 1, 1, 1, 1}
		s, e := NewBSplineSurface(3, 3, 4, 4, poles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
		return p
	}
	p := makePatch(false, false)
	w, seam, e := NewWrappedSurfacePatch(context.Background(), p, WrappedSurfaceOptions{WrapU: true, WrapV: true, ContinuityOrder: 1})
	if e != nil || seam.FirstPartialsMatched != ([2]bool{true, true}) {
		t.Fatal(seam, e)
	}
	for i := -5; i <= 5; i++ {
		for j := -4; j <= 4; j++ {
			got, e := w.EvaluateRational(context.Background(), big.NewRat(int64(3*i+1), 3), big.NewRat(int64(7*j+2), 7), SurfacePatchEvaluationOptions{})
			want, f := p.EvaluateRational(context.Background(), big.NewRat(1, 3), big.NewRat(2, 7), SurfacePatchEvaluationOptions{})
			if e != nil || f != nil {
				t.Fatal(e, f)
			}
			for axis := 0; axis < 3; axis++ {
				if got.Point[axis].Cmp(want.Point[axis]) != 0 || got.Du[axis].Cmp(want.Du[axis]) != 0 {
					t.Fatal(got, want)
				}
			}
		}
	}
	bad := makePatch(true, false)
	_, seam, e = NewWrappedSurfacePatch(context.Background(), bad, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if !errors.Is(e, ErrDegenerate) || !seam.PositionMatched[0] || seam.FirstPartialsMatched[0] {
		t.Fatal(seam, e)
	}
	_, _, e = NewWrappedSurfacePatch(context.Background(), bad, WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = NewWrappedSurfacePatch(context.Background(), makePatch(false, true), WrappedSurfaceOptions{WrapU: true})
	if !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	_, _, e = NewWrappedSurfacePatch(context.Background(), p, WrappedSurfaceOptions{WrapU: true, MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, e = NewWrappedSurfacePatch(canceled, p, WrappedSurfaceOptions{WrapU: true})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
