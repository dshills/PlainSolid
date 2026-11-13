package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestExactSurfaceCurveComposition(t *testing.T) {
	for _, rational := range []bool{false, true} {
		var poles []Vec3
		var weights []float64
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				z := float64(i*j) / 4
				if i == 2 {
					z++
				}
				if j == 2 {
					z++
				}
				poles = append(poles, Vec3{float64(i) / 2, float64(j) / 2, z})
				w := 1.
				if rational && i == 1 {
					w = .5
				}
				weights = append(weights, w)
			}
		}
		s, e := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, weights)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		controls := [][4]*big.Rat{{big.NewRat(1, 8), big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(3, 8), big.NewRat(1, 4), new(big.Rat), big.NewRat(1, 2)}, {big.NewRat(1, 2), big.NewRat(7, 8), new(big.Rat), big.NewRat(1, 1)}}
		uv, e := NewExactHomogeneousBezier(context.Background(), controls, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		model, e := patch.ComposeParameterCurve(context.Background(), uv, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if !rational && model.patch.degreeU != 4 {
			t.Fatal("polynomial total-degree2 composition should reduce to quartic", model.patch.degreeU)
		}
		for _, parameter := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 7), big.NewRat(2, 3), big.NewRat(1, 1)} {
			chart, e := uv.patch.EvaluateRational(context.Background(), parameter, new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want, e := patch.EvaluateRational(context.Background(), chart.Point[0], chart.Point[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			got, e := model.patch.EvaluateRational(context.Background(), parameter, new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if got.Point[axis].Cmp(want.Point[axis]) != 0 {
					t.Fatal("exact composition", rational, parameter, axis)
				}
			}
		}
		_, e = patch.ComposeParameterCurve(context.Background(), uv, SurfacePatchOptions{MaxOperations: 1})
		if e == nil {
			t.Fatal("budget")
		}
	}
}
func TestExactSurfaceCurveCompositionHullAndCancellation(t *testing.T) {
	a, _ := graphContactFixture(t, false, false)
	uv, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(2, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.ComposeParameterCurve(context.Background(), uv, SurfacePatchOptions{}); e == nil {
		t.Fatal("outside hull")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = a.ComposeParameterCurve(ctx, uv, SurfacePatchOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}
