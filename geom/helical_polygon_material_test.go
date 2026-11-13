package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalPolygonMaterialIntervals(t *testing.T) {
	ctx := context.Background()
	f, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	profile := []Vec2{{2, 0}, {3, .5}, {2, 1}}
	p, e := NewHelicalPolygon(f, profile, 2, RightHanded, -1, 2)
	if e != nil {
		t.Fatal(e)
	}
	origin := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(5, 2), big.NewRat(0, 1)}
	r, e := p.IntersectAxisLine(ctx, big.NewRat(0, 1), big.NewRat(1, 1), origin, big.NewRat(1, 1), HelicalPolygonLineOptions{})
	if e != nil || !r.Complete || len(r.Intervals) != 3 {
		t.Fatal(r, e)
	}
	for i, interval := range r.Intervals {
		want := big.NewRat(int64(-5+8*i), 4)
		if interval.Bounds[0][0].Cmp(want) != 0 || interval.Bounds[1][1].Cmp(new(big.Rat).Add(want, big.NewRat(1, 2))) != 0 || interval.Touch || len(interval.Contributors) != 1 || len(interval.Contributors[0].LowerEdges) != 1 || interval.Contributors[0].LowerEdges[0] != 0 || interval.Contributors[0].UpperEdges[0] != 1 {
			t.Fatal(interval)
		}
	}
	// Same triangular profile with overlapping turns must be merged exactly,
	// even when every endpoint shares a nonrational phase.
	tight, e := NewHelicalPolygon(f, profile, .25, LeftHanded, -1, 2)
	if e != nil {
		t.Fatal(e)
	}
	rayOrigin := [3]*big.Rat{big.NewRat(-3, 2), big.NewRat(-2, 1), big.NewRat(0, 1)}
	union, e := tight.IntersectAxisLine(ctx, big.NewRat(-3, 5), big.NewRat(-4, 5), rayOrigin, big.NewRat(1, 1), HelicalPolygonLineOptions{PhaseBits: 64})
	if e != nil || !union.Complete || len(union.Intervals) != 1 || len(union.Intervals[0].Contributors) != 3 || union.Intervals[0].Offsets[0].Sign() != 0 || union.Intervals[0].Offsets[1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(union, e)
	}
	reverse, e := p.IntersectAxisLine(ctx, big.NewRat(0, 1), big.NewRat(1, 1), origin, big.NewRat(-2, 1), HelicalPolygonLineOptions{})
	if e != nil || len(reverse.Intervals) != 3 {
		t.Fatal(reverse, e)
	}
	for i := 0; i < 3; i++ {
		want := new(big.Rat).Quo(new(big.Rat).Neg(r.Intervals[2-i].Bounds[1][1]), big.NewRat(2, 1))
		if reverse.Intervals[i].Bounds[0][0].Cmp(want) != 0 {
			t.Fatal(reverse)
		}
	}
	tangentOrigin := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(3, 1), big.NewRat(0, 1)}
	touch, e := p.IntersectAxisLine(ctx, big.NewRat(0, 1), big.NewRat(1, 1), tangentOrigin, big.NewRat(1, 1), HelicalPolygonLineOptions{})
	if e != nil || len(touch.Intervals) != 3 || !touch.Intervals[0].Touch || len(touch.Intervals[0].Contributors[0].LowerEdges) != 2 {
		t.Fatal(touch, e)
	}
	if bad, e := p.IntersectAxisLine(ctx, big.NewRat(0, 1), big.NewRat(1, 1), origin, big.NewRat(1, 1), HelicalPolygonLineOptions{MaxTurns: 2}); e == nil || bad.Complete || len(bad.Intervals) != 0 {
		t.Fatal(bad, e)
	}

	clockwise, e := NewHelicalPolygon(f, []Vec2{{2, 1}, {3, .5}, {2, 0}}, 2, RightHanded, -1, 2)
	if e != nil {
		t.Fatal(e)
	}
	same, e := clockwise.IntersectAxisLine(ctx, big.NewRat(0, 1), big.NewRat(1, 1), origin, big.NewRat(1, 1), HelicalPolygonLineOptions{})
	if e != nil || len(same.Intervals) != 3 || same.Intervals[1].Bounds[0][0].Cmp(r.Intervals[1].Bounds[0][0]) != 0 {
		t.Fatal(same, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := p.IntersectAxisLine(cancelled, big.NewRat(0, 1), big.NewRat(1, 1), origin, big.NewRat(1, 1), HelicalPolygonLineOptions{}); e == nil {
		t.Fatal("cancel")
	}
	if bad, e := p.IntersectAxisLine(ctx, big.NewRat(0, 1), big.NewRat(1, 1), origin, big.NewRat(1, 1), HelicalPolygonLineOptions{MaxOperations: 1}); e == nil || bad.Complete {
		t.Fatal("budget")
	}
	if _, e := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, 1}, {2, 1}, {3, 0}}, 1, RightHanded, 0, 1); e == nil {
		t.Fatal("bow tie accepted")
	}
}
