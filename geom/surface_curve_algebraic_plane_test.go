package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestAlgebraicCellPlaneSharedEndpoint(t *testing.T) {
	p, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane := [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)}
	var hits [2]*ExactComposedPlaneEvent
	for i, cell := range cells.Cells {
		r, e := cell.IntersectRawPlane(context.Background(), plane, ExactCurvePlaneEventOptions{})
		mask := uint8(2)
		if i == 1 {
			mask = 1
		}
		if e != nil || !r.Complete || r.Coincident || len(r.Events) != 1 || r.Events[0].EndpointMask() != mask || r.Events[0].Multiplicity() != 1 {
			t.Fatal(r, e)
		}
		hits[i] = r.Events[0]
		bounds, e := hits[i].Enclose(context.Background(), ComposedCurveEndpointOptions{Tolerance: 1e-9})
		if e != nil || !bounds.Converged || bounds.Coordinates[0].Lower.Cmp(big.NewRat(1, 2)) > 0 || bounds.Coordinates[0].Upper.Cmp(big.NewRat(1, 2)) < 0 {
			t.Fatal(bounds, e)
		}
	}
	order, e := hits[0].CompareParameter(context.Background(), hits[1], ExactCurvePlaneEventOptions{})
	if e != nil || !order.Proven || order.Order != 0 {
		t.Fatal(order, e)
	}
	// A nearby plane contact is strictly outside the first actual cell, although
	// its initial rational root bracket can overlap the algebraic boundary.
	plane[3] = new(big.Rat).Sub(big.NewRat(-1, 2), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	for i, cell := range cells.Cells {
		r, e := cell.IntersectRawPlane(context.Background(), plane, ExactCurvePlaneEventOptions{})
		if e != nil || !r.Complete || len(r.Events) != i {
			t.Fatal(i, r, e)
		}
		if i == 1 && r.Events[0].EndpointMask() != 0 {
			t.Fatal("near endpoint snapped")
		}
	}
}
func TestAlgebraicCellPlaneEndpointsInteriorAndCoincidence(t *testing.T) {
	p, _ := graphContactFixture(t, false, false)
	uv, e := NewExactHomogeneousBezier(context.Background(), [][4]*big.Rat{{new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cells, e := p.BuildSurfaceCurveCompositionCells(context.Background(), uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell := cells.Cells[0]
	r, e := cell.IntersectRawPlane(context.Background(), [4]*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 1), nil, nil}, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || len(r.Events) != 2 || r.Events[0].EndpointMask() != 1 || r.Events[1].EndpointMask() != 2 {
		t.Fatal(r, e)
	}
	r, e = cell.IntersectRawPlane(context.Background(), [4]*big.Rat{nil, nil, big.NewRat(1, 1), big.NewRat(-1, 2)}, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || len(r.Events) != 2 || r.Events[0].EndpointMask() != 1 || r.Events[1].EndpointMask() != 0 {
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
	r, e = cells.Cells[0].IntersectRawPlane(context.Background(), [4]*big.Rat{nil, big.NewRat(1, 1), nil, nil}, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Complete || !r.Coincident || len(r.Events) != 0 || r.Boundary[0] == nil || r.Boundary[1] == nil {
		t.Fatal(r, e)
	}
	_, e = cell.IntersectRawPlane(context.Background(), [4]*big.Rat{}, ExactCurvePlaneEventOptions{})
	if e == nil {
		t.Fatal("invalid plane")
	}
	r, e = cell.IntersectRawPlane(context.Background(), [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)}, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
