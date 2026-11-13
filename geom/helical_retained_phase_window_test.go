package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalRetainedPhaseWindow(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, 0}, {3, .25}, {2, .25}}, 1, RightHanded, 0, 1)
	s, _ := NewSeparatedHelicalStarts(p, 1)
	r := big.NewRat
	phase := [2]*big.Rat{r(0, 1), r(3, 4)}
	band := [2]*big.Rat{r(0, 1), r(2, 1)}
	q, e := CompareHelicalRetainedPhaseWindow(context.Background(), s, s, r(5, 2), r(0, 1), phase, band, band, HelicalStartsLineOptions{})
	if e != nil || !q.Complete || q.NoPositiveOverlap || q.WitnessPhase == nil || !q.Witness.Overlap {
		t.Fatal(q, e)
	}
	q, e = CompareHelicalRetainedPhaseWindow(context.Background(), s, s, r(5, 2), r(2, 1), phase, band, band, HelicalStartsLineOptions{})
	if e != nil || !q.NoPositiveOverlap || !q.Complete || q.Probes < 2 {
		t.Fatal(q, e)
	}
	// A narrow stock is initially wholly cut, but finite phase advance exposes
	// positive retained length. Endpoint order must be split at 1/10 and 1/5.
	narrow := [2]*big.Rat{r(1, 10), r(1, 5)}
	q, e = CompareHelicalRetainedPhaseWindow(context.Background(), s, s, r(5, 2), r(0, 1), phase, narrow, narrow, HelicalStartsLineOptions{})
	if e != nil || q.NoPositiveOverlap || q.WitnessPhase.Cmp(r(1, 10)) <= 0 {
		t.Fatal(q, e)
	}
	found := false
	for _, v := range q.Events {
		if v.Cmp(r(1, 10)) == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal(q.Events)
	}
	if _, e = CompareHelicalRetainedPhaseWindow(context.Background(), s, s, r(5, 2), r(0, 1), phase, band, band, HelicalStartsLineOptions{MaxIntervals: 1}); e == nil {
		t.Fatal("budget")
	}
}
