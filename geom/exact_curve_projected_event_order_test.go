package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestIndependentProjectedAlgebraicEventOrder(t *testing.T) {
	// A on [0,3] has q=t/3. Cubic B on [-2,5] has q=(15-3t)/7.
	// Neither map requires representable rational coefficients. B lies at z=1,
	// so equal projected events deliberately do not mean model coincidence.
	sa, _ := NewBSpline(2, []Vec3{{}, {X: .5}, {X: 1, Y: 1}}, []float64{0, 0, 0, 3, 3, 3}, nil)
	sb, _ := NewBSpline(3, []Vec3{{X: 3, Y: 9, Z: 1}, {X: 2, Y: 3, Z: 1}, {X: 1, Z: 1}, {Z: 1}}, []float64{-2, -2, -2, -2, 5, 5, 5, 5}, nil)
	a, e := NewExactSplineCurve(sa, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewExactSplineCurve(sb, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	curves := [2]*ExactSplineCurve{a, b}
	event := [4]*big.Rat{nil, big.NewRat(1, 1), nil, big.NewRat(-1, 2)}
	planes := [2][4]*big.Rat{event, event}
	intervals := [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(3, 1)}, {big.NewRat(-2, 1), big.NewRat(5, 1)}}
	r, e := CompareProjectedExactCurvePlaneEvents(context.Background(), curves, Vec3{X: 1}, planes, intervals, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Order != 0 || r.Maps[0].Scale.Cmp(big.NewRat(1, 3)) != 0 || r.Maps[1].Scale.Cmp(big.NewRat(-3, 7)) != 0 {
		t.Fatal(r, e)
	}
	planes[1][3] = new(big.Rat).Sub(event[3], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80)))
	r, e = CompareProjectedExactCurvePlaneEvents(context.Background(), curves, Vec3{X: 1}, planes, intervals, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Order != -1 || r.Intervals[0][1].Cmp(r.Intervals[1][0]) >= 0 {
		t.Fatal(r, e)
	}
	for i := 0; i < 2; i++ {
		for _, parameter := range r.Parameters[i] {
			q := new(big.Rat).Add(r.Maps[i].Offset, new(big.Rat).Mul(r.Maps[i].Scale, parameter))
			if q.Cmp(r.Intervals[i][0]) < 0 || q.Cmp(r.Intervals[i][1]) > 0 {
				t.Fatal("original parameter map", i, q)
			}
		}
	}
	swapped, e := CompareProjectedExactCurvePlaneEvents(context.Background(), [2]*ExactSplineCurve{b, a}, Vec3{X: 1}, [2][4]*big.Rat{planes[1], planes[0]}, [2][2]*big.Rat{intervals[1], intervals[0]}, ExactCurvePlaneEventOptions{})
	if e != nil || !swapped.Proven || swapped.Order != 1 {
		t.Fatal(swapped, e)
	}
	_, e = CompareProjectedExactCurvePlaneEvents(context.Background(), curves, Vec3{Y: 1}, planes, intervals, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal("nonaffine projection", e)
	}
	r, e = CompareProjectedExactCurvePlaneEvents(context.Background(), curves, Vec3{X: 1}, planes, intervals, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
}
