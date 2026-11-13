package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func rationalInversePatch(t *testing.T) *SurfacePatch {
	t.Helper()
	var poles []Vec3
	for i, x := range []float64{0, .5, 2} {
		for _, v := range []float64{0, 1} {
			poles = append(poles, Vec3{x, float64(i) / 2, v})
		}
	}
	s, e := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestRationalSurfaceProjectionCandidateAndKnot(t *testing.T) {
	ctx := context.Background()
	p := rationalInversePatch(t)
	var e error
	p, e = p.InsertUKnot(.5, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, uv := range [][2]*big.Rat{{big.NewRat(1, 3), big.NewRat(2, 5)}, {big.NewRat(1, 2), big.NewRat(2, 5)}, {new(big.Rat), big.NewRat(1, 1)}} {
		query := [2]*big.Rat{new(big.Rat).Add(uv[0], new(big.Rat).Mul(uv[0], uv[0])), new(big.Rat).Set(uv[1])}
		r, e := InvertSurfaceProjectionRational(ctx, p, query, RationalSurfaceInverseOptions{ProjectionAxes: [2]int{0, 2}, Candidates: [][2]*big.Rat{uv}})
		if e != nil || !r.Found || !r.Complete || r.Parameters[0].Cmp(uv[0]) != 0 || r.Parameters[1].Cmp(uv[1]) != 0 {
			t.Fatal(r, e)
		}
		r.Parameters[0].SetInt64(99)
		again, e := p.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil || again.Point[0].Cmp(query[0]) != 0 {
			t.Fatal(again, e)
		}
	}
}
func TestRationalSurfaceProjectionAutomaticAndExcluded(t *testing.T) {
	ctx := context.Background()
	p := rationalInversePatch(t)
	r, e := InvertSurfaceProjectionRational(ctx, p, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}, RationalSurfaceInverseOptions{ProjectionAxes: [2]int{1, 2}})
	if e != nil || !r.Found || r.Parameters[0].Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
	r, e = InvertSurfaceProjectionRational(ctx, p, [2]*big.Rat{big.NewRat(3, 4), big.NewRat(1, 2)}, RationalSurfaceInverseOptions{ProjectionAxes: [2]int{0, 2}})
	if e != nil || !r.Found || r.Parameters[0].Cmp(big.NewRat(1, 2)) != 0 || r.CandidatesTested < 2 {
		t.Fatal(r, e)
	}
	r, e = InvertSurfaceProjectionRational(ctx, p, [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 2)}, RationalSurfaceInverseOptions{ProjectionAxes: [2]int{0, 2}})
	if e != nil || !r.Excluded || !r.Complete || r.Found {
		t.Fatal(r, e)
	}
}
func TestRationalSurfaceProjectionWrongHintRemainsPending(t *testing.T) {
	ctx := context.Background()
	p := rationalInversePatch(t)
	query := [2]*big.Rat{big.NewRat(4, 9), big.NewRat(2, 5)}
	o := RationalSurfaceInverseOptions{ProjectionAxes: [2]int{0, 2}, Candidates: [][2]*big.Rat{{big.NewRat(1, 2), big.NewRat(2, 5)}}, MaxDepth: 3}
	r, e := InvertSurfaceProjectionRational(ctx, p, query, o)
	if !errors.Is(e, ErrConvergence) || r.Found || r.Excluded || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
	o.MaxOperations = 1
	_, e = InvertSurfaceProjectionRational(ctx, p, query, o)
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = InvertSurfaceProjectionRational(canceled, p, query, RationalSurfaceInverseOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
