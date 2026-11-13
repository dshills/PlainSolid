package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPlanarCircleExactSectorPartition(t *testing.T) {
	ctx := context.Background()
	zero := new(big.Rat)
	unit := big.NewRat(1, 1)
	center := [3]*big.Rat{unit, zero, zero}
	for _, reverse := range []bool{false, true} {
		a := directedCircleCell(t, 0, false, reverse, zero)
		r, e := a.ClassifyPlanarCircle(ctx, [2]int{0, 1}, center, unit, ExactCurvePlaneEventOptions{})
		sign := -1
		if reverse {
			sign = 1
		}
		if e != nil || !r.Complete || r.Coincident || len(r.Contacts) != 1 || len(r.Intervals) != 2 || r.Intervals[0].Sign != sign || r.Intervals[1].Sign != -sign || r.Contacts[0].SideSigns != [2]int{sign, -sign} || r.Contacts[0].Order != 1 {
			t.Fatal(r, e)
		}
		// Restriction at the actual irrational crossing must retain a boundary root,
		// and only the surviving strict sector; an enclosing rational cut is not used.
		cut, e := a.RestrictAlgebraic(ctx, r.Contacts[0].Event.Root(), r.Domain[1], ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		sub, e := cut.Cell.ClassifyPlanarCircle(ctx, [2]int{0, 1}, center, unit, ExactCurvePlaneEventOptions{})
		if e != nil || !sub.Complete || len(sub.Contacts) != 1 || len(sub.Intervals) != 1 || sub.Intervals[0].Sign != -sign || sub.Contacts[0].SideSigns != [2]int{0, -sign} {
			t.Fatal(sub, e)
		}
	}
	a := directedCircleCell(t, 0, false, false, zero)
	same, e := a.ClassifyPlanarCircle(ctx, [2]int{0, 1}, [3]*big.Rat{zero, zero, zero}, unit, ExactCurvePlaneEventOptions{})
	if e != nil || !same.Complete || !same.Coincident || len(same.Contacts) != 0 || len(same.Intervals) != 0 {
		t.Fatal(same, e)
	}
	tangent, e := a.ClassifyPlanarCircle(ctx, [2]int{0, 1}, [3]*big.Rat{big.NewRat(2, 1), zero, zero}, unit, ExactCurvePlaneEventOptions{})
	if e != nil || !tangent.Complete || len(tangent.Contacts) != 1 || tangent.Contacts[0].Order != 2 || tangent.Contacts[0].SideSigns != [2]int{0, 1} || len(tangent.Intervals) != 1 || tangent.Intervals[0].Sign != 1 {
		t.Fatal(tangent, e)
	}
	// Both containment alternatives have no artificial crossing events.
	for _, radius := range []int64{4, 1} {
		c := [3]*big.Rat{zero, zero, zero}
		want := -1
		if radius == 1 {
			c[0] = big.NewRat(4, 1)
			want = 1
		}
		r, e := a.ClassifyPlanarCircle(ctx, [2]int{0, 1}, c, big.NewRat(radius, 1), ExactCurvePlaneEventOptions{})
		if e != nil || !r.Complete || len(r.Contacts) != 0 || len(r.Intervals) != 1 || r.Intervals[0].Sign != want {
			t.Fatal(r, e)
		}
	}
	for _, opts := range []ExactCurvePlaneEventOptions{{MaxOperations: 1}, {MaxCoefficientBits: 2}} {
		r, e := a.ClassifyPlanarCircle(ctx, [2]int{0, 1}, [3]*big.Rat{big.NewRat(17, 3), zero, zero}, unit, opts)
		if e == nil || r.Complete || r.Domain[0] != nil || len(r.Contacts) != 0 || len(r.Intervals) != 0 {
			t.Fatal(r, e)
		}
	}
	r, e := a.ClassifyPlanarCircle(ctx, [2]int{0, 1}, [3]*big.Rat{zero, zero, big.NewRat(1, 1000000)}, unit, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrDomain) || r.Complete {
		t.Fatal(r, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = a.ClassifyPlanarCircle(cancelled, [2]int{0, 1}, center, unit, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}

func TestPlanarCircleLineTwoCrossingsAndInteriorTouch(t *testing.T) {
	ctx := context.Background()
	for _, height := range []int64{0, 1} {
		net := [][4]*big.Rat{{big.NewRat(-2, 1), big.NewRat(height, 1), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(2, 1), big.NewRat(height, 1), new(big.Rat), big.NewRat(1, 1)}}
		curve, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		r, e := cell.ClassifyPlanarCircle(ctx, [2]int{0, 1}, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, big.NewRat(1, 1), ExactCurvePlaneEventOptions{})
		if e != nil || !r.Complete {
			t.Fatal(r, e)
		}
		if height == 0 {
			if len(r.Contacts) != 2 || len(r.Intervals) != 3 || r.Intervals[0].Sign != 1 || r.Intervals[1].Sign != -1 || r.Intervals[2].Sign != 1 {
				t.Fatal(r)
			}
			for i, event := range r.Contacts {
				p, e := event.Event.EvaluateRationalWitness(ctx, big.NewRat(int64(1+2*i), 4), ExactCurvePlaneEventOptions{})
				if e != nil || p.Point[0].Cmp(big.NewRat(int64(-1+2*i), 1)) != 0 {
					t.Fatal(p, e)
				}
			}
		} else if len(r.Contacts) != 1 || r.Contacts[0].Order != 2 || r.Contacts[0].SideSigns != [2]int{1, 1} || len(r.Intervals) != 2 || r.Intervals[0].Sign != 1 || r.Intervals[1].Sign != 1 {
			t.Fatal(r)
		}
	}
}
