package geom

import (
	"context"
	"math/big"
	"testing"
)

func checkComposedPieces(t *testing.T, p *SurfacePatch, uv *ExactSplineCurve, r SurfaceCurveCompositionPieces) {
	t.Helper()
	if !r.Complete || !r.Continuous || len(r.Pieces) == 0 {
		t.Fatal(r)
	}
	for _, piece := range r.Pieces {
		for _, s := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)} {
			global := new(big.Rat).Add(piece.Start, new(big.Rat).Mul(new(big.Rat).Sub(piece.End, piece.Start), s))
			chart, e := uv.patch.EvaluateRational(context.Background(), global, new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want, e := p.EvaluateRational(context.Background(), chart.Point[0], chart.Point[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			got, e := piece.Curve.patch.EvaluateRational(context.Background(), s, new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if got.Point[axis].Cmp(want.Point[axis]) != 0 {
					t.Fatal("independent model evaluation")
				}
			}
		}
	}
}
func TestQuadraticCompositionPiecesRationalCorner(t *testing.T) {
	p, _ := graphContactFixture(t, false, false)
	p, _ = p.InsertUKnot(.5, p.degreeU, SurfacePatchOptions{})
	p, _ = p.InsertVKnot(.5, p.degreeV, SurfacePatchOptions{})
	// u=v=t/(2-t), expressed as an elevated rational quadratic. Both knot
	// coordinates are crossed at exact nonbinary t=2/3.
	uv, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(2, 1)}, {big.NewRat(1, 2), big.NewRat(1, 2), new(big.Rat), big.NewRat(3, 2)}, {big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := p.ComposeParameterCurvePieces(context.Background(), uv, SurfaceCurveCompositionOptions{})
	if e != nil || len(r.Pieces) != 2 || r.Pieces[0].End.Cmp(big.NewRat(2, 3)) != 0 {
		t.Fatal(r, e)
	}
	checkComposedPieces(t, p, uv, r)
	reverse, e := uv.RationalSegment(context.Background(), big.NewRat(1, 1), new(big.Rat), TubeAffineLineOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = p.ComposeParameterCurvePieces(context.Background(), reverse, SurfaceCurveCompositionOptions{})
	if e != nil || r.Pieces[0].End.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
	checkComposedPieces(t, p, reverse, r)
}
func TestQuadraticCompositionPiecesCurvedAndIrrational(t *testing.T) {
	p, _ := graphContactFixture(t, false, false)
	p, _ = p.InsertUKnot(.25, 1, SurfacePatchOptions{})
	p, _ = p.InsertVKnot(.5, 1, SurfacePatchOptions{})
	uv, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := p.ComposeParameterCurvePieces(context.Background(), uv, SurfaceCurveCompositionOptions{})
	if e != nil || len(r.Pieces) != 2 {
		t.Fatal(r, e)
	}
	checkComposedPieces(t, p, uv, r)
	p, _ = p.InsertUKnot(.5, 1, SurfacePatchOptions{})
	r, e = p.ComposeParameterCurvePieces(context.Background(), uv, SurfaceCurveCompositionOptions{})
	if e == nil || r.Complete || len(r.Unresolved) == 0 {
		t.Fatal(r, e)
	}
}
func TestQuadraticCompositionPiecesHullSubdivision(t *testing.T) {
	p, _ := graphContactFixture(t, false, false)
	uv, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(5, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := p.ComposeParameterCurvePieces(context.Background(), uv, SurfaceCurveCompositionOptions{})
	if e != nil || len(r.Pieces) != 2 || r.HullSubdivisions != 1 {
		t.Fatal(r, e)
	}
	checkComposedPieces(t, p, uv, r)
	r, e = p.ComposeParameterCurvePieces(context.Background(), uv, SurfaceCurveCompositionOptions{MaxPieces: 1})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = p.ComposeParameterCurvePieces(ctx, uv, SurfaceCurveCompositionOptions{})
	if e != context.Canceled {
		t.Fatal(e)
	}
}

func TestCompositionKnotEndpointAndInteriorRoot(t *testing.T) {
	p, _ := graphContactFixture(t, false, false)
	p, _ = p.InsertUKnot(.25, 1, SurfacePatchOptions{})
	// u=t²-t/2+1/4 touches the same knot at t=0 and t=1/2.
	uv, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{big.NewRat(1, 4), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(3, 4), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pieces, e := p.ComposeParameterCurvePieces(context.Background(), uv, SurfaceCurveCompositionOptions{})
	if e != nil || !pieces.Complete || len(pieces.Pieces) != 2 || pieces.Pieces[0].End.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(pieces, e)
	}
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil || !cells.Complete || len(cells.Cells) != 2 {
		t.Fatal(cells, e)
	}
}
