package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestAffineProjectedGraphExactOriginalEvidence(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{2*u + v, 7, u + 2*v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u + 2*v, 3, 2*u + v} })
	for _, axes := range [][2]int{{0, 2}, {2, 0}} {
		ga, e := NewAffineProjectedGraph(ctx, a, axes, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		gb, e := NewAffineProjectedGraph(ctx, b, axes, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
		r, e := ga.CompareHeightAtRational(ctx, gb, uv, SurfacePatchEvaluationOptions{})
		if e != nil || !r.Complete || r.ResidualAxis != 1 || r.Sign != 1 || r.Difference.Cmp(big.NewRat(4, 1)) != 0 || r.SourcePoint[1].Cmp(big.NewRat(7, 1)) != 0 || r.TargetPoint[1].Cmp(big.NewRat(3, 1)) != 0 {
			t.Fatal(r, e)
		}
		if r.TargetUV[0].Cmp(uv[1]) != 0 || r.TargetUV[1].Cmp(uv[0]) != 0 {
			t.Fatal("chart inverse", r)
		}
		evaluation, e := ga.EvaluateRational(ctx, uv, SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		location, e := gb.InvertRational(ctx, [2]*big.Rat{evaluation.Point[axes[0]], evaluation.Point[axes[1]]}, SurfacePatchEvaluationOptions{})
		if e != nil || !location.Inside || location.UV[0].Cmp(uv[1]) != 0 {
			t.Fatal(location, e)
		}
		inverse, _ := ga.InverseCoefficients()
		inverse[0][0].SetInt64(999)
		again, _ := ga.InverseCoefficients()
		if again[0][0].Cmp(inverse[0][0]) == 0 {
			t.Fatal("alias")
		}
		clipped, _ := b.Trim(ParameterRectangle{0, .125, 0, .125})
		gc, e := NewAffineProjectedGraph(ctx, clipped, axes, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		outside, e := ga.CompareHeightAtRational(ctx, gc, uv, SurfacePatchEvaluationOptions{})
		if e != nil || !outside.TargetOutside || outside.Difference != nil || outside.SourcePoint[1].Cmp(big.NewRat(7, 1)) != 0 {
			t.Fatal(outside, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := NewAffineProjectedGraph(cancelled, a, [2]int{0, 2}, SurfacePatchOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := NewAffineProjectedGraph(ctx, a, [2]int{0, 2}, SurfacePatchOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	if _, e := NewAffineProjectedGraph(ctx, a, [2]int{0, 1}, SurfacePatchOptions{}); e == nil {
		t.Fatal("singular projection")
	}
}
