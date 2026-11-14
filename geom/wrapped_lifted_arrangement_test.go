package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestWrappedLiftedRectangleArrangement(t *testing.T) {
	ctx := context.Background()
	w, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	other, _, e := w.InsertKnot(ctx, 0, .125, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	relation, _, e := CompareWrappedSurfaceCharts(ctx, w, other, ExactSurfaceComparisonOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a := [2][2]*big.Rat{{big.NewRat(-1, 4), big.NewRat(1, 4)}, {big.NewRat(-1, 1), big.NewRat(1, 1)}}
	b := [2][2]*big.Rat{{big.NewRat(31, 8), big.NewRat(35, 8)}, {big.NewRat(0, 1), big.NewRat(2, 1)}}
	opts := WrappedRectangleEmbeddingOptions{ProjectionAxes: [2]int{1, 2}}
	if legacy, e := relation.CertifyEmbeddedRectangleArrangement(ctx, a, b, opts); e == nil || legacy.Arrangement.Complete {
		t.Fatal("legacy guard changed")
	}
	r, e := relation.CertifyLiftedRectangleArrangement(ctx, a, b, opts)
	if e != nil || !r.Arrangement.Complete || !r.Embedding.ProvenInjective || r.Embedding.FundamentalPieces != 2 || r.Arrangement.TargetPeriodShift[0].Cmp(big.NewInt(-1)) != 0 {
		t.Fatal(r, e)
	}
	if r.ExactOperations != r.Arrangement.ExactOperations+r.Embedding.ExactOperations {
		t.Fatal("lost cumulative work")
	}
	bad := opts
	bad.Arrangement.MaxOperations = r.ExactOperations - 1
	out, e := relation.CertifyLiftedRectangleArrangement(ctx, a, b, bad)
	if e == nil || out.Arrangement.Complete || out.Embedding.ProvenInjective || len(out.Arrangement.Cells) != 0 {
		t.Fatal("partial lifted arrangement", out, e)
	}
}
