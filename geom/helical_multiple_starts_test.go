package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSeparatedHelicalStarts(t *testing.T) {
	ctx := context.Background()
	f, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	p, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, .25}, {2, .5}}, 2, RightHanded, -1, 2)
	s, e := NewSeparatedHelicalStarts(p, 3)
	if e != nil {
		t.Fatal(e)
	}
	n, spacing, gap, e := s.Separation()
	if e != nil || n != 3 || spacing.Cmp(big.NewRat(2, 3)) != 0 || gap.Cmp(big.NewRat(1, 6)) != 0 {
		t.Fatal(n, spacing, gap, e)
	}
	spacing.SetInt64(100)
	_, spacing, _, _ = s.Separation()
	if spacing.Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal("mutable proof")
	}
	for _, ray := range [][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(-3, 5), big.NewRat(4, 5)}} {
		origin := [3]*big.Rat{new(big.Rat).Mul(ray[0], big.NewRat(5, 2)), new(big.Rat).Mul(ray[1], big.NewRat(5, 2)), big.NewRat(0, 1)}
		r, e := s.IntersectAxisLine(ctx, ray[0], ray[1], origin, big.NewRat(-2, 1), HelicalStartsLineOptions{})
		if e != nil || !r.Complete || len(r.Intervals) != 9 {
			t.Fatal(r, e)
		}
		counts := [3]int{}
		for i, interval := range r.Intervals {
			counts[interval.Source.Start]++
			if interval.Source.AxialOffset.Cmp(new(big.Rat).Mul(big.NewRat(int64(interval.Source.Start), 1), big.NewRat(2, 3))) != 0 || len(interval.Source.Meridians) != 1 {
				t.Fatal(interval)
			}
			if i > 0 && r.Intervals[i-1].Offsets[1].Cmp(interval.Offsets[0]) >= 0 {
				t.Fatal("overlap")
			}
		}
		if counts != [3]int{3, 3, 3} {
			t.Fatal(counts)
		}

		if ray[0].Sign() == 0 {
			for _, interval := range r.Intervals {
				u := interval.Source.Meridians[0].Turn[0]
				height := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), u), new(big.Rat).Add(big.NewRat(3, 8), interval.Source.AxialOffset))
				want := new(big.Rat).Quo(height, big.NewRat(-2, 1))
				if interval.Bounds[0][0].Cmp(want) != 0 {
					t.Fatal("independent endpoint", interval, want)
				}
			}
		}
		if bad, e := s.IntersectAxisLine(ctx, ray[0], ray[1], origin, big.NewRat(1, 1), HelicalStartsLineOptions{HelicalPolygonLineOptions: HelicalPolygonLineOptions{MaxOperations: 1}}); e == nil || bad.Complete {
			t.Fatal("work cap")
		}
		if bad, e := s.IntersectAxisLine(ctx, ray[0], ray[1], origin, big.NewRat(1, 1), HelicalStartsLineOptions{MaxIntervals: 8}); e == nil || bad.Complete || len(bad.Intervals) != 0 {
			t.Fatal("limit", bad, e)
		}
	}
	if _, e := NewSeparatedHelicalStarts(p, 4); e == nil {
		t.Fatal("equal spacing contact accepted")
	}
	if _, e := NewSeparatedHelicalStarts(p, 5); e == nil {
		t.Fatal("insufficient global gap accepted")
	}
}
