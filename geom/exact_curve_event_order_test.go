package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactAlgebraicCurveEventIdentityAndOrder(t *testing.T) {
	s, _ := NewBezier([]Vec3{{}, {X: 1}, {X: 2, Y: 1}, {X: 3, Y: 3, Z: 1}}, nil)
	c := isoIntersectionFixture(t, s)
	// C=(3t,3t²,t³); distinct event polynomials share sqrt(1/2).
	a := [4]*big.Rat{nil, big.NewRat(1, 1), nil, big.NewRat(-3, 2)}
	b := [4]*big.Rat{big.NewRat(-1, 6), nil, big.NewRat(1, 1), nil}
	intervals := [2][2]*big.Rat{{big.NewRat(1, 2), big.NewRat(1, 1)}, {big.NewRat(2, 3), big.NewRat(3, 4)}}
	r, e := CompareExactCurvePlaneEvents(context.Background(), c, [2][4]*big.Rat{a, b}, intervals, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Order != 0 {
		t.Fatal(r, e)
	}
	r.Intervals[0][0].SetInt64(9)
	if intervals[0][0].Cmp(big.NewRat(1, 2)) != 0 || r.Intervals[1][0].Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal("aliased rational outputs")
	}
	// Two irrational roots separated by less than 2^-80 remain distinct.
	b = a
	b[3] = new(big.Rat).Sub(a[3], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80)))
	for i, planes := range [2][2][4]*big.Rat{{a, b}, {b, a}} {
		r, e := CompareExactCurvePlaneEvents(context.Background(), c, planes, intervals, ExactCurvePlaneEventOptions{})
		expected := -1
		if i == 1 {
			expected = 1
		}
		if e != nil || !r.Proven || r.Order != expected {
			t.Fatal(r, e)
		}
		if expected < 0 && r.Intervals[0][1].Cmp(r.Intervals[1][0]) >= 0 {
			t.Fatal("order lacks disjoint enclosures")
		}
	}
	limited := ExactCurvePlaneEventOptions{MaxSubdivisions: 2}
	r, e = CompareExactCurvePlaneEvents(context.Background(), c, [2][4]*big.Rat{a, b}, intervals, limited)
	if !errors.Is(e, ErrConvergence) || r.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = CompareExactCurvePlaneEvents(ctx, c, [2][4]*big.Rat{a, b}, intervals, ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestExactCurveEventRepeatedAndNonunique(t *testing.T) {
	s, _ := NewBezier([]Vec3{{X: 1}, {X: -1}, {X: 1}}, nil)
	c := isoIntersectionFixture(t, s)
	p := [4]*big.Rat{big.NewRat(1, 1), nil, nil, nil}
	intervals := [2][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(1, 2), big.NewRat(1, 2)}}
	r, e := CompareExactCurvePlaneEvents(context.Background(), c, [2][4]*big.Rat{p, p}, intervals, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Order != 0 {
		t.Fatal(r, e)
	}
	p[3] = big.NewRat(-1, 2)
	_, e = CompareExactCurvePlaneEvents(context.Background(), c, [2][4]*big.Rat{p, p}, intervals, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
