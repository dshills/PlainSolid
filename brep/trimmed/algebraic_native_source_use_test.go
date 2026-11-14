package trimmed

import (
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestNativeAlgebraicCurvedSourceUse(t *testing.T) {
	r := func(n int64) *big.Rat { return big.NewRat(n, 1) }
	quadratic, e := NewExactQuadraticUV([3][3]*big.Rat{{r(0), r(0), r(1)}, {big.NewRat(3, 2), r(0), r(1)}, {r(3), r(9), r(1)}})
	if e != nil {
		t.Fatal(e)
	}
	cubic, e := NewExactBezierUV([][3]*big.Rat{{r(0), r(0), r(1)}, {r(1), r(0), r(1)}, {r(2), r(0), r(1)}, {r(3), r(27), r(1)}})
	if e != nil {
		t.Fatal(e)
	}
	type exactEval interface {
		EvaluateRational(*big.Rat) ([2]*big.Rat, [2]*big.Rat, error)
	}
	for _, curve := range []PCurve{quadratic, cubic} {
		for _, sourceReverse := range []bool{false, true} {
			for _, resultReverse := range []bool{false, true} {
				start, end := geom.Vec2{}, geom.Vec2{X: 3, Y: 9}
				if _, ok := curve.(ExactBezierUV); ok {
					end.Y = 27
				}
				authored := curve
				if sourceReverse {
					authored = reversePCurve(curve)
					start, end = end, start
				}
				use := Use{Edge: 7, UVStart: start, UVEnd: end, Curve: authored, Reversed: sourceReverse}
				f := Face{Holes: [][]Use{{use}}}
				out, e := nativeAlgebraicSourceUse(f, 7, resultReverse, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)})
				if e != nil {
					t.Fatalf("%T sourceReverse=%v resultReverse=%v: %v", curve, sourceReverse, resultReverse, e)
				}
				var net [][3]*big.Rat
				switch q := out.Curve.(type) {
				case ExactQuadraticUV:
					h := q.HomogeneousControls()
					net = h[:]
				case ExactBezierUV:
					net = q.HomogeneousControls()
				}
				expectedY := []int64{1, 2, 4}
				if len(net) == 4 {
					expectedY = []int64{1, 2, 4, 8}
				}
				for i, h := range net {
					j := i
					if resultReverse {
						j = len(net) - 1 - i
					}
					wantU := new(big.Rat).Add(r(1), big.NewRat(int64(j), int64(len(net)-1)))
					if h[0].Cmp(wantU) != 0 || h[1].Cmp(r(expectedY[j])) != 0 || h[2].Cmp(r(1)) != 0 {
						t.Fatal("full restricted polynomial identity", net)
					}
				}
				if out.Curve == nil || out.Reversed != resultReverse {
					t.Fatal("lost curve/orientation")
				}
				for n := 0; n <= 4; n++ {
					q := big.NewRat(int64(n), 4)
					originalT := new(big.Rat).Set(q)
					if resultReverse {
						originalT.Sub(r(1), originalT)
					}
					originalT.Add(big.NewRat(1, 3), new(big.Rat).Quo(originalT, r(3)))
					want, _, e := curve.(exactEval).EvaluateRational(originalT)
					if e != nil {
						t.Fatal(e)
					}
					got, _, e := out.Curve.(exactEval).EvaluateRational(q)
					if e != nil {
						t.Fatal(e)
					}
					for j := range got {
						if got[j].Cmp(want[j]) != 0 {
							t.Fatal("continuous source correspondence", got, want)
						}
					}
				}
				bad := f
				bad.Outer = []Use{use}
				if _, e = nativeAlgebraicSourceUse(bad, 7, false, [2]*big.Rat{r(0), r(1)}); e == nil {
					t.Fatal("ambiguous duplicate")
				}
			}
		}
	}
	// Curved source endpoints cannot be silently replaced by rounded native UV.
	f := Face{Outer: []Use{{Edge: 0, UVStart: quadratic.ends[0], UVEnd: quadratic.ends[1], Curve: quadratic}}}
	if _, e = nativeAlgebraicSourceUse(f, 0, false, [2]*big.Rat{big.NewRat(1, 7), big.NewRat(2, 3)}); e == nil {
		t.Fatal("unrepresentable endpoint")
	}
	f.Outer[0].UVEnd.X = 4
	if _, e = nativeAlgebraicSourceUse(f, 0, false, [2]*big.Rat{r(0), r(1)}); e == nil {
		t.Fatal("stale source endpoint")
	}
}
