package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestWrappedRectangleEmbedding(t *testing.T) {
	ctx := context.Background()
	a, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := a.InsertKnot(ctx, 0, .125, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	relation, _, e := CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rect := func(lo, hi *big.Rat, v0, v1 int64) [2][2]*big.Rat {
		return [2][2]*big.Rat{{lo, hi}, {big.NewRat(v0, 1), big.NewRat(v1, 1)}}
	}
	opts := WrappedRectangleEmbeddingOptions{ProjectionAxes: [2]int{1, 2}}
	for _, turn := range []int64{-2, 0, 3} {
		shift := big.NewRat(4*turn, 1)
		ar := rect(new(big.Rat).Add(big.NewRat(1, 8), shift), new(big.Rat).Add(big.NewRat(3, 8), shift), -1, 1)
		br := rect(big.NewRat(1, 4), big.NewRat(1, 2), 0, 2)
		result, err := relation.CertifyEmbeddedRectangleArrangement(ctx, ar, br, opts)
		if err != nil {
			t.Fatal(turn, err)
		}
		if !result.Arrangement.Complete || !result.Injectivity.ProvenInjective || result.SourcePeriodShift[0].Int64() != turn || result.SourceHull != (ParameterRectangle{UMin: .125, UMax: .5, VMin: -1, VMax: 2}) || result.ExactOperations <= result.Arrangement.ExactOperations {
			t.Fatal(result)
		}
		if result.Arrangement.Cells[0].A[0][0].Cmp(ar[0][0]) != 0 {
			t.Fatal("lost original lifted owner")
		}
	}
	ar := rect(big.NewRat(1, 8), big.NewRat(3, 8), -1, 1)
	br := rect(big.NewRat(1, 4), big.NewRat(1, 2), 0, 2)
	bad := opts
	bad.ProjectionAxes = [2]int{0, 1}
	bad.MaxDepth = 1
	bad.MaxCells = 8
	check := func(a, b [2][2]*big.Rat, o WrappedRectangleEmbeddingOptions) {
		t.Helper()
		r, e := relation.CertifyEmbeddedRectangleArrangement(ctx, a, b, o)
		if e == nil || r.Arrangement.Complete || r.Injectivity.ProvenInjective || r.ExactOperations != 0 {
			t.Fatal("non-atomic rejection", r, e)
		}
	}
	check(ar, br, bad)
	bad = opts
	bad.Arrangement.MaxOperations = 1
	check(ar, br, bad)
	check(rect(big.NewRat(1, 9), big.NewRat(3, 8), -1, 1), br, opts)
	seamA := rect(big.NewRat(3, 1), big.NewRat(5, 1), 0, 2)
	seamB := rect(big.NewRat(0, 1), big.NewRat(2, 1), 1, 3)
	raw, e := relation.ArrangeRectanglesRational(ctx, seamA, seamB, WrappedRectangleOverlapOptions{})
	if e != nil || !raw.Complete {
		t.Fatal(raw, e)
	}
	check(seamA, seamB, opts)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	r, e := relation.CertifyEmbeddedRectangleArrangement(canceled, ar, br, opts)
	if e == nil || r.Arrangement.Complete {
		t.Fatal(r, e)
	}
}
