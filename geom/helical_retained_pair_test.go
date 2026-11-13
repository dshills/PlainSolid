package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalRetainedPair(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, 0}, {3, .25}, {2, .25}}, 1, RightHanded, 0, 1)
	s, e := NewSeparatedHelicalStarts(p, 1)
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	band := [2]*big.Rat{r(0, 1), r(2, 1)}
	q, e := CompareHelicalRetainedQuarter(context.Background(), s, s, r(5, 2), r(0, 1), r(0, 1), band, band, HelicalStartsLineOptions{})
	if e != nil || !q.Complete || !q.Overlap || len(q.A) != 2 || q.A[0].Bounds[0].Cmp(r(1, 4)) != 0 || q.A[0].Bounds[1].Cmp(r(1, 1)) != 0 || q.A[1].Bounds[0].Cmp(r(5, 4)) != 0 {
		t.Fatal(q, e)
	}
	if !q.A[0].Boundary[0].CutterUpper || q.A[0].Boundary[1].Turn != 1 || !q.A[1].Boundary[1].StockCap {
		t.Fatal(q.A)
	}
	// Beyond the finite turn end, material remains. A nut stock there collides
	// even though an infinite periodic cutter would have removed part of it.
	q, e = CompareHelicalRetainedQuarter(context.Background(), s, s, r(5, 2), r(1, 4), r(0, 1), band, [2]*big.Rat{r(3, 2), r(7, 4)}, HelicalStartsLineOptions{})
	if e != nil || !q.Overlap {
		t.Fatal(q, e)
	}
	// Radius outside cutters gives exact stock cap contact after translating B.
	q, e = CompareHelicalRetainedQuarter(context.Background(), s, s, r(4, 1), r(0, 1), r(2, 1), band, band, HelicalStartsLineOptions{})
	if e != nil || !q.Touch || q.Overlap || len(q.Contacts) != 1 {
		t.Fatal(q, e)
	}
	q, e = CompareHelicalRetainedQuarter(context.Background(), s, s, r(4, 1), r(0, 1), r(3, 1), band, band, HelicalStartsLineOptions{})
	if e != nil || q.Touch || q.Overlap {
		t.Fatal(q, e)
	}
	if _, e = CompareHelicalRetainedQuarter(context.Background(), s, s, r(5, 2), r(1, 3), r(0, 1), band, band, HelicalStartsLineOptions{}); e != ErrDomain {
		t.Fatal(e)
	}
	if _, e = CompareHelicalRetainedQuarter(context.Background(), s, s, r(5, 2), r(0, 1), r(0, 1), band, band, HelicalStartsLineOptions{MaxIntervals: 1}); e == nil {
		t.Fatal("budget")
	}
}
