package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestTrapezoidChartExactIsoClip(t *testing.T) {
	ctx := context.Background()
	target := rationalPlanePointTube(t, false, false).b
	if _, e := NewAffineSurfaceChart(ctx, target, AffineSurfaceChartOptions{}); e == nil {
		t.Fatal("nonaffine fixture")
	}
	refined, err := target.InsertUKnot(.5, 1, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	refined, err = refined.InsertVKnot(.75, 1, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewTrapezoidSurfaceChart(ctx, refined, AffineSurfaceChartOptions{}); err != nil {
		t.Fatal("original knot proof", err)
	}
	reversed := rationalPlanePointTube(t, true, false).b
	if _, err = NewTrapezoidSurfaceChart(ctx, reversed, AffineSurfaceChartOptions{}); err != nil {
		t.Fatal("reversed original chart", err)
	}
	chart, e := NewTrapezoidSurfaceChart(ctx, target, AffineSurfaceChartOptions{})
	if e != nil {
		t.Fatal(e)
	}
	controls := make([][4]*big.Rat, 4)
	for i := range controls {
		controls[i] = [4]*big.Rat{new(big.Rat), big.NewRat(int64(-1+4*(i/2)), 2), new(big.Rat), big.NewRat(1, 1)}
	}
	p, e := NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	curve, e := p.IsoCurve(1, 0, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	projected, _, e := ProjectIsoCurveToTrapezoidChart(ctx, curve, chart, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	point, e := exactCurveRationalPoint(projected.patch, big.NewRat(1, 2), &polynomialBudget{maxOperations: 1000000, maxBits: 16384})
	if e != nil || point[0].Cmp(big.NewRat(1, 2)) != 0 || point[1].Cmp(big.NewRat(4, 5)) != 0 {
		t.Fatal("exact inverse", point, e)
	}
	original, e := target.EvaluateRational(ctx, point[0], point[1], SurfacePatchEvaluationOptions{})
	if e != nil || original.Point[0].Sign() != 0 || original.Point[1].Cmp(big.NewRat(1, 2)) != 0 || original.Point[2].Sign() != 0 {
		t.Fatal("original model", original, e)
	}
	clip, e := ClipIsoCurveToTrapezoidChart(ctx, curve, chart, SplinePlaneOptions{})
	if e != nil || !clip.Complete || len(clip.Spans) != 1 || len(clip.Contacts) != 0 {
		t.Fatal("complete clip", clip, e)
	}
	span := clip.Spans[0]
	if span.Start.Parameter.Cmp(big.NewRat(1, 4)) != 0 || span.End.Parameter.Cmp(big.NewRat(3, 4)) != 0 || span.Start.UV[1].Cmp(big.NewRat(1, 1)) != 0 || span.End.UV[1].Cmp(big.NewRat(2, 3)) != 0 || span.Start.BoundaryMask != 9 || span.End.BoundaryMask != 2 {
		t.Fatal("exact endpoint ownership", span)
	}
	for _, limit := range []int{1, 100} {
		bad, e := ClipIsoCurveToTrapezoidChart(ctx, curve, chart, SplinePlaneOptions{MaxOperations: limit})
		if !errors.Is(e, ErrConvergence) || bad.Complete || len(bad.Spans) != 0 {
			t.Fatal("atomic limit", bad, e)
		}
	}
	if _, e := NewTrapezoidSurfaceChart(ctx, rationalPlanePointTube(t, false, true).b, AffineSurfaceChartOptions{}); e == nil {
		t.Fatal("nonplanar mutation accepted")
	}
	// A source extension through an inverse pole must not be partially projected.
	for i := range controls {
		controls[i][1] = big.NewRat(int64(-3+4*(i/2)), 1)
	}
	polePatch, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	poleCurve, err := polePatch.IsoCurve(1, 0, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if out, _, err := ProjectIsoCurveToTrapezoidChart(ctx, poleCurve, chart, SurfacePatchOptions{}); !errors.Is(err, ErrConvergence) || out != nil {
		t.Fatal("inverse pole published", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e := ProjectIsoCurveToTrapezoidChart(canceled, curve, chart, SurfacePatchOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
