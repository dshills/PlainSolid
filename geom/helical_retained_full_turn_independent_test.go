package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestFullTurnMixedStartsNonbinaryShift(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	frame, _ := NewFrame(Vec3{X: 3, Y: -2, Z: 7}, Vec3{Y: 1}, Vec3{X: -1})
	profile := []Vec2{{2, 0}, {3, 0}, {3, .25}, {2, .25}}
	for _, hand := range []HelixHandedness{RightHanded, LeftHanded} {
		p, e := NewHelicalPolygon(frame, profile, 2, hand, -.125, 1.125)
		if e != nil {
			t.Fatal(e)
		}
		a, _ := NewSeparatedHelicalStarts(p, 2)
		b, _ := NewSeparatedHelicalStarts(p, 3)
		radial := [2]*big.Rat{r(9, 4), r(11, 4)}
		// Independently disjoint stocks: [0,1/4] and [1/3,7/12].
		narrow := [2]*big.Rat{r(0, 1), r(1, 4)}
		clear, e := CompareHelicalRetainedFullTurn(ctx, a, b, r(1, 3), radial, narrow, narrow, HelicalStartsLineOptions{})
		if e != nil || !clear.Complete || !clear.NoPositiveOverlap || clear.WitnessRadius != nil {
			t.Fatal(hand, clear, e)
		}
		// At phase 1/3 only source turn n=0 exists. A cuts [2/3,11/12]
		// and [5/3,23/12]. B shifted by 1/3 cuts [1,5/4],
		// [5/3,23/12], [7/3,31/12]. Thus [0,2/3] of A overlaps
		// the shifted B stock from 1/3, giving exactly [1/3,2/3].
		stock := [2]*big.Rat{r(0, 1), r(3, 1)}
		witness, e := compareHelicalRetainedPhase(ctx, a, b, r(5, 2), r(1, 3), r(1, 3), stock, stock, HelicalStartsLineOptions{})
		if e != nil || !witness.Overlap || len(witness.Contacts) == 0 || witness.Contacts[0].Bounds[0].Cmp(r(1, 3)) != 0 || witness.Contacts[0].Bounds[1].Cmp(r(2, 3)) != 0 {
			t.Fatal(hand, witness, e)
		}
		if witness.A[0].Boundary[1].Start != 0 || witness.A[0].Boundary[1].Turn != 0 {
			t.Fatal(witness.A)
		}
		full, e := CompareHelicalRetainedFullTurn(ctx, a, b, r(1, 3), radial, stock, stock, HelicalStartsLineOptions{})
		if e != nil || !full.Complete || full.NoPositiveOverlap || !full.Witness.Overlap {
			t.Fatal(hand, full, e)
		}
		// Both physical source turns 0 and 1 lie in the finite window on the
		// seam. Phase one labels them -1 and 0; geometry must remain identical.
		q0, e := compareHelicalRetainedPhase(ctx, a, b, r(5, 2), r(0, 1), r(1, 3), stock, stock, HelicalStartsLineOptions{})
		if e != nil {
			t.Fatal(e)
		}
		q1, e := compareHelicalRetainedPhase(ctx, a, b, r(5, 2), r(1, 1), r(1, 3), stock, stock, HelicalStartsLineOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for side, x := range [][]HelicalRetainedInterval{q0.A, q0.B} {
			y := [][]HelicalRetainedInterval{q1.A, q1.B}[side]
			if len(x) != len(y) {
				t.Fatal(x, y)
			}
			for i, v := range x {
				for j := 0; j < 2; j++ {
					if v.Bounds[j].Cmp(y[i].Bounds[j]) != 0 {
						t.Fatal(v, y[i])
					}
					if !v.Boundary[j].StockCap && (v.Boundary[j].Start != y[i].Boundary[j].Start || v.Boundary[j].Turn != y[i].Boundary[j].Turn+1) {
						t.Fatal(v, y[i])
					}
				}
			}
		}
	}
	left, _ := NewHelicalPolygon(frame, profile, 2, LeftHanded, 0, 1)
	right, _ := NewHelicalPolygon(frame, profile, 2, RightHanded, 0, 1)
	a, _ := NewSeparatedHelicalStarts(left, 2)
	b, _ := NewSeparatedHelicalStarts(right, 3)
	bad, e := CompareHelicalRetainedFullTurn(ctx, a, b, r(1, 3), [2]*big.Rat{r(2, 1), r(3, 1)}, [2]*big.Rat{r(0, 1), r(1, 1)}, [2]*big.Rat{r(0, 1), r(1, 1)}, HelicalStartsLineOptions{})
	if e != ErrDomain || bad.Complete || bad.NoPositiveOverlap {
		t.Fatal(bad, e)
	}
}
