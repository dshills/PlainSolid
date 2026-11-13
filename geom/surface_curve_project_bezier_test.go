package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestComposedProjectBezierNonbinaryCuts(t *testing.T) {
	ctx := context.Background()
	source := composedTouchFixture(t, false)
	ends := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(5, 7)}
	var roots [2]*ExactAlgebraicRoot
	for i, q := range ends {
		roots[i], _, _ = NewExactRationalRoot(ctx, q, ExactCurvePlaneEventOptions{})
	}
	cut, e := source.RestrictAlgebraic(ctx, roots[0], roots[1], ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := cut.Cell.ProjectBezier2D(ctx, [2]int{2, 0}, ends, ExactCurvePlaneEventOptions{})
	if e != nil || r.Curve == nil || r.OriginalRange[0].Cmp(ends[0]) != 0 {
		t.Fatal(r, e)
	}
	cell, e := NewExactComposedBezierCell(ctx, r.Curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []*big.Rat{new(big.Rat), big.NewRat(1, 5), big.NewRat(2, 3), big.NewRat(1, 1)} {
		p, e := cell.EvaluateRational(ctx, s, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		u := new(big.Rat).Add(ends[0], new(big.Rat).Mul(new(big.Rat).Sub(ends[1], ends[0]), s))
		z := new(big.Rat).Sub(new(big.Rat).Mul(u, u), big.NewRat(1, 2))
		z.Mul(z, z)
		if p.Point[0].Cmp(z) != 0 || p.Point[1].Cmp(u) != 0 || p.Point[2].Sign() != 0 {
			t.Fatal(p, u, z)
		}
	}
	bad := ends
	bad[0] = big.NewRat(1, 4)
	r, e = cut.Cell.ProjectBezier2D(ctx, [2]int{2, 0}, bad, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || r.Curve != nil {
		t.Fatal("wrong hint", r, e)
	}
	r, e = cut.Cell.ProjectBezier2D(ctx, [2]int{2, 0}, ends, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || r.Curve != nil {
		t.Fatal("atomic", r, e)
	}
}
func TestComposedProjectBezierRejectAlgebraicEnd(t *testing.T) {
	ctx := context.Background()
	source := composedTouchFixture(t, false)
	hits, e := source.IntersectRawPlane(ctx, [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	end, _ := source.EndpointRoot(1)
	cut, e := source.RestrictAlgebraic(ctx, hits.Events[0].Root(), end, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	interval := hits.Events[0].Interval()
	mid := new(big.Rat).Quo(new(big.Rat).Add(interval.Lower, interval.Upper), big.NewRat(2, 1))
	r, e := cut.Cell.ProjectBezier2D(ctx, [2]int{0, 2}, [2]*big.Rat{mid, big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || r.Curve != nil {
		t.Fatal(r, e)
	}
}
