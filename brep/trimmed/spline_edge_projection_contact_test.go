//go:build research

package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestProjectedSplineEdgesIndependentReverseAndAlgebraic(t *testing.T) {
	makeEdge := func(poles []geom.Vec3, lo, hi float64) Solid {
		t.Helper()
		c, e := geom.NewBSpline(2, poles, []float64{lo, lo, lo, hi, hi, hi}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return Solid{edges: []parametric.Edge{{Curve: c, Range: parametric.Interval{Min: lo, Max: hi}}}}
	}
	ctx := context.Background()
	direction := geom.Vec3{X: 1}
	a := makeEdge([]geom.Vec3{{}, {X: .5}, {X: 1, Y: 1}}, 0, 1)
	// A=(x,x²), B=(x,-x²+h) with x=(10-t)/4. Both curves
	// are nonlinear and their original parameter directions disagree.
	b := func(h float64) Solid {
		return makeEdge([]geom.Vec3{{X: 1, Y: h - 1}, {X: .5, Y: h}, {Y: h}}, 6, 10)
	}
	r, e := IntersectSplineEdgesByProjection(ctx, a, 0, b(.5), 0, direction, geom.ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Events) != 1 {
		t.Fatal(r, e)
	}
	p := r.Events[0]
	if p.Exact == nil || !p.Exact.Splittable || !p.Transverse || p.Exact.Anchors[0].Parameter != .5 ||
		p.Exact.Anchors[1].Parameter != 8 || p.Exact.Point[1].Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(p)
	}
	r, e = IntersectSplineEdgesByProjection(ctx, a, 0, b(1), 0, direction, geom.ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Events) != 1 || r.Events[0].Exact != nil {
		t.Fatal(r, e)
	}
	p = r.Events[0]
	toX := func(t *big.Rat) *big.Rat {
		return new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(10, 1), t), big.NewRat(4, 1))
	}
	if !p.Transverse || p.Lower[0].Cmp(toX(p.Upper[1])) != 0 || p.Upper[0].Cmp(toX(p.Lower[1])) != 0 ||
		new(big.Rat).Mul(p.Lower[0], p.Lower[0]).Cmp(big.NewRat(1, 2)) >= 0 ||
		new(big.Rat).Mul(p.Upper[0], p.Upper[0]).Cmp(big.NewRat(1, 2)) <= 0 {
		t.Fatal(p)
	}
	bb := makeEdge([]geom.Vec3{{X: 1, Y: 1}, {X: .5}, {}}, 6, 10)
	r, e = IntersectSplineEdgesByProjection(ctx, a, 0, bb, 0, direction, geom.ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Coincident) != 1 || !r.Coincident[0].Reversed ||
		r.Coincident[0].Lower[1].Cmp(big.NewRat(6, 1)) != 0 || r.Coincident[0].Upper[1].Cmp(big.NewRat(10, 1)) != 0 {
		t.Fatal(r, e)
	}
}
