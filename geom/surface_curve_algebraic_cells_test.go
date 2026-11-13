package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestAlgebraicComposedCellsExactEvaluation(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	rational, e := p.ComposeParameterCurvePieces(context.Background(), uv, SurfaceCurveCompositionOptions{})
	if e == nil || rational.Complete {
		t.Fatal("irrational cut falsely exported")
	}
	r, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil || !r.Complete || len(r.Cells) != 2 || len(r.Events) != 3 {
		t.Fatal(r, e)
	}
	for i, parameters := range [][]*big.Rat{{big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(7, 10)}, {big.NewRat(3, 4), big.NewRat(1, 1)}} {
		for _, parameter := range parameters {
			point, e := r.Cells[i].EvaluateRational(context.Background(), parameter, ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			chart, e := uv.patch.EvaluateRational(context.Background(), parameter, new(big.Rat), SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want, e := p.EvaluateRational(context.Background(), chart.Point[0], chart.Point[1], SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for axis := 0; axis < 3; axis++ {
				if point.Point[axis].Cmp(want.Point[axis]) != 0 {
					t.Fatal("exact global evaluation")
				}
			}
		}
	}
	for i, parameter := range []*big.Rat{big.NewRat(3, 4), big.NewRat(7, 10)} {
		_, e = r.Cells[i].EvaluateRational(context.Background(), parameter, ExactCurvePlaneEventOptions{})
		if !errors.Is(e, ErrDomain) {
			t.Fatal("membership not checked", e)
		}
	}
	// Both public endpoint and polynomial snapshots are independent of proof.
	r.Events[1].Lower.SetInt64(99)
	intervals := r.Cells[0].EndpointIntervals()
	intervals[1].Upper.SetInt64(-5)
	power, _, e := r.Cells[0].HomogeneousPower(context.Background(), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	power[3][0].SetInt64(-1)
	point, e := r.Cells[0].EvaluateRational(context.Background(), big.NewRat(1, 2), ExactCurvePlaneEventOptions{})
	if e != nil || point.Point[0].Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(point, e)
	}
}
func TestAlgebraicComposedCellsLimits(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	r, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{SurfacePatchOptions: SurfacePatchOptions{MaxOperations: 1}})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
	r, e = p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	_, e = r.Cells[0].EvaluateRational(context.Background(), big.NewRat(1, 2), ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil {
		t.Fatal("evaluation budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = r.Cells[0].EvaluateRational(ctx, big.NewRat(1, 2), ExactCurvePlaneEventOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
