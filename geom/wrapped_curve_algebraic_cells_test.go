package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestWrappedAlgebraicCurveCells(t *testing.T) {
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	uv, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(-1, 2), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(-1, 2), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := w.ComposeParameterCurvePieces(ctx, uv, SurfaceCurveCompositionOptions{}); e == nil || r.Complete {
		t.Fatal("legacy rational gate widened")
	}
	for _, reverse := range []bool{false, true} {
		curve := uv
		if reverse {
			curve, e = uv.RationalSegment(ctx, big.NewRat(1, 1), new(big.Rat), TubeAffineLineOptions{})
			if e != nil {
				t.Fatal(e)
			}
		}
		result, e := w.BuildParameterCurveCells(ctx, curve, WrappedCurveCellsOptions{})
		if e != nil || result.Len() != 2 {
			t.Fatal(result, e)
		}
		first, _ := result.Piece(0)
		second, _ := result.Piece(1)
		root, e := first.Model.EndpointRoot(1)
		if e != nil {
			t.Fatal(e)
		}
		kind, e := root.Classify(ctx, ExactCurvePlaneEventOptions{})
		if e != nil || kind.Kind != ExactRootKindIrrational {
			t.Fatal(kind, e)
		}
		polynomial := []string{"-1", "0", "2"}
		if reverse {
			polynomial = []string{"1", "-4", "2"}
		}
		expected, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: polynomial, Lower: "0", Upper: "1"}, ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		order, e := root.Compare(ctx, expected, ExactCurvePlaneEventOptions{})
		if e != nil || !order.Proven || order.Order != 0 {
			t.Fatal(order, e)
		}
		other, _ := second.Model.EndpointRoot(0)
		order, e = root.Compare(ctx, other, ExactCurvePlaneEventOptions{})
		if e != nil || order.Order != 0 {
			t.Fatal("seam identity", order, e)
		}
		recipe, e := root.Export(ctx, ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		restored, e := ImportExactRoot(ctx, recipe, ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		order, e = root.Compare(ctx, restored, ExactCurvePlaneEventOptions{})
		if e != nil || order.Order != 0 {
			t.Fatal("recipe", e)
		}
		for i, s := range []*big.Rat{big.NewRat(1, 8), big.NewRat(7, 8)} {
			piece, _ := result.Piece(i)
			original, e := curve.patch.EvaluateRational(ctx, s, new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			lifted, e := piece.UV.EvaluateRational(ctx, s, ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want, e := w.EvaluateRational(ctx, original.Point[0], original.Point[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			got, e := piece.Model.EvaluateRational(ctx, s, ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if lifted.Point[axis].Cmp(original.Point[axis]) != 0 || got.Point[axis].Cmp(want.Point[axis]) != 0 {
					t.Fatal("original evaluation", i, axis)
				}
			}
			if _, e := piece.Model.EvaluateRational(ctx, new(big.Rat).Sub(big.NewRat(1, 1), s), ExactCurvePlaneEventOptions{}); e == nil {
				t.Fatal("wrong side accepted")
			}
		}
		shift0, shift1 := int64(-1), int64(0)
		if reverse {
			shift0, shift1 = shift1, shift0
		}
		if first.PeriodShift[0].Cmp(big.NewInt(shift0)) != 0 || second.PeriodShift[0].Cmp(big.NewInt(shift1)) != 0 {
			t.Fatal("authored lift lost")
		}
		first.PeriodShift[0].SetInt64(77)
		again, _ := result.Piece(0)
		if again.PeriodShift[0].Cmp(big.NewInt(shift0)) != 0 {
			t.Fatal("mutable result")
		}
		for _, o := range []WrappedCurveCellsOptions{{MaxEvents: 2}, {MaxStations: 1}, {SurfacePatchOptions: SurfacePatchOptions{MaxOperations: result.ExactOperations() - 1}}, {MaxSubdivisions: 1}} {
			if r, e := w.BuildParameterCurveCells(ctx, curve, o); e == nil || r != nil {
				t.Fatal("non atomic limit", o, r, e)
			}
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := w.BuildParameterCurveCells(canceled, uv, WrappedCurveCellsOptions{}); e == nil || r != nil {
		t.Fatal("cancel")
	}
	// The unwrapped V coordinate actually exits the source; a UV hull alone
	// cannot authorize extrapolated composition.
	bad, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(-1, 2), big.NewRat(4, 1), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(-1, 2), big.NewRat(4, 1), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), big.NewRat(4, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r, e := w.BuildParameterCurveCells(ctx, bad, WrappedCurveCellsOptions{}); e == nil || r != nil {
		t.Fatal("extrapolation", r, e)
	}
}

func TestWrappedAlgebraicCurveCellsRefinedKnots(t *testing.T) {
	ctx := context.Background()
	support, e := wrappedCylinderPatch(t).InsertUKnot(.125, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := NewWrappedSurfacePatch(ctx, support, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	uv, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(-1, 2), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(-1, 2), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	result, e := w.BuildParameterCurveCells(ctx, uv, WrappedCurveCellsOptions{})
	if e != nil || result.Len() != 3 {
		t.Fatal(result, e)
	}
	for i := 0; i < result.Len(); i++ {
		piece, _ := result.Piece(i)
		ends := piece.UV.EndpointIntervals()
		s := new(big.Rat).Quo(new(big.Rat).Add(ends[0].Upper, ends[1].Lower), big.NewRat(2, 1))
		chart, e := piece.UV.EvaluateRational(ctx, s, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		model, e := piece.Model.EvaluateRational(ctx, s, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		original, e := w.EvaluateRational(ctx, chart.Point[0], chart.Point[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for axis := 0; axis < 3; axis++ {
			if model.Point[axis].Cmp(original.Point[axis]) != 0 {
				t.Fatal("refined source mismatch")
			}
		}
		if i > 0 {
			root, _ := piece.Model.EndpointRoot(0)
			kind, e := root.Classify(ctx, ExactCurvePlaneEventOptions{})
			if e != nil || kind.Kind != ExactRootKindIrrational {
				t.Fatal("rounded seam/knot", kind, e)
			}
		}
	}
}
