package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestComposedCollectionSameImageDifferentPartitions(t *testing.T) {
	ctx := context.Background()
	cell := func(reverse bool) *ExactComposedCurveCell {
		points := [][2]*big.Rat{{big.NewRat(0, 1), big.NewRat(0, 1)}, {big.NewRat(1, 2), big.NewRat(0, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1)}}
		if reverse {
			points[0], points[2] = points[2], points[0]
		}
		net := make([][4]*big.Rat, 3)
		for i, p := range points {
			net[i] = [4]*big.Rat{p[0], p[1], new(big.Rat), big.NewRat(1, 1)}
		}
		curve, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		c, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	a, b := cell(false), cell(true)
	zero, _ := a.EndpointRoot(0)
	one, _ := a.EndpointRoot(1)
	third := importTestRoot(t, big.NewRat(-1, 3), false, "1/3", "1/3")
	quarter := importTestRoot(t, big.NewRat(-1, 4), false, "1/4", "1/4")
	irrational := importTestRoot(t, big.NewRat(-1, 2), true, "2/3", "3/4")
	partition := func(c *ExactComposedCurveCell, cuts ...*ExactAlgebraicRoot) SurfaceCurveCompositionCells {
		result := SurfaceCurveCompositionCells{}
		for i := 0; i+1 < len(cuts); i++ {
			piece, e := c.RestrictAlgebraic(ctx, cuts[i], cuts[i+1], ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			result.Cells = append(result.Cells, piece.Cell)
		}
		return result
	}
	left := partition(a, zero, third, one)
	right := partition(b, zero, quarter, irrational, one)
	r, e := CompareComposedCollectionsAffineProjection(ctx, left, right, 0, ComposedCollectionComparisonOptions{})
	if e != nil || !r.Complete || !r.SameImage || !r.AContained || !r.BContained || r.PartialCoincidence || r.ProjectionOrientation != [2]int{1, -1} {
		t.Fatal(r, e)
	}
	threeQuarter := importTestRoot(t, big.NewRat(-3, 4), false, "3/4", "3/4")
	restricted, e := right.RestrictAlgebraic(ctx, quarter, threeQuarter, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	partial, e := CompareComposedCollectionsAffineProjection(ctx, left, restricted, 0, ComposedCollectionComparisonOptions{})
	if e != nil || !partial.Complete || partial.SameImage || partial.AContained || !partial.BContained || !partial.PartialCoincidence {
		t.Fatal(partial, e)
	}
	// A mutable complete flag cannot conceal a parameter gap.
	bad := left
	bad.Complete = true
	bad.Cells = append([]*ExactComposedCurveCell(nil), left.Cells...)
	gap, e := a.RestrictAlgebraic(ctx, threeQuarter, one, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bad.Cells[1] = gap.Cell
	if forged, e := CompareComposedCollectionsAffineProjection(ctx, bad, right, 0, ComposedCollectionComparisonOptions{}); e == nil || forged.SameImage || forged.Complete {
		t.Fatal(forged, e)
	}
	if _, e = CompareComposedCollectionsAffineProjection(ctx, left, right, 0, ComposedCollectionComparisonOptions{MaxPairs: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = CompareComposedCollectionsAffineProjection(cancelCtx, left, right, 0, ComposedCollectionComparisonOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
