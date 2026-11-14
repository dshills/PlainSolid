package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestWrappedRectangleLiftedOverlap(t *testing.T) {
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
	rect := func(u0, u1, v0, v1 int64) [2][2]*big.Rat {
		return [2][2]*big.Rat{{big.NewRat(u0, 1), big.NewRat(u1, 1)}, {big.NewRat(v0, 1), big.NewRat(v1, 1)}}
	}
	r, e := relation.OverlapRectanglesRational(ctx, rect(3, 5, 0, 2), rect(0, 1, 1, 3), WrappedRectangleOverlapOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 {
		t.Fatal(r, e)
	}
	c := r.Components[0]
	if c.Dimension != 2 || c.TargetPeriodShift[0].Cmp(big.NewInt(1)) != 0 || c.A[0][0].Cmp(big.NewRat(4, 1)) != 0 || c.A[0][1].Cmp(big.NewRat(5, 1)) != 0 || c.B[0][0].Sign() != 0 || c.B[0][1].Cmp(big.NewRat(1, 1)) != 0 || c.A[1][0].Cmp(big.NewRat(1, 1)) != 0 || c.A[1][1].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal(c)
	}
	r, e = relation.OverlapRectanglesRational(ctx, rect(-5, -3, 0, 2), rect(0, 1, 1, 3), WrappedRectangleOverlapOptions{})
	if e != nil || len(r.Components) != 1 || r.Components[0].TargetPeriodShift[0].Cmp(big.NewInt(-1)) != 0 || r.Components[0].A[0][0].Cmp(big.NewRat(-4, 1)) != 0 {
		t.Fatal(r, e)
	}
	r, e = relation.OverlapRectanglesRational(ctx, rect(0, 8, 0, 2), rect(0, 4, 0, 2), WrappedRectangleOverlapOptions{})
	if e != nil || len(r.Components) != 4 {
		t.Fatal(r, e)
	}
	for i, c := range r.Components {
		want := 2
		if i == 0 || i == 3 {
			want = 1
		}
		if c.Dimension != want || c.TargetPeriodShift[0].Cmp(big.NewInt(int64(i-1))) != 0 {
			t.Fatal(i, c)
		}
	}
	huge := new(big.Int).Lsh(big.NewInt(1), 1000)
	bigRect := rect(0, 1, 0, 1)
	bigRect[0][0] = new(big.Rat).SetInt(huge)
	bigRect[0][1] = new(big.Rat).Add(bigRect[0][0], big.NewRat(1, 1))
	large, err := relation.OverlapRectanglesRational(ctx, bigRect, rect(0, 1, 0, 1), WrappedRectangleOverlapOptions{MaxComponents: 1})
	wantShift := new(big.Int).Rsh(new(big.Int).Set(huge), 2)
	if err != nil || len(large.Components) != 1 || large.Components[0].TargetPeriodShift[0].Cmp(wantShift) != 0 || large.Components[0].A[0][0].Cmp(bigRect[0][0]) != 0 {
		t.Fatal("large winding", large, err)
	}
	// These are distinct lifted occurrences, not four physical branches.
	r, e = relation.OverlapRectanglesRational(ctx, rect(0, 8, 0, 2), rect(0, 4, 0, 2), WrappedRectangleOverlapOptions{MaxComponents: 3})
	if !errors.Is(e, ErrConvergence) || r.Complete || len(r.Components) != 0 {
		t.Fatal("atomic count", r, e)
	}
	r, e = relation.OverlapRectanglesRational(ctx, rect(0, 1, 0, 1), rect(1, 2, 1, 2), WrappedRectangleOverlapOptions{})
	if e != nil || len(r.Components) != 1 || r.Components[0].Dimension != 0 {
		t.Fatal("corner", r, e)
	}
	r, e = relation.OverlapRectanglesRational(ctx, rect(0, 1, 0, 1), rect(2, 3, 1, 2), WrappedRectangleOverlapOptions{})
	if e != nil || !r.Complete || len(r.Components) != 0 {
		t.Fatal("disjoint", r, e)
	}
	r, e = relation.OverlapRectanglesRational(ctx, rect(0, 1, 0, 4), rect(0, 1, 0, 2), WrappedRectangleOverlapOptions{})
	if !errors.Is(e, ErrDomain) || r.Complete {
		t.Fatal("unwrapped exit", r, e)
	}
	r, e = relation.OverlapRectanglesRational(ctx, rect(0, 1, 0, 1), rect(0, 1, 0, 1), WrappedRectangleOverlapOptions{MaxOperations: 1})
	if e == nil || r.Complete || len(r.Components) != 0 {
		t.Fatal("work", r, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = relation.OverlapRectanglesRational(cancelled, rect(0, 1, 0, 1), rect(0, 1, 0, 1), WrappedRectangleOverlapOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}

func TestWrappedRectangleReversedOriginalOwners(t *testing.T) {
	ctx := context.Background()
	a, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := a.Reparameterize(ctx, ParameterRectangle{UMin: 2, UMax: 10, VMin: -3, VMax: 7}, true, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	relation, _, e := CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true, ReverseBU: true})
	if e != nil {
		t.Fatal(e)
	}
	q := func(n int64) *big.Rat { return big.NewRat(n, 1) }
	r, e := relation.OverlapRectanglesRational(ctx, [2][2]*big.Rat{{q(3), q(5)}, {q(0), q(2)}}, [2][2]*big.Rat{{q(2), q(4)}, {q(1), q(5)}}, WrappedRectangleOverlapOptions{})
	if e != nil || len(r.Components) != 1 {
		t.Fatal(r, e)
	}
	c := r.Components[0]
	if c.A[0][0].Cmp(q(3)) != 0 || c.A[0][1].Cmp(q(4)) != 0 || c.B[0][0].Cmp(q(2)) != 0 || c.B[0][1].Cmp(q(4)) != 0 {
		t.Fatal("original ranges", c)
	}
	for _, u := range c.A[0] {
		for _, v := range c.A[1] {
			mapped, e := relation.MapRational(ctx, [2]*big.Rat{u, v}, SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			x, e := a.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			y, e := b.EvaluateRational(ctx, mapped[0], mapped[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for j := 0; j < 3; j++ {
				if x.Point[j].Cmp(y.Point[j]) != 0 {
					t.Fatal("corner model mismatch")
				}
			}
		}
	}
}
