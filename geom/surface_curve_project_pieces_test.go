package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestComposedProjectionMixedWeightSubdivision(t *testing.T) {
	ctx := context.Background()
	// W=(t-1/2)^2+1/16 >0. Degree-six elevation has positive controls,
	// but its degree-four representation has a negative middle coefficient.
	powers := [][]*big.Rat{{new(big.Rat), big.NewRat(5, 16), big.NewRat(-1, 1), big.NewRat(1, 1)}, {new(big.Rat), new(big.Rat), big.NewRat(5, 16), big.NewRat(-1, 1), big.NewRat(1, 1)}, nil, {big.NewRat(5, 16), big.NewRat(-1, 1), big.NewRat(1, 1)}}
	net := make([][4]*big.Rat, 7)
	for i := range net {
		for axis, p := range powers {
			net[i][axis] = new(big.Rat)
			for j := 0; j <= i && j < len(p); j++ {
				ratio := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(6, int64(j)))
				net[i][axis].Add(net[i][axis], new(big.Rat).Mul(p[j], ratio))
			}
		}
		if net[i][3].Sign() <= 0 {
			t.Fatal("fixture weights")
		}
	}
	curve, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	ends := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	if _, e = cell.ProjectBezier2D(ctx, [2]int{0, 1}, ends, ExactCurvePlaneEventOptions{}); e != ErrConvergence {
		t.Fatal("strict weight proof", e)
	}
	r, e := cell.ProjectBezierPieces2D(ctx, [2]int{0, 1}, ends, ComposedProjectionOptions{})
	if e != nil || !r.Complete || len(r.Pieces) != 2 {
		t.Fatal(r, e)
	}
	if r.Pieces[0].OriginalRange[1].Cmp(big.NewRat(1, 2)) != 0 || r.Pieces[1].OriginalRange[0].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("shared exact cut")
	}
	for _, piece := range r.Pieces {
		p, e := NewExactComposedBezierCell(ctx, piece.Curve, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, s := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(1, 1)} {
			v, e := p.EvaluateRational(ctx, s, ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			t0 := new(big.Rat).Add(piece.OriginalRange[0], new(big.Rat).Mul(s, new(big.Rat).Sub(piece.OriginalRange[1], piece.OriginalRange[0])))
			if v.Point[0].Cmp(t0) != 0 || v.Point[1].Cmp(new(big.Rat).Mul(t0, t0)) != 0 {
				t.Fatal(v, t0)
			}
		}
	}
	r, e = cell.ProjectBezierPieces2D(ctx, [2]int{0, 1}, ends, ComposedProjectionOptions{MaxPieces: 1})
	if e == nil || r.Complete || len(r.Pieces) != 0 {
		t.Fatal("atomic", r, e)
	}
}
