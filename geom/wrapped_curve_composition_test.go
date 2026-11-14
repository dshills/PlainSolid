package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestWrappedQuadraticComposition(t *testing.T) {
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	curve, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(-1, 2), new(big.Rat), new(big.Rat), big.NewRat(2, 1)}, {big.NewRat(-1, 8), big.NewRat(1, 2), new(big.Rat), big.NewRat(3, 2)}, {big.NewRat(1, 4), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, reverse := range []bool{false, true} {
		uv := curve
		cut := big.NewRat(2, 3)
		if reverse {
			uv, e = curve.RationalSegment(ctx, big.NewRat(1, 1), new(big.Rat), TubeAffineLineOptions{})
			if e != nil {
				t.Fatal(e)
			}
			cut = big.NewRat(1, 3)
		}
		r, e := w.ComposeParameterCurvePieces(ctx, uv, SurfaceCurveCompositionOptions{})
		if e != nil || !r.Complete || !r.Continuous || len(r.Pieces) != 2 {
			t.Fatal(r, e)
		}
		if r.Pieces[0].End.Cmp(cut) != 0 || r.Pieces[1].Start.Cmp(cut) != 0 {
			t.Fatal("rounded seam parameter")
		}
		if r.Pieces[0].PeriodShift[0].Cmp(r.Pieces[1].PeriodShift[0]) == 0 {
			t.Fatal("missing distinct original lifts")
		}
		for _, piece := range r.Pieces {
			for _, s := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1)} {
				global := new(big.Rat).Add(piece.Start, new(big.Rat).Mul(new(big.Rat).Sub(piece.End, piece.Start), s))
				p, e := uv.patch.EvaluateRational(ctx, global, new(big.Rat), SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				original, e := piece.UV.patch.EvaluateRational(ctx, s, new(big.Rat), SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for axis := 0; axis < 2; axis++ {
					if p.Point[axis].Cmp(original.Point[axis]) != 0 {
						t.Fatal("lost original UV")
					}
				}
				want, e := w.EvaluateRational(ctx, p.Point[0], p.Point[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				got, e := piece.Curve.patch.EvaluateRational(ctx, s, new(big.Rat), SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for axis := 0; axis < 3; axis++ {
					if got.Point[axis].Cmp(want.Point[axis]) != 0 {
						t.Fatal("model composition mismatch")
					}
				}
			}
		}
		for _, o := range []SurfaceCurveCompositionOptions{{MaxPieces: 1}, {SurfacePatchOptions: SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}}} {
			q, e := w.ComposeParameterCurvePieces(ctx, uv, o)
			if e == nil || q.Complete || len(q.Pieces) != 0 {
				t.Fatal("partial output", e)
			}
		}
	}
	lift, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(33, 8), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(17, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	lifted, e := w.ComposeParameterCurvePieces(ctx, lift, SurfaceCurveCompositionOptions{})
	if e != nil || len(lifted.Pieces) != 1 {
		t.Fatal("full-range lift", e)
	}
	point, e := lifted.Pieces[0].UV.patch.EvaluateRational(ctx, new(big.Rat), new(big.Rat), SurfacePatchEvaluationOptions{})
	if e != nil || point.Point[0].Cmp(big.NewRat(33, 8)) != 0 {
		t.Fatal("full-range restriction lost original lift", e)
	}
	irrational, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(-1, 8), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(-1, 8), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 8), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := w.ComposeParameterCurvePieces(ctx, irrational, SurfaceCurveCompositionOptions{}); e == nil || r.Complete || len(r.Pieces) != 0 {
		t.Fatal("irrational seam rounded")
	}
	cancel, stop := context.WithCancel(ctx)
	stop()
	if r, e := w.ComposeParameterCurvePieces(cancel, curve, SurfaceCurveCompositionOptions{}); e == nil || r.Complete || len(r.Pieces) != 0 {
		t.Fatal("cancel publication")
	}
}
