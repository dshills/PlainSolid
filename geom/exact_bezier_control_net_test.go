package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestEllipseExactBezierControlRoundtrip(t *testing.T) {
	a, b, h := rotatedEllipseFixture(t, false, true)
	arcs, e := BuildSplineGraphQuadraticEllipseArcs(context.Background(), a, b, h, [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}, GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, curve := range arcs.Arcs {
		net, operations, e := curve.BezierControlNet(context.Background(), SurfacePatchOptions{})
		if e != nil || operations == 0 || len(net.Control) != 3 || net.Start != 0 || net.End != 1 {
			t.Fatal(net, operations, e)
		}
		for _, h := range net.Control {
			if h[3].Sign() <= 0 {
				t.Fatal("weight")
			}
		}
		copy, e := NewExactHomogeneousBezier(context.Background(), net.Control, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		equal, e := CompareExactCurvesContext(context.Background(), curve, copy, ExactCurveComparisonOptions{})
		if e != nil || !equal.Complete || !equal.Equal {
			t.Fatal(equal, e)
		}
		before, _ := curve.Eval(.5)
		net.Control[0][0].SetInt64(99)
		after, _ := curve.Eval(.5)
		if before != after {
			t.Fatal("extraction aliases source")
		}
	}
}
func TestExactBezierControlExtractionLimits(t *testing.T) {
	controls := [][4]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}, {big.NewRat(1, 3), big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(2, 3)}}
	curve, e := NewExactHomogeneousBezier(context.Background(), controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	_, _, e = curve.BezierControlNet(context.Background(), SurfacePatchOptions{MaxOperations: 1})
	if e == nil {
		t.Fatal("budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, e = curve.BezierControlNet(ctx, SurfacePatchOptions{})
	if e != context.Canceled {
		t.Fatal(e)
	}
}
