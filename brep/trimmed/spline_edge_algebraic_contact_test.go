package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestOriginalSplineEdgeAlgebraicRanges(t *testing.T) {
	curve, _ := geom.NewBSpline(2, []geom.Vec3{{}, {X: .5}, {X: 1, Y: 1}}, []float64{10, 10, 10, 14, 14, 14}, nil)
	line, _ := geom.NewBSpline(1, []geom.Vec3{{X: 1, Y: .5}, {Y: .5}}, []float64{2, 2, 6, 6}, nil)
	transforms := []geom.Affine{geom.Identity(), {{2, 1, 0, 3}, {0, 1, 1, -2}, {1, 0, 1, 4}}}
	for _, transform := range transforms {
		ca := parametric.MapCurve(curve, transform)
		cb := parametric.MapCurve(line, transform)
		a := Solid{edges: []parametric.Edge{{Curve: ca, Range: parametric.Interval{Min: 10, Max: 14}}}}
		b := Solid{edges: []parametric.Edge{{Curve: cb, Range: parametric.Interval{Min: 2, Max: 6}}}}
		tiny := float64(1)
		for i := 0; i < 80; i++ {
			tiny /= 2
		}
		skew, _ := geom.NewBSpline(1, []geom.Vec3{{X: 1, Y: .5, Z: tiny}, {Y: .5, Z: tiny}}, []float64{2, 2, 6, 6}, nil)
		skewSolid := Solid{edges: []parametric.Edge{{Curve: parametric.MapCurve(skew, transform), Range: parametric.Interval{Min: 2, Max: 6}}}}
		separated, e := IntersectSplineEdgesAlgebraic(context.Background(), a, 0, skewSolid, 0, geom.ExactCurveSegmentOptions{})
		if e != nil || !separated.Complete || len(separated.Contacts)+len(separated.Algebraic) != 0 {
			t.Fatal(separated, e)
		}
		for swapped, pair := range [2][2]Solid{{a, b}, {b, a}} {
			r, e := IntersectSplineEdgesAlgebraic(context.Background(), pair[0], 0, pair[1], 0, geom.ExactCurveSegmentOptions{})
			if e != nil || !r.Complete || len(r.Algebraic) != 1 || len(r.Contacts) != 0 {
				t.Fatal(r, e)
			}
			hit := r.Algebraic[0]
			if hit.AtEnd != ([2]bool{}) {
				t.Fatal(hit)
			}
			// Original A t=10+4sqrt(1/2); reversed B t=6-4sqrt(1/2).
			lo := new(big.Rat).Quo(new(big.Rat).Sub(hit.Lower[swapped], big.NewRat(10, 1)), big.NewRat(4, 1))
			hi := new(big.Rat).Quo(new(big.Rat).Sub(hit.Upper[swapped], big.NewRat(10, 1)), big.NewRat(4, 1))
			checkSqrtHalfBracket(t, lo, hi)
			lo = new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(6, 1), hit.Upper[1-swapped]), big.NewRat(4, 1))
			hi = new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(6, 1), hit.Lower[1-swapped]), big.NewRat(4, 1))
			checkSqrtHalfBracket(t, lo, hi)
		}
	}
}
func checkSqrtHalfBracket(t *testing.T, lo, hi *big.Rat) {
	t.Helper()
	if lo.Sign() < 0 || lo.Cmp(hi) > 0 || new(big.Rat).Mul(lo, lo).Cmp(big.NewRat(1, 2)) > 0 || new(big.Rat).Mul(hi, hi).Cmp(big.NewRat(1, 2)) < 0 || new(big.Rat).Sub(hi, lo).Cmp(big.NewRat(1, 10000000)) > 0 {
		t.Fatal(lo, hi)
	}
}
func TestReversedIsoEdgeAlgebraicRanges(t *testing.T) {
	var poles []geom.Vec3
	for _, p := range []geom.Vec3{{}, {X: .5}, {X: 1, Y: 1}} {
		poles = append(poles, p, geom.Vec3{X: p.X, Y: p.Y, Z: 1})
	}
	surface, _ := geom.NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	curve := surfaceTrimCurve{surface: surface, a: geom.Vec2{X: 1}, b: geom.Vec2{}}
	line, _ := geom.NewBezier([]geom.Vec3{{Y: .5}, {X: 1, Y: .5}}, nil)
	a := Solid{edges: []parametric.Edge{{Curve: curve, Range: parametric.Interval{Max: 1}}}}
	b := Solid{edges: []parametric.Edge{{Curve: line, Range: parametric.Interval{Max: 1}}}}
	r, e := IntersectSplineEdgesAlgebraic(context.Background(), a, 0, b, 0, geom.ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Algebraic) != 1 {
		t.Fatal(r, e)
	}
	hit := r.Algebraic[0]
	checkSqrtHalfBracket(t, new(big.Rat).Sub(big.NewRat(1, 1), hit.Upper[0]), new(big.Rat).Sub(big.NewRat(1, 1), hit.Lower[0]))
	checkSqrtHalfBracket(t, hit.Lower[1], hit.Upper[1])
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = IntersectSplineEdgesAlgebraic(canceled, a, 0, b, 0, geom.ExactCurveSegmentOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	r, e = IntersectSplineEdgesAlgebraic(context.Background(), a, 0, b, 0, geom.ExactCurveSegmentOptions{MaxOperations: 1})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
