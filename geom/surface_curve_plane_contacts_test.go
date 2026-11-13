package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestComposedPlaneContactTableSupportKnot(t *testing.T) {
	ctx := context.Background()
	p, uv, _, _ := algebraicSeamFixture(t)
	cells, e := p.BuildSurfaceCurveCompositionCells(ctx, uv, SurfaceCurveAgreementOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := cells.DiscoverPlaneContacts(ctx, [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)}, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Events) != 1 {
		t.Fatal(r, e)
	}
	a := r.Events[0]
	if len(a.Cells) != 2 || a.Cells[0] != 0 || a.Cells[1] != 1 || a.Junction == nil || !a.Junction.Crossing || len(a.Local) != 2 {
		t.Fatal(a)
	}
	r, e = cells.DiscoverPlaneContacts(ctx, [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-2, 1)}, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Events) != 0 {
		t.Fatal(r, e)
	}
	r, e = cells.DiscoverPlaneContacts(ctx, [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-1, 2)}, ComposedPlaneContactsOptions{MaxEvents: 1})
	if e == nil || r.Complete || len(r.Events) != 0 {
		t.Fatal("atomic count", r, e)
	}
}
func TestComposedPlaneContactTableOrderedAndMismatch(t *testing.T) {
	ctx := context.Background()
	root, _, _ := NewExactRationalRoot(ctx, big.NewRat(1, 2), ExactCurvePlaneEventOptions{})
	plane := [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}
	before := junctionEndpoint(t, junctionLinearCell(t, 1, 0), root, true)
	after := junctionEndpoint(t, junctionLinearCell(t, -1, 0), root, false)
	cells := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{before.cell, after.cell}}
	r, e := cells.DiscoverPlaneContacts(ctx, plane, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Events) != 1 || r.Events[0].Junction == nil || !r.Events[0].Junction.Touching {
		t.Fatal(r, e)
	}
	// Even when the plane has no root, public Complete cannot hide a bad seam.
	wrong := junctionEndpoint(t, junctionLinearCell(t, -1, 1), root, false)
	cells.Cells[1] = wrong.cell
	r, e = cells.DiscoverPlaneContacts(ctx, [4]*big.Rat{nil, nil, big.NewRat(1, 1), big.NewRat(10, 1)}, ComposedPlaneContactsOptions{})
	if e != ErrDomain || r.Complete || len(r.Events) != 0 {
		t.Fatal(r, e)
	}
	cells.Cells[1] = after.cell
	r, e = cells.DiscoverPlaneContacts(ctx, [4]*big.Rat{nil, big.NewRat(1, 1), nil, nil}, ComposedPlaneContactsOptions{})
	if e != ErrConvergence || r.Complete {
		t.Fatal("coincident", r, e)
	}
	r, e = cells.DiscoverPlaneContacts(ctx, plane, ComposedPlaneContactsOptions{EventOptions: ExactCurvePlaneEventOptions{MaxOperations: 1}})
	if e == nil || r.Complete || len(r.Events) != 0 {
		t.Fatal("atomic work", r, e)
	}
}

func TestComposedPlaneContactTableThreeCells(t *testing.T) {
	ctx := context.Background()
	net := make([][4]*big.Rat, 4)
	zs := []*big.Rat{big.NewRat(-3, 32), big.NewRat(13, 96), big.NewRat(-13, 96), big.NewRat(3, 32)}
	for i := range net {
		net[i] = [4]*big.Rat{big.NewRat(int64(i), 3), new(big.Rat), zs[i], big.NewRat(1, 1)}
	}
	curve, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cuts := []*ExactAlgebraicRoot{}
	for _, q := range []*big.Rat{new(big.Rat), big.NewRat(1, 3), big.NewRat(2, 3), big.NewRat(1, 1)} {
		r, _, e := NewExactRationalRoot(ctx, q, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cuts = append(cuts, r)
	}
	cells := SurfaceCurveCompositionCells{Complete: true}
	for i := 0; i < 3; i++ {
		piece, e := cell.RestrictAlgebraic(ctx, cuts[i], cuts[i+1], ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		cells.Cells = append(cells.Cells, piece.Cell)
	}
	r, e := cells.DiscoverPlaneContacts(ctx, [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Events) != 3 {
		t.Fatal(r, e)
	}
	for i, event := range r.Events {
		if len(event.Cells) != 1 || event.Cells[0] != i || !event.Local[0].Crossing || event.Junction != nil {
			t.Fatal(event)
		}
		q, _, _ := NewExactRationalRoot(ctx, big.NewRat(int64(i+1), 4), ExactCurvePlaneEventOptions{})
		cmp, e := event.Event.Root().Compare(ctx, q, ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 {
			t.Fatal(cmp, e)
		}
	}
}
