package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestCompareExactCurvesFractionalOriginalClockAndKnot(t *testing.T) {
	ctx := context.Background()
	chart := projectiveOverlapTestChart(t, false, false)
	b, err := chart.IsoCurve(0, -2, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, reverse := range []bool{false, true} {
		ends := [2]*big.Rat{big.NewRat(5, 4), big.NewRat(-1, 4)}
		ratio := big.NewRat(3, 2)
		if reverse {
			ends[0], ends[1] = ends[1], ends[0]
			ratio.Inv(ratio)
		}
		var poles [][4]*big.Rat
		for _, x := range ends {
			poles = append(poles, [4]*big.Rat{x, big.NewRat(1, 2), new(big.Rat).Quo(x, big.NewRat(2, 1)), big.NewRat(1, 1)})
		}
		a, err := NewExactHomogeneousBezier(ctx, poles, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		o := ExactCurveComparisonOptions{NormalizeDomains: true, ReverseB: reverse, BStationRatio: ratio}
		r, err := CompareExactCurvesContext(ctx, a, b, o)
		if err != nil || !r.Complete || !r.Equal || r.ComparedSpans != 2 {
			t.Fatal("true raw knot cover must compare through its exact fractional clock", reverse, r, err)
		}
		pa, err := a.RationalSegmentPieces(ctx, new(big.Rat), big.NewRat(1, 1), SurfaceAffineCurveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		pb, err := b.RationalSegmentPieces(ctx, big.NewRat(4, 1), big.NewRat(8, 1), SurfaceAffineCurveOptions{})
		if err != nil || len(pb.Pieces) != 2 || pb.Pieces[0].End.Cmp(big.NewRat(3, 4)) != 0 {
			t.Fatal("missing original B knot in independent piece extraction", pb, err)
		}
		r, err = CompareAffineParameterCurvePieces(ctx, pa, pb, o)
		if err != nil || !r.Complete || !r.Equal || r.ComparedSpans != 2 {
			t.Fatal("owner piece collections disagree under their known clock", reverse, r, err)
		}
		wrong := o
		wrong.BStationRatio = big.NewRat(1, 1)
		r, err = CompareAffineParameterCurvePieces(ctx, pa, pb, wrong)
		if err != nil || !r.Complete || r.Equal || r.DifferentAxis != 0 {
			t.Fatal("wrong clock preserves endpoints but not the original curve", reverse, r, err)
		}
		o.MaxSpans = 1
		if r, err = CompareExactCurvesContext(ctx, a, b, o); !errors.Is(err, ErrConvergence) || r.Complete {
			t.Fatal("missing pulled original span must not certify equality", reverse, r, err)
		}
	}
}
