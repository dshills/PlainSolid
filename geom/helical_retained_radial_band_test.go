package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalRetainedRadialBand(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, .25}, {2, .5}}, 2, RightHanded, 0, .5)
	s, _ := NewSeparatedHelicalStarts(p, 1)
	r := big.NewRat
	radial := [2]*big.Rat{r(2, 1), r(3, 1)}
	stock := [2]*big.Rat{r(1, 5), r(3, 10)}
	q, e := CompareHelicalRetainedRadialBand(context.Background(), s, s, r(0, 1), r(0, 1), radial, stock, stock, HelicalStartsLineOptions{})
	if e != nil || !q.Complete || q.NoPositiveOverlap || q.WitnessRadius == nil || q.WitnessRadius.Cmp(r(14, 5)) <= 0 || !q.Witness.Overlap {
		t.Fatal(q, e)
	}
	found := false
	for _, v := range q.Events {
		if v.Cmp(r(14, 5)) == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal(q.Events)
	}
	q, e = CompareHelicalRetainedRadialBand(context.Background(), s, s, r(0, 1), r(0, 1), [2]*big.Rat{r(2, 1), r(14, 5)}, stock, stock, HelicalStartsLineOptions{})
	if e != nil || !q.NoPositiveOverlap || !q.Complete {
		t.Fatal(q, e)
	}
	q, e = CompareHelicalRetainedRadialBand(context.Background(), s, s, r(1, 3), r(2, 1), radial, stock, stock, HelicalStartsLineOptions{})
	if e != nil || !q.NoPositiveOverlap {
		t.Fatal(q, e)
	}
	if _, e = CompareHelicalRetainedRadialBand(context.Background(), s, s, r(0, 1), r(0, 1), radial, stock, stock, HelicalStartsLineOptions{MaxIntervals: 2}); e == nil {
		t.Fatal("budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = CompareHelicalRetainedRadialBand(ctx, s, s, r(0, 1), r(0, 1), radial, stock, stock, HelicalStartsLineOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}
