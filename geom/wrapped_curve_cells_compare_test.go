package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestWrappedCurveCellsCompareModel(t *testing.T) {
	ctx := context.Background()
	support := wrappedCylinderPatch(t)
	w, _, e := NewWrappedSurfacePatch(ctx, support, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	refined, e := support.InsertUKnot(.125, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	refined, e = refined.InsertVKnot(.375, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	other, _, e := NewWrappedSurfacePatch(ctx, refined, WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	controls := [][4]*big.Rat{{big.NewRat(-1, 2), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(-1, 2), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 2), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}
	uv, e := NewExactHomogeneousBezier(ctx, controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := w.BuildParameterCurveCells(ctx, uv, WrappedCurveCellsOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Independent original owner is shifted by a complete exact period.
	for i := range controls {
		controls[i][0] = new(big.Rat).Add(controls[i][0], new(big.Rat).Mul(w.Periods()[0], controls[i][3]))
	}
	shifted, e := NewExactHomogeneousBezier(ctx, controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, reverse := range []bool{false, true} {
		curve := shifted
		if reverse {
			curve, e = shifted.RationalSegment(ctx, big.NewRat(1, 1), new(big.Rat), TubeAffineLineOptions{})
			if e != nil {
				t.Fatal(e)
			}
		}
		b, e := other.BuildParameterCurveCells(ctx, curve, WrappedCurveCellsOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bounded, e := a.CompareModelBounded(ctx, b, reverse, 0, ExactCurvePlaneEventOptions{})
		if e != nil || !bounded.Proven || !bounded.Equivalent || bounded.Reversed != reverse {
			t.Fatal("bounded", bounded, e)
		}
		if q, e := a.CompareModelBounded(ctx, b, reverse, 0, ExactCurvePlaneEventOptions{MaxOperations: bounded.ExactOperations - 1}); e == nil || q.Proven {
			t.Fatal("bounded atomic", q, e)
		}
		if a.Len() == b.Len() {
			t.Fatal("fixture did not refine partitions")
		}
		result, e := a.CompareModel(ctx, b, reverse, ComposedCollectionAffineParameterOptions{})
		if e != nil || !result.Proven || !result.Equivalent || result.Reversed != reverse || result.Pieces != b.Len() {
			t.Fatal(result, e)
		}
		for _, o := range []ComposedCollectionAffineParameterOptions{{MaxCells: 1}, {MaxCuts: 2}, {EventOptions: ExactCurvePlaneEventOptions{MaxOperations: 1}}} {
			r, e := a.CompareModel(ctx, b, reverse, o)
			if e == nil || r.Proven || r.Equivalent || r.Pieces != 0 {
				t.Fatal("partial proof", r, e)
			}
		}
	}
	// Same original endpoints and seam occurrence do not prove continuous model
	// agreement. Alter only V's middle control, producing a genuine curved lift.
	controls[1][1] = big.NewRat(3, 4)
	perturbed, e := NewExactHomogeneousBezier(ctx, controls, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c, e := other.BuildParameterCurveCells(ctx, perturbed, WrappedCurveCellsOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bounded, e := a.CompareModelBounded(ctx, c, false, 0, ExactCurvePlaneEventOptions{})
	if e != nil || !bounded.Proven || bounded.Equivalent {
		t.Fatal("bounded false equality", bounded, e)
	}
	result, e := a.CompareModel(ctx, c, false, ComposedCollectionAffineParameterOptions{})
	if e != nil || !result.Proven || result.Equivalent {
		t.Fatal("endpoint-only equality", result, e)
	}
	for side := 0; side < 2; side++ {
		ai, ci := 0, 0
		if side == 1 {
			ai = a.Len() - 1
			ci = c.Len() - 1
		}
		ap, _ := a.Piece(ai)
		cp, _ := c.Piece(ci)
		tval := big.NewRat(int64(side), 1)
		x, e := ap.Model.EvaluateRational(ctx, tval, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		y, e := cp.Model.EvaluateRational(ctx, tval, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for axis := 0; axis < 3; axis++ {
			if x.Point[axis].Cmp(y.Point[axis]) != 0 {
				t.Fatal("fixture endpoints differ")
			}
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := a.CompareModel(canceled, a, false, ComposedCollectionAffineParameterOptions{}); e == nil || r.Proven {
		t.Fatal("cancel")
	}
	if r, e := (&WrappedCurveCells{}).CompareModel(ctx, a, false, ComposedCollectionAffineParameterOptions{}); e == nil || r.Proven {
		t.Fatal("empty forged proof")
	}
}
