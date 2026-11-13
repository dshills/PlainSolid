package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func cornerLedgerFixture(t *testing.T) *SplineCornerFan {
	t.Helper()
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, .125)
	b, _ := makeCornerShearBox(t, [3]float64{-1, -1, -2}, [3]float64{2, 2, 0}, 0)
	var anchors [2]*SplineCornerAnchor
	for i, face := range []int{0, 2} {
		box := geom.ParameterRectangle{UMin: -.125, UMax: .125, VMin: 0, VMax: .25}
		if i == 1 {
			box = geom.ParameterRectangle{UMin: 0, UMax: .25, VMin: -.125, VMax: .125}
		}
		r, e := CertifySplineFaceCornerAnchor(ctx, a, b, [2]int{face, 5}, [2]geom.ParameterRectangle{{UMax: .125, VMax: .125}, box}, 0, geom.SurfaceTubeBoundaryOptions{SurfacePairTubeOptions: geom.SurfacePairTubeOptions{FreeAxis: i}, Anchor: [4]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)}}, SplineRectangleEmbeddingOptions{})
		if e != nil {
			t.Fatal(e)
		}
		anchors[i] = r
	}
	p, e := CertifySplineFacePointContact(ctx, a, b, [2]int{4, 5}, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	f, e := CertifySplineCornerFan(ctx, anchors, p, SplineRectangleEmbeddingOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return f
}
func TestSplineCornerSectionLedger(t *testing.T) {
	ctx := context.Background()
	fan := cornerLedgerFixture(t)
	ledger, e := DiagnoseSplineCornerSections(ctx, fan, SplineCornerSectionLedgerOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !ledger.PendingAssembly() || ledger.AllPairsClassified() {
		t.Fatal("unsafe acceptance")
	}
	pairs := ledger.Pairs()
	if len(pairs) != 36 {
		t.Fatal("missing original face pairs", len(pairs))
	}
	counts := [4]int{}
	for i, p := range pairs {
		if p.Faces != ([2]int{i / 6, i % 6}) {
			t.Fatal("ordering")
		}
		counts[p.Kind]++
		if p.Kind == SplinePairUnknown && p.Reason == "" {
			t.Fatal("missing unknown evidence")
		}
		if p.Kind == SplinePairPointOnlyPendingFan && p.Faces != ([2]int{4, 5}) {
			t.Fatal("unbound point event")
		}
	}
	if counts != ([4]int{2, 31, 2, 1}) {
		t.Fatal(counts)
	}
	t.Log("unknown,separated,branches,point", counts)
	pairs[0].Kind = SplinePairBranches
	if ledger.Pairs()[0].Kind == SplinePairBranches {
		t.Fatal("snapshot alias")
	}
	if bad, e := DiagnoseSplineCornerSections(ctx, fan, SplineCornerSectionLedgerOptions{MaxPairs: 35}); e == nil || bad != nil {
		t.Fatal("matrix limit")
	}
	exhausted, e := DiagnoseSplineCornerSections(ctx, fan, SplineCornerSectionLedgerOptions{Faces: SplineRectangleEmbeddingOptions{Patches: geom.SurfacePatchOptions{MaxOperations: 1}}})
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range exhausted.Pairs() {
		if p.Kind != SplinePairUnknown {
			t.Fatal("budget silently dropped pair")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := DiagnoseSplineCornerSections(canceled, fan, SplineCornerSectionLedgerOptions{}); e != context.Canceled || bad != nil {
		t.Fatal("cancel", e)
	}
}
