package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalRetainedJointWindow(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, .25}, {2, .5}}, 2, RightHanded, 0, .5)
	s, _ := NewSeparatedHelicalStarts(p, 1)
	r := big.NewRat
	radial := [2]*big.Rat{r(2, 1), r(3, 1)}
	phase := [2]*big.Rat{r(0, 1), r(1, 10)}
	stock := [2]*big.Rat{r(1, 5), r(3, 10)}
	q, e := CompareHelicalRetainedJointWindow(context.Background(), s, s, r(0, 1), radial, phase, stock, stock, HelicalStartsLineOptions{})
	if e != nil || !q.Complete || q.NoPositiveOverlap || q.WitnessRadius == nil || q.WitnessPhase == nil || !q.Witness.Overlap {
		t.Fatal(q, e)
	}
	found := false
	for _, v := range q.RadiusEvents {
		if v.Cmp(r(14, 5)) == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal(q.RadiusEvents)
	}
	q, e = CompareHelicalRetainedJointWindow(context.Background(), s, s, r(0, 1), [2]*big.Rat{r(2, 1), r(5, 2)}, [2]*big.Rat{r(0, 1), r(1, 32)}, stock, stock, HelicalStartsLineOptions{})
	if e != nil || !q.Complete || !q.NoPositiveOverlap {
		t.Fatal(q, e)
	}
	q, e = CompareHelicalRetainedJointWindow(context.Background(), s, s, r(2, 1), radial, phase, stock, stock, HelicalStartsLineOptions{})
	if e != nil || !q.NoPositiveOverlap {
		t.Fatal(q, e)
	}
	if _, e = CompareHelicalRetainedJointWindow(context.Background(), s, s, r(0, 1), radial, phase, stock, stock, HelicalStartsLineOptions{MaxIntervals: 2}); e == nil {
		t.Fatal("cap")
	}
}

func TestHelicalJointInteriorOnlyAndSeam(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	r := big.NewRat
	pa, _ := NewHelicalPolygon(f, []Vec2{{2, -1}, {4, -1}, {4, .25}, {2, 1}}, 4, RightHanded, 0, 1)
	pb, _ := NewHelicalPolygon(f, []Vec2{{2, -1}, {4, -1}, {4, 1}, {2, .25}}, 4, RightHanded, 0, 1)
	a, _ := NewSeparatedHelicalStarts(pa, 1)
	b, _ := NewSeparatedHelicalStarts(pb, 1)
	stock := [2]*big.Rat{r(0, 1), r(1, 1)}
	phases := [2]*big.Rat{r(0, 1), r(1, 32)}
	radial := [2]*big.Rat{r(2, 1), r(4, 1)}
	// Independent endpoint argument: one cutter has [-1,1]+4phase;
	// its lower stays <=-7/8 and upper >=1, so it covers all stock.
	for _, radius := range radial {
		q, e := CompareHelicalRetainedPhaseWindow(context.Background(), a, b, radius, r(0, 1), phases, stock, stock, HelicalStartsLineOptions{})
		if e != nil || !q.NoPositiveOverlap {
			t.Fatal(q, e)
		}
	}
	q, e := CompareHelicalRetainedJointWindow(context.Background(), a, b, r(0, 1), radial, phases, stock, stock, HelicalStartsLineOptions{})
	if e != nil || q.NoPositiveOverlap || q.WitnessRadius.Cmp(radial[0]) <= 0 || q.WitnessRadius.Cmp(radial[1]) >= 0 {
		t.Fatal(q, e)
	}
	// At r=3 both uppers are 5/8+4phase <=3/4, leaving [3/4,1].
	mid, e := compareHelicalRetainedPhase(context.Background(), a, b, r(3, 1), r(1, 64), r(0, 1), stock, stock, HelicalStartsLineOptions{})
	if e != nil || !mid.Overlap || mid.Contacts[0].Bounds[0].Cmp(r(11, 16)) != 0 {
		t.Fatal(mid, e)
	}
	var seam [2]HelicalRetainedPair
	for i := 0; i < 2; i++ {
		seam[i], e = compareHelicalRetainedPhase(context.Background(), a, b, r(3, 1), r(int64(i), 1), r(0, 1), stock, stock, HelicalStartsLineOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(seam[0].A) != len(seam[1].A) {
		t.Fatal(seam)
	}
	for i, v := range seam[0].A {
		w := seam[1].A[i]
		for j := 0; j < 2; j++ {
			if v.Bounds[j].Cmp(w.Bounds[j]) != 0 {
				t.Fatal(seam)
			}
			if !v.Boundary[j].StockCap && v.Boundary[j].Turn != w.Boundary[j].Turn+1 {
				t.Fatal(v, w)
			}
		}
	}
	full, e := CompareHelicalRetainedFullTurn(context.Background(), a, b, r(3, 1), radial, stock, stock, HelicalStartsLineOptions{})
	if e != nil || !full.Complete || !full.NoPositiveOverlap {
		t.Fatal(full, e)
	}
	failed, e := CompareHelicalRetainedFullTurn(context.Background(), a, b, r(0, 1), radial, stock, stock, HelicalStartsLineOptions{HelicalPolygonLineOptions: HelicalPolygonLineOptions{MaxOperations: 5}})
	if e == nil || failed.Complete || failed.NoPositiveOverlap {
		t.Fatal(failed, e)
	}
}
