package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestComposedAffineProjectionObliqueOracle(t *testing.T) {
	ctx := context.Background()
	curve, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{new(big.Rat), new(big.Rat), big.NewRat(-3, 1), big.NewRat(1, 1)}, {big.NewRat(1, 2), new(big.Rat), big.NewRat(-5, 2), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ends := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(4, 5)}
	var roots [2]*ExactAlgebraicRoot
	for i, q := range ends {
		roots[i], _, _ = NewExactRationalRoot(ctx, q, ExactCurvePlaneEventOptions{})
	}
	cut, e := cell.RestrictAlgebraic(ctx, roots[0], roots[1], ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	forms := [2][4]*big.Rat{{big.NewRat(2, 1), big.NewRat(-1, 1), nil, big.NewRat(1, 3)}, {big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(-1, 1), big.NewRat(2, 7)}}
	plane := [4]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(-1, 1), big.NewRat(-3, 1)}
	r, e := cut.Cell.ProjectAffineBezierPieces2D(ctx, forms, plane, ends, ComposedProjectionOptions{})
	if e != nil || !r.Complete || len(r.Pieces) != 1 {
		t.Fatal(r, e)
	}
	projected, e := NewExactComposedBezierCell(ctx, r.Pieces[0].Curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []*big.Rat{new(big.Rat), big.NewRat(2, 5), big.NewRat(1, 1)} {
		p, e := projected.EvaluateRational(ctx, s, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		u := new(big.Rat).Add(ends[0], new(big.Rat).Mul(s, new(big.Rat).Sub(ends[1], ends[0])))
		u2 := new(big.Rat).Mul(u, u)
		x := new(big.Rat).Add(new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), u), u2), big.NewRat(1, 3))
		y := new(big.Rat).Sub(big.NewRat(23, 7), u2)
		if p.Point[0].Cmp(x) != 0 || p.Point[1].Cmp(y) != 0 || p.Point[2].Sign() != 0 {
			t.Fatal(p, x, y)
		}
	}
	// Published coefficients must not alias caller inputs.
	r.Forms[0][0].SetInt64(99)
	if forms[0][0].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal("alias")
	}
	wrong := plane
	wrong[3] = new(big.Rat).Add(plane[3], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	r, e = cut.Cell.ProjectAffineBezierPieces2D(ctx, forms, wrong, ends, ComposedProjectionOptions{})
	if e != ErrDomain || r.Complete || len(r.Pieces) != 0 {
		t.Fatal("nonincident", r, e)
	}
	duplicate := forms
	duplicate[1] = duplicate[0]
	r, e = cut.Cell.ProjectAffineBezierPieces2D(ctx, duplicate, plane, ends, ComposedProjectionOptions{})
	if e != ErrDegenerate || r.Complete {
		t.Fatal(r, e)
	}
	r, e = cut.Cell.ProjectAffineBezierPieces2D(ctx, forms, plane, ends, ComposedProjectionOptions{EventOptions: ExactCurvePlaneEventOptions{MaxOperations: 1}})
	if e == nil || r.Complete || len(r.Pieces) != 0 {
		t.Fatal("atomic", r, e)
	}
}
