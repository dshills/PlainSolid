package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestWrappedSurfaceChartRelation(t *testing.T) {
	ctx := context.Background()
	p := wrappedCylinderPatch(t)
	a, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := a.InsertKnot(ctx, 0, .125, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e = b.InsertKnot(ctx, 1, .75, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if SameWrappedSurfaceRecipe(a, b) {
		t.Fatal("fixture must have independent knot bases")
	}
	relation, proof, e := CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{})
	if e != nil || !proof.Complete || !proof.Equal || relation == nil || relation.ChartOrientation() != 1 {
		t.Fatal(proof, e)
	}
	b, _, e = b.Reparameterize(ctx, ParameterRectangle{UMin: 2, UMax: 10, VMin: -3, VMax: 7}, true, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	relation, proof, e = CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true, ReverseBU: true})
	if e != nil || !proof.Equal || relation.ChartOrientation() != -1 {
		t.Fatal(proof, e)
	}
	for _, u := range []*big.Rat{big.NewRat(0, 1), big.NewRat(4, 1), big.NewRat(13, 3), big.NewRat(-101, 3)} {
		uv := [2]*big.Rat{u, big.NewRat(2, 5)}
		mapped, e := relation.MapRational(ctx, uv, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := new(big.Rat).Sub(big.NewRat(10, 1), new(big.Rat).Mul(big.NewRat(2, 1), u))
		if mapped[0].Cmp(want) != 0 {
			t.Fatal("lost lifted winding", mapped, want)
		}
		x, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		y, e := b.EvaluateRational(ctx, mapped[0], mapped[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for j := 0; j < 3; j++ {
			if x.Point[j].Cmp(y.Point[j]) != 0 {
				t.Fatal("model identity", u, x.Point, y.Point)
			}
		}
		mapped[0].SetInt64(999)
		again, e := relation.MapRational(ctx, uv, SurfacePatchOptions{})
		if e != nil || again[0].Cmp(want) != 0 {
			t.Fatal("map alias", again, e)
		}
	}
	if _, e = relation.MapRational(ctx, [2]*big.Rat{big.NewRat(0, 1), big.NewRat(4, 1)}, SurfacePatchOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	if _, e = relation.MapRational(ctx, [2]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1)}, SurfacePatchOptions{MaxOperations: 1}); e == nil {
		t.Fatal("map budget")
	}
	none, proof, e := CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true})
	if e != nil || !proof.Complete || proof.Equal || none != nil {
		t.Fatal("wrong reversal", proof, e)
	}
	none, proof, e = CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true, ReverseBU: true, MaxOperations: 1})
	if e == nil || none != nil || proof.Complete {
		t.Fatal("atomic budget", proof, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	none, proof, e = CompareWrappedSurfaceCharts(cancelled, a, b, ExactSurfaceComparisonOptions{})
	if !errors.Is(e, context.Canceled) || none != nil || proof.Complete {
		t.Fatal("cancel", proof, e)
	}
	if _, _, e = CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{SwapB: true, NormalizeDomains: true}); !errors.Is(e, ErrDomain) {
		t.Fatal("wrap mismatch", e)
	}
}

func TestWrappedSurfaceChartRelationSwappedAxes(t *testing.T) {
	ctx := context.Background()
	p := wrappedCylinderPatch(t)
	a, _, e := NewWrappedSurfacePatch(ctx, p, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	spline, e := p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	spline, e = spline.Transposed()
	if e != nil {
		t.Fatal(e)
	}
	target, e := NewSurfacePatch(spline, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := NewWrappedSurfacePatch(ctx, target, WrappedSurfaceOptions{WrapV: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	relation, proof, e := CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{SwapB: true})
	if e != nil || !proof.Equal || relation.ChartOrientation() != -1 {
		t.Fatal(proof, e)
	}
	uv := [2]*big.Rat{big.NewRat(101, 3), big.NewRat(-1, 7)}
	mapped, e := relation.MapRational(ctx, uv, SurfacePatchOptions{})
	if e != nil || mapped[0].Cmp(uv[1]) != 0 || mapped[1].Cmp(uv[0]) != 0 {
		t.Fatal(mapped, e)
	}
	x, e := a.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	y, e := b.EvaluateRational(ctx, mapped[0], mapped[1], SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for j := 0; j < 3; j++ {
		if x.Point[j].Cmp(y.Point[j]) != 0 {
			t.Fatal(x, y)
		}
	}
}
