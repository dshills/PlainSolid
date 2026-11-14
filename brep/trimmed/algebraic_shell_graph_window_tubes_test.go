package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestGraphWindowRefinedTubes(t *testing.T) {
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(context.Background(), AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	w := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(3, 32))
	options := AlgebraicGraphWindowRefineOptions{MaxCells: 85, MaxDepth: 3}
	baseline, e := q.RefineWindowDepth(context.Background(), w, options)
	if e != nil {
		t.Fatal(e)
	}
	refined, report, e := q.WithRefinedWindowTubes(context.Background(), AlgebraicGraphTubeWindowOptions{})
	if e != nil {
		t.Fatal(e, report)
	}
	segments := 0
	for _, loop := range q.loops {
		segments += len(loop)
	}
	if report.Segments != segments || report.Intervals != 2*segments {
		t.Fatal("incomplete segment partition", report)
	}
	if !report.Complete || report.ContractedIntervals == 0 || refined == q || q.windowTubeBoxes != nil {
		t.Fatal("prepared refinement", report)
	}
	result, e := refined.RefineWindowDepth(context.Background(), w, options)
	if e != nil {
		t.Fatal(e)
	}
	if !result.HasBounds || result.UnresolvedArea.Cmp(baseline.UnresolvedArea) >= 0 || result.Volume[1].Cmp(baseline.Volume[1]) > 0 {
		t.Fatalf("no tightening: area %v -> %v; V%v -> %v", baseline.UnresolvedArea, result.UnresolvedArea, baseline.Volume, result.Volume)
	}
	exact := math.Pi * math.Pow(5.0/64, 4) / 2
	lo, _ := result.Volume[0].Float64()
	hi, _ := result.Volume[1].Float64()
	if lo > exact || hi < exact {
		t.Fatal("analytic volume", lo, hi, exact)
	}
	// On exactly the same spatial grid, every old proof still succeeds and
	// newly separated cells only tighten the lower and upper integral bounds.
	var oldLower, newLower big.Rat
	oldFull, newFull := 0, 0
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			cell := AlgebraicGraphProjectionWindow{}
			for axis, k := range []int{i, j} {
				span := new(big.Rat).Sub(w.Max[axis], w.Min[axis])
				cell.Min[axis] = new(big.Rat).Add(w.Min[axis], new(big.Rat).Mul(span, big.NewRat(int64(k), 8)))
				cell.Max[axis] = new(big.Rat).Add(w.Min[axis], new(big.Rat).Mul(span, big.NewRat(int64(k+1), 8)))
			}
			a, ea := q.BoundWindowDepth(context.Background(), cell, AlgebraicGraphWindowOptions{})
			b, eb := refined.BoundWindowDepth(context.Background(), cell, AlgebraicGraphWindowOptions{})
			if ea == nil {
				if eb != nil || a.FullMaterial != b.FullMaterial || a.NoFootprint != b.NoFootprint || a.Faces != b.Faces {
					t.Fatal("old cell certificate lost", ea, eb)
				}
			}
			if ea == nil && a.FullMaterial {
				oldFull++
				oldLower.Add(&oldLower, a.Depth[0])
			}
			if eb == nil && b.FullMaterial {
				newFull++
				newLower.Add(&newLower, b.Depth[0])
			}
		}
	}
	if newFull < oldFull || newLower.Cmp(&oldLower) < 0 {
		t.Fatal("fixed grid degraded")
	}
	// True rim remains unresolved: neither contraction nor partition can remove it.
	rim := graphWindow(big.NewRat(27, 16), big.NewRat(9, 2), big.NewRat(5, 64))
	bad, e := refined.BoundWindowDepth(context.Background(), rim, AlgebraicGraphWindowOptions{})
	if !errors.Is(e, geom.ErrConvergence) || bad.Complete {
		t.Fatal("rim incorrectly excluded", e)
	}
	for _, o := range []AlgebraicGraphTubeWindowOptions{{MaxIntervals: 1}, {Exact: geom.ExactCurvePlaneEventOptions{MaxOperations: 1}}} {
		failed, r, e := q.WithRefinedWindowTubes(context.Background(), o)
		if !errors.Is(e, geom.ErrConvergence) || failed != nil || r.Complete {
			t.Fatal("resource atomicity", r, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	failed, r, e := q.WithRefinedWindowTubes(ctx, AlgebraicGraphTubeWindowOptions{})
	if !errors.Is(e, context.Canceled) || failed != nil || r.Complete {
		t.Fatal("cancel", e)
	}
	t.Logf("%+v; unresolved %s -> %s; volume [%g,%g]", report, baseline.UnresolvedArea, result.UnresolvedArea, lo, hi)
}
