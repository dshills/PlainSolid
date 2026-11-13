package geom

import (
	"context"
	"math"
	"testing"
)

func TestQuadraticExactLengthTightGeometricBound(t *testing.T) {
	ctx := context.Background()
	c, e := NewBSpline(2, []Vec3{{}, {X: 1, Y: 1}, {X: 2}}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	exact, e := NewExactSplineCurve(c, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundExactCurveLengthContext(ctx, exact, CurveLengthBoundOptions{Tolerance: 1e-4})
	want := math.Sqrt2 + math.Asinh(1)
	if e != nil || !r.Complete || r.Lower > want || r.Upper < want || r.Upper-r.Lower > 1e-4 || r.ExactOperations > 200000 {
		t.Fatal(want, r, e)
	}
	// Extreme positive weights keep the geometric enclosure valid. A single
	// span's upper bound is exactly the control polygon, with no division by
	// the minimum weight and no assumption that the curve is circular.
	c, e = NewBSpline(2, []Vec3{{}, {X: 1, Y: 1, Z: 1}, {X: 2}}, []float64{0, 0, 0, 1, 1, 1}, []float64{1, 1024, .25})
	if e != nil {
		t.Fatal(e)
	}
	exact, e = NewExactSplineCurve(c, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	budget := &polynomialBudget{maxOperations: 100000, maxBits: 16384}
	lo, hi, e := exactSpanLengthBounds(exact.patch, budget)
	if e != nil || lo > 2 || hi < 2*math.Sqrt(3) || hi > 2*math.Sqrt(3)+1e-14 {
		t.Fatal(lo, hi, e)
	}
	// Higher degrees must retain the general derivative fallback.
	cubic, e := NewBSpline(3, []Vec3{{}, {X: 1}, {X: 2}, {X: 3}}, []float64{0, 0, 0, 0, 1, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	exact, e = NewExactSplineCurve(cubic, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = BoundExactCurveLengthContext(ctx, exact, CurveLengthBoundOptions{Tolerance: .01})
	if e != nil || r.Lower > 3 || r.Upper < 3 {
		t.Fatal(r, e)
	}
}
