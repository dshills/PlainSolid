package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestWrappedSurfaceExactReparameterizationAndSeams(t *testing.T) {
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true, ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	target := ParameterRectangle{UMin: -3, UMax: 5, VMin: 10, VMax: 20}
	for _, reverse := range []bool{false, true} {
		mapped, seam, e := w.Reparameterize(ctx, target, reverse, true, SurfacePatchOptions{})
		if e != nil || !seam.FirstPartialsMatched[0] {
			t.Fatal(seam, e)
		}
		// New U=-3+8/3 maps to old U=4/3 (or8/3 when reflected).
		u := big.NewRat(-1, 3)
		oldU := big.NewRat(4, 3)
		factor := big.NewRat(1, 2)
		if reverse {
			oldU = big.NewRat(8, 3)
			factor.Neg(factor)
		}
		got, e := mapped.EvaluateRational(ctx, u, big.NewRat(40, 3), SurfacePatchEvaluationOptions{})
		want, f := w.EvaluateRational(ctx, oldU, big.NewRat(4, 3), SurfacePatchEvaluationOptions{})
		if e != nil || f != nil {
			t.Fatal(e, f)
		}
		for axis := 0; axis < 3; axis++ {
			if got.Point[axis].Cmp(want.Point[axis]) != 0 || got.Du[axis].Cmp(new(big.Rat).Mul(want.Du[axis], factor)) != 0 || got.Dv[axis].Cmp(new(big.Rat).Mul(want.Dv[axis], big.NewRat(-1, 2))) != 0 {
				t.Fatal(got, want)
			}
		}
		if mapped.Periods()[0].Cmp(big.NewRat(8, 1)) != 0 {
			t.Fatal(mapped.Periods())
		}
		first, e := mapped.SeamIso(0, false, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		last, e := mapped.SeamIso(0, true, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		identity, e := CompareExactCurves(first, last, ExactCurveComparisonOptions{})
		if e != nil || !identity.Equal {
			t.Fatal(identity, e)
		}
		_, e = mapped.SeamIso(1, false, SurfacePatchOptions{})
		if !errors.Is(e, ErrDomain) {
			t.Fatal(e)
		}
	}
	_, _, e = w.Reparameterize(ctx, ParameterRectangle{UMin: 0, UMax: 1.0 / 3, VMin: -2, VMax: 3}, false, false, SurfacePatchOptions{})
	if !errors.Is(e, ErrUnrepresentable) {
		t.Fatal("rounded mapped knot accepted", e)
	}
	_, _, e = w.Reparameterize(ctx, target, false, false, SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, e = w.Reparameterize(canceled, target, false, false, SurfacePatchOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
