package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestAlgebraicCellSimultaneousPlanes(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	planes := [2][4]*big.Rat{{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)}, {nil, big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(-3, 2)}}
	var events [2]*ExactComposedPlaneEvent
	for i, cell := range cells.Cells {
		r, e := cell.IntersectRawPlanePair(context.Background(), planes, ExactCurvePlaneEventOptions{})
		if e != nil || !r.Complete || r.Coincident || len(r.Events) != 1 || r.Events[0].Multiplicity() != 1 {
			t.Fatal(r, e)
		}
		events[i] = r.Events[0]
	}
	comparison, e := events[0].CompareParameter(context.Background(), events[1], ExactCurvePlaneEventOptions{})
	if e != nil || !comparison.Proven || comparison.Order != 0 {
		t.Fatal(comparison, e)
	}
	planes[1][3] = new(big.Rat).Add(big.NewRat(-3, 2), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	for _, cell := range cells.Cells {
		r, e := cell.IntersectRawPlanePair(context.Background(), planes, ExactCurvePlaneEventOptions{})
		if e != nil || !r.Complete || len(r.Events) != 0 || r.Coincident {
			t.Fatal("near nonincident line", r, e)
		}
	}
}
func TestAlgebraicCellPlanePairCoincidenceAndOrder(t *testing.T) {
	s, e := NewBSplineSurface(1, 1, 2, 2, []Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	_, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell := cells.Cells[0]
	planes := [2][4]*big.Rat{{nil, nil, big.NewRat(1, 1), nil}, {big.NewRat(1, 1), nil, nil, nil}}
	r, e := cell.IntersectRawPlanePair(context.Background(), planes, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || len(r.Events) != 1 || r.Events[0].Multiplicity() != 2 || r.Events[0].EndpointMask() != 1 {
		t.Fatal(r, e)
	}
	line, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cells, e = p.BuildSurfaceCurveCompositionCells(context.Background(), line, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	planes[1] = [4]*big.Rat{nil, big.NewRat(1, 1), nil, nil}
	r, e = cells.Cells[0].IntersectRawPlanePair(context.Background(), planes, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || !r.Coincident || r.Boundary[0] == nil {
		t.Fatal(r, e)
	}
	planes[1] = planes[0]
	_, e = cell.IntersectRawPlanePair(context.Background(), planes, ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
}
