//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCurvedSplineEdgesAlgebraicConic(t *testing.T) {
	// A(t)=(q²,q), q=(t-10)/4. B is the reversed rational unit-circle
	// quadrant on [2,6]. Contact q satisfies q^4+q²-1=0.
	aCurve, _ := geom.NewBSpline(2, []geom.Vec3{{}, {Y: .5}, {X: 1, Y: 1}}, []float64{10, 10, 10, 14, 14, 14}, nil)
	bCurve, _ := geom.NewBSpline(2, []geom.Vec3{{Y: 1}, {X: 1, Y: 1}, {X: 1}}, []float64{2, 2, 2, 6, 6, 6}, []float64{2, 1, 1})
	for _, m := range []geom.Affine{geom.Identity(), {{2, 1, 0, 3}, {0, 1, 1, -2}, {1, 0, 1, 4}}} {
		a := Solid{edges: []parametric.Edge{{Curve: parametric.MapCurve(aCurve, m), Range: parametric.Interval{Min: 10, Max: 14}}}}
		b := Solid{edges: []parametric.Edge{{Curve: parametric.MapCurve(bCurve, m), Range: parametric.Interval{Min: 2, Max: 6}}}}
		for swap, pair := range [2][2]Solid{{a, b}, {b, a}} {
			r, e := IntersectSplineEdgesQuadraticArc(context.Background(), pair[0], 0, pair[1], 0, geom.ExactCurveSegmentOptions{})
			if e != nil || !r.Complete || len(r.Algebraic) != 1 || len(r.Contacts)+len(r.Overlaps) != 0 {
				t.Fatal(r, e)
			}
			hit := r.Algebraic[0]
			lo := new(big.Rat).Quo(new(big.Rat).Sub(hit.Lower[swap], big.NewRat(10, 1)), big.NewRat(4, 1))
			hi := new(big.Rat).Quo(new(big.Rat).Sub(hit.Upper[swap], big.NewRat(10, 1)), big.NewRat(4, 1))
			checkQuarticContactBracket(t, lo, hi, 1)
			lo = new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(6, 1), hit.Upper[1-swap]), big.NewRat(4, 1))
			hi = new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(6, 1), hit.Lower[1-swap]), big.NewRat(4, 1))
			checkQuarticContactBracket(t, lo, hi, 4)
		}
		// Both projected curves remain identical, but an exact 2^-80 normal
		// displacement destroys incidence, even under oblique placement.
		delta := float64(1)
		for i := 0; i < 80; i++ {
			delta /= 2
		}
		skew, _ := geom.NewBSpline(2, []geom.Vec3{{Z: delta}, {Y: .5, Z: delta}, {X: 1, Y: 1, Z: delta}}, []float64{10, 10, 10, 14, 14, 14}, nil)
		a.edges[0].Curve = parametric.MapCurve(skew, m)
		r, e := IntersectSplineEdgesQuadraticArc(context.Background(), a, 0, b, 0, geom.ExactCurveSegmentOptions{})
		if e != nil || !r.Complete || len(r.Contacts)+len(r.Algebraic)+len(r.Overlaps) != 0 {
			t.Fatal(r, e)
		}
	}
}
func checkQuarticContactBracket(t *testing.T, lo, hi *big.Rat, k int64) {
	t.Helper()
	f := func(x *big.Rat) *big.Rat {
		x2 := new(big.Rat).Mul(x, x)
		return new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(x2, x2), new(big.Rat).Mul(big.NewRat(k, 1), x2)), big.NewRat(1, 1))
	}
	if lo.Sign() < 0 || lo.Cmp(hi) > 0 || f(lo).Sign() > 0 || f(hi).Sign() < 0 || new(big.Rat).Sub(hi, lo).Cmp(big.NewRat(1, 1000000)) > 0 {
		t.Fatal(lo, hi)
	}
}
func TestCurvedSplineEdgeExactOverlapMap(t *testing.T) {
	aCurve, _ := geom.NewBSpline(2, []geom.Vec3{{X: 1}, {X: 1, Y: 1}, {Y: 1}}, []float64{10, 10, 10, 14, 14, 14}, []float64{1, 1, 2})
	bCurve, _ := geom.NewBSpline(2, []geom.Vec3{{Y: 1}, {X: 1, Y: 1}, {X: 1}}, []float64{2, 2, 2, 6, 6, 6}, []float64{2, 1, 1})
	a := Solid{edges: []parametric.Edge{{Curve: aCurve, Range: parametric.Interval{Min: 10, Max: 14}}}}
	b := Solid{edges: []parametric.Edge{{Curve: bCurve, Range: parametric.Interval{Min: 2, Max: 6}}}}
	r, e := IntersectSplineEdgesQuadraticArc(context.Background(), a, 0, b, 0, geom.ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Overlaps) != 1 {
		t.Fatal(r, e)
	}
	q := r.Overlaps[0]
	if q.Offset.Cmp(big.NewRat(16, 1)) != 0 || q.Scale.Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal(q)
	}
	r, e = IntersectSplineEdgesQuadraticArc(context.Background(), a, 0, b, 0, geom.ExactCurveSegmentOptions{MaxOperations: 1})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
