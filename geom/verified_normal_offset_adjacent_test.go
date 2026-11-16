package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetAdjacencySeparatesCurvedPatchInteriors(t *testing.T) {
	ctx := context.Background()
	origA := normalSeamParaboloidPatch(t, false, false, 0)
	origB := normalSeamParaboloidPatch(t, true, true, 0)
	a, _, err := NewVerifiedNormalOffsetPatch(ctx, origA, big.NewRat(1, 10),
		NormalOffsetProjection{Axes: [2]int{0, 1}}, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := NewVerifiedNormalOffsetPatch(ctx, origB, big.NewRat(-1, 10),
		NormalOffsetProjection{Axes: [2]int{0, 1}, Reversed: [2]bool{false, true}}, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	o := SurfaceNormalOffsetSeamOptions{ReverseB: true, FlipNormalB: true}
	got, err := CertifyVerifiedNormalOffsetAdjacency(ctx, a, b, SurfaceEdgeUMax, SurfaceEdgeUMin, o)
	if err != nil || !got.Complete || !got.Seam.Complete || got.Axis != 0 ||
		got.Coordinate.Sign() != 0 || got.InteriorSigns != [2]int{-1, 1} {
		t.Fatal("offset patch adjacency should preserve the common separating plane", got, err)
	}
	// The independently known common seam of the two analytic
	// paraboloids is (0,y,y²), whose true normal has X=0 everywhere.
	for _, v := range []*big.Rat{big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(2, 3)} {
		w := new(big.Rat).Sub(big.NewRat(1, 1), v)
		x, err := a.EvaluateRational(ctx, big.NewRat(1, 1), v, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		y, err := b.EvaluateRational(ctx, new(big.Rat), w, SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for axis := range 3 {
			if x.Lower[axis].Cmp(y.Lower[axis]) != 0 || x.Upper[axis].Cmp(y.Upper[axis]) != 0 {
				t.Fatal("matched offset station has different rational enclosures", v, axis)
			}
		}
		if x.Lower[0].Sign() != 0 || x.Upper[0].Sign() != 0 {
			t.Fatal("original edge drifted from the exact separator", x)
		}
	}
	short := o
	short.MaxOperations = got.ExactOperations - 1
	if failure, err := CertifyVerifiedNormalOffsetAdjacency(ctx, a, b, SurfaceEdgeUMax, SurfaceEdgeUMin, short); !errors.Is(err, ErrConvergence) || failure.Complete || failure.Coordinate != nil {
		t.Fatal("insufficient work published offset adjacency", failure, err)
	}
	if failure, err := CertifyVerifiedNormalOffsetAdjacency(ctx, a, a, SurfaceEdgeUMax, SurfaceEdgeUMax, SurfaceNormalOffsetSeamOptions{}); err != nil || failure.Complete {
		t.Fatal("same-side geometry cannot be adjacent without overlap", failure, err)
	}
	if failure, err := CertifyVerifiedNormalOffsetAdjacency(ctx, a, b, SurfaceEdgeUMax, SurfaceEdgeUMin, SurfaceNormalOffsetSeamOptions{ReverseB: true}); err != nil || failure.Complete {
		t.Fatal("opposite signed distances require the original normal flip", failure, err)
	}
}
