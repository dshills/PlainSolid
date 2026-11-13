package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestMultispanAffineParameterPiecesExactCuts(t *testing.T) {
	p, _ := graphContactFixture(t, false, true)
	var e error
	p, e = p.InsertUKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.5, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertVKnot(.5, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	start, end := [2]*big.Rat{big.NewRat(1, 7), big.NewRat(1, 9)}, [2]*big.Rat{big.NewRat(5, 6), big.NewRat(7, 8)}
	for _, pair := range [2][2][2]*big.Rat{{start, end}, {end, start}} {
		r, e := p.AffineParameterCurvePieces(context.Background(), pair[0], pair[1], SurfaceAffineCurveOptions{})
		if e != nil || !r.Complete || !r.Continuous || len(r.Pieces) != 4 {
			t.Fatal(r, e)
		}
		for _, piece := range r.Pieces {
			local := big.NewRat(1, 3)
			global := new(big.Rat).Add(piece.Start, new(big.Rat).Mul(local, new(big.Rat).Sub(piece.End, piece.Start)))
			var uv [2]*big.Rat
			for axis := 0; axis < 2; axis++ {
				uv[axis] = new(big.Rat).Add(pair[0][axis], new(big.Rat).Mul(global, new(big.Rat).Sub(pair[1][axis], pair[0][axis])))
			}
			want, e := p.EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			got, e := piece.Curve.patch.EvaluateRational(context.Background(), local, new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if got.Point[axis].Cmp(want.Point[axis]) != 0 {
					t.Fatal("exact multispan mismatch")
				}
			}
		}
	}
	// Simultaneous U/V knot crossings become one exact cut each.
	p, _ = graphContactFixture(t, false, true)
	p, _ = p.InsertUKnot(.5, 1, SurfacePatchOptions{})
	p, _ = p.InsertVKnot(.5, 1, SurfacePatchOptions{})
	start, end = [2]*big.Rat{big.NewRat(1, 7), big.NewRat(1, 7)}, [2]*big.Rat{big.NewRat(5, 6), big.NewRat(5, 6)}
	r, e := p.AffineParameterCurvePieces(context.Background(), start, end, SurfaceAffineCurveOptions{})
	if e != nil || len(r.Pieces) != 2 || r.Pieces[0].End.Cmp(big.NewRat(15, 29)) != 0 {
		t.Fatal(r, e)
	}
	// A segment contained in a knot line uses the right-hand cell.
	r, e = p.AffineParameterCurvePieces(context.Background(), [2]*big.Rat{big.NewRat(1, 2), new(big.Rat)}, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 1)}, SurfaceAffineCurveOptions{})
	if e != nil || len(r.Pieces) != 2 {
		t.Fatal(r, e)
	}
	for _, piece := range r.Pieces {
		if piece.Cell.UMin != .5 {
			t.Fatal(piece.Cell)
		}
	}
	r, e = p.AffineParameterCurvePieces(context.Background(), start, end, SurfaceAffineCurveOptions{MaxPieces: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
}
func TestMultispanAffinePiecesC0Only(t *testing.T) {
	s, _ := NewBSplineSurface(1, 1, 3, 2, []Vec3{{}, {Y: 1}, {X: .5}, {X: .5, Y: 1}, {X: 1, Z: .5}, {X: 1, Y: 1, Z: .5}}, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	r, e := p.AffineParameterCurvePieces(context.Background(), [2]*big.Rat{new(big.Rat), big.NewRat(1, 4)}, [2]*big.Rat{big.NewRat(1, 1), big.NewRat(3, 4)}, SurfaceAffineCurveOptions{})
	if e != nil || !r.Complete || !r.Continuous || len(r.Pieces) != 2 {
		t.Fatal(r, e)
	}
}
