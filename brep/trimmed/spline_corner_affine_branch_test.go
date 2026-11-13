package trimmed

import (
	"context"
	"plainsolid/geom"
	"testing"
)

func TestSplineCornerAffineBranches(t *testing.T) {
	ctx := context.Background()
	fan := cornerLedgerFixture(t)
	for _, anchor := range fan.anchors {
		branch, e := CertifySplineCornerAffineBranch(ctx, anchor, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		ends := branch.Endpoints()
		near, far := ends[branch.CornerEnd()], ends[branch.CornerEnd()^1]
		for _, q := range near.Point {
			if q.Sign() != 0 {
				t.Fatal("corner")
			}
		}
		want := [3]float64{0, 1, .25}
		if anchor.pair[0] == 2 {
			want = [3]float64{1, 0, .25}
		}
		for i, q := range far.Point {
			v, exact := q.Float64()
			if !exact || v != want[i] {
				t.Fatal("full segment", far.Point)
			}
		}
		if near.BoundaryB != 0 || far.BoundaryB != 0 || !branch.PendingAssembly() {
			t.Fatal("ownership")
		}
		ends[0].Point[0].SetInt64(999)
		if branch.Endpoints()[0].Point[0].RatString() == "999" {
			t.Fatal("alias")
		}
		if r, e := CertifySplineCornerAffineBranch(ctx, anchor, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}); e == nil || r != nil {
			t.Fatal("budget")
		}
	}
	ledger, e := DiagnoseSplineCornerSections(ctx, fan, SplineCornerSectionLedgerOptions{})
	if e != nil {
		t.Fatal(e)
	}
	full, e := ResolveSplineCornerAffineSections(ctx, ledger, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !full.AllPairsClassified() || !full.PendingAssembly() || ledger.AllPairsClassified() || len(full.Pairs()) != 36 || len(full.AffineCornerBranches()) != 2 {
		t.Fatal("ledger")
	}
	counts := [5]int{}
	for _, p := range full.Pairs() {
		counts[p.Kind]++
	}
	if counts != ([5]int{0, 31, 2, 1, 2}) {
		t.Fatal(counts)
	}
	repeated, e := ResolveSplineCornerAffineSections(ctx, full, SplineRectangleEmbeddingOptions{})
	if e != nil || len(repeated.AffineCornerBranches()) != 2 {
		t.Fatal("duplicate", e)
	}
	exhausted, e := ResolveSplineCornerAffineSections(ctx, ledger, SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}})
	if e != nil || exhausted.AllPairsClassified() || len(exhausted.AffineCornerBranches()) != 0 {
		t.Fatal("unknown discarded", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := ResolveSplineCornerAffineSections(canceled, ledger, SplineRectangleEmbeddingOptions{}); e != context.Canceled || r != nil {
		t.Fatal("cancel", e)
	}
}
