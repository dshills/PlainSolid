package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactAffinePiecesDifferentBasesAndReversal(t *testing.T) {
	makePatch := func(degree int) *SurfacePatch {
		var poles []Vec3
		for i := 0; i <= degree; i++ {
			for j := 0; j <= degree; j++ {
				u, v := float64(i)/float64(degree), float64(j)/float64(degree)
				poles = append(poles, Vec3{u, v, u * v})
			}
		}
		knots := make([]float64, 2*(degree+1))
		for i := degree + 1; i < len(knots); i++ {
			knots[i] = 1
		}
		s, e := NewBSplineSurface(degree, degree, degree+1, degree+1, poles, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	a, b := makePatch(1), makePatch(2)
	b, _ = b.InsertUKnot(.5, 1, SurfacePatchOptions{})
	b, _ = b.InsertVKnot(.25, 1, SurfacePatchOptions{})
	start, end := [2]*big.Rat{big.NewRat(1, 7), big.NewRat(1, 9)}, [2]*big.Rat{big.NewRat(5, 6), big.NewRat(7, 8)}
	x, e := a.AffineParameterCurvePieces(context.Background(), start, end, SurfaceAffineCurveOptions{})
	if e != nil {
		t.Fatal(e)
	}
	y, e := b.AffineParameterCurvePieces(context.Background(), start, end, SurfaceAffineCurveOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := CompareAffineParameterCurvePieces(context.Background(), x, y, ExactCurveComparisonOptions{})
	if e != nil || !r.Complete || !r.Equal || r.ComparedSpans != 3 {
		t.Fatal(r, e)
	}
	reversed, e := b.AffineParameterCurvePieces(context.Background(), end, start, SurfaceAffineCurveOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = CompareAffineParameterCurvePieces(context.Background(), x, reversed, ExactCurveComparisonOptions{ReverseB: true})
	if e != nil || !r.Complete || !r.Equal {
		t.Fatal(r, e)
	}
	transformed, e := y.Pieces[1].Curve.Transform(Translation(Vec3{Z: 1.0 / (1 << 50)}), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	y.Pieces[1].Curve = transformed
	r, e = CompareAffineParameterCurvePieces(context.Background(), x, y, ExactCurveComparisonOptions{})
	if e != nil || !r.Complete || r.Equal || r.DifferentAxis != 2 {
		t.Fatal(r, e)
	}
	y.Pieces[0].Start = big.NewRat(1, 100)
	if _, e = CompareAffineParameterCurvePieces(context.Background(), x, y, ExactCurveComparisonOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	r, e = CompareAffineParameterCurvePieces(context.Background(), x, reversed, ExactCurveComparisonOptions{ReverseB: true, MaxOperations: 1})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
