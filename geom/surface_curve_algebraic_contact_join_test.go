package geom

import (
	"context"
	"math/big"
	"testing"
)

func junctionLinearCell(t *testing.T, sign, y int64) *ExactComposedCurveCell {
	t.Helper()
	ctx := context.Background()
	curve, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{new(big.Rat), big.NewRat(y, 1), big.NewRat(-sign, 2), big.NewRat(1, 1)}, {big.NewRat(1, 1), big.NewRat(y, 1), big.NewRat(sign, 2), big.NewRat(1, 1)}}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return cell
}
func junctionEndpoint(t *testing.T, cell *ExactComposedCurveCell, root *ExactAlgebraicRoot, left bool) *ExactComposedPlaneEvent {
	t.Helper()
	ctx := context.Background()
	a, _ := cell.EndpointRoot(0)
	b, _ := cell.EndpointRoot(1)
	if left {
		b = root
	} else {
		a = root
	}
	cut, e := cell.RestrictAlgebraic(ctx, a, b, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	event, e := cut.Cell.BindRoot(ctx, root, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return event
}
func TestComposedPlaneJunctionC0(t *testing.T) {
	ctx := context.Background()
	root, _, _ := NewExactRationalRoot(ctx, big.NewRat(1, 2), ExactCurvePlaneEventOptions{})
	plane := [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}
	left := junctionEndpoint(t, junctionLinearCell(t, 1, 0), root, true)
	for _, sign := range []int64{1, -1} {
		right := junctionEndpoint(t, junctionLinearCell(t, sign, 0), root, false)
		r, e := ClassifyComposedPlaneJunction(ctx, left, right, plane, ExactCurvePlaneEventOptions{})
		if e != nil || !r.Proven || r.Crossing != (sign == 1) || r.Touching != (sign == -1) || !r.Before.Transverse || !r.After.Transverse || r.SideSigns != [2]int{-1, int(sign)} {
			t.Fatal(r, e)
		}
	}
	// Same parameter and plane but different model point is not a C0 join.
	wrong := junctionEndpoint(t, junctionLinearCell(t, 1, 1), root, false)
	r, e := ClassifyComposedPlaneJunction(ctx, left, wrong, plane, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || r.Proven || r.Before.Proven {
		t.Fatal(r, e)
	}
	right := junctionEndpoint(t, junctionLinearCell(t, 1, 0), root, false)
	shifted := plane
	shifted[3] = big.NewRat(1, 1000000)
	r, e = ClassifyComposedPlaneJunction(ctx, left, right, shifted, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || r.Proven {
		t.Fatal(r, e)
	}
	r, e = ClassifyComposedPlaneJunction(ctx, left, right, plane, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || r.Proven || r.Before.Proven {
		t.Fatal("atomic", r, e)
	}
	other, _, _ := NewExactRationalRoot(ctx, big.NewRat(2, 3), ExactCurvePlaneEventOptions{})
	unmatched := junctionEndpoint(t, junctionLinearCell(t, 1, 0), other, false)
	r, e = ClassifyComposedPlaneJunction(ctx, left, unmatched, plane, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || r.Proven {
		t.Fatal(r, e)
	}
}
func TestComposedPlaneJunctionAlgebraicTouch(t *testing.T) {
	ctx := context.Background()
	cell := composedTouchFixture(t, false)
	plane := [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}
	hits, e := cell.IntersectRawPlane(ctx, plane, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	root := hits.Events[0].Root()
	left, right := junctionEndpoint(t, cell, root, true), junctionEndpoint(t, cell, root, false)
	r, e := ClassifyComposedPlaneJunction(ctx, left, right, plane, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || !r.Touching || r.Crossing || r.SideSigns != [2]int{1, 1} || !r.Before.Tangent || !r.After.Tangent {
		t.Fatal(r, e)
	}
}

func TestComposedPlaneJunctionActualSupportKnot(t *testing.T) {
	ctx := context.Background()
	p, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(ctx, uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane := [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)}
	var events [2]*ExactComposedPlaneEvent
	for i := 0; i < 2; i++ {
		hits, e := cells.Cells[i].IntersectRawPlane(ctx, plane, ExactCurvePlaneEventOptions{})
		if e != nil || len(hits.Events) != 1 {
			t.Fatal(hits, e)
		}
		events[i] = hits.Events[0]
	}
	r, e := ClassifyComposedPlaneJunction(ctx, events[0], events[1], plane, ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || !r.Crossing || r.SideSigns != [2]int{-1, 1} {
		t.Fatal(r, e)
	}
}
