package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestExactModelPlaneContactsOriginalUnits(t *testing.T) {
	ctx := context.Background()
	net := make([][4]*big.Rat, 4)
	for i, z := range []int64{-1, 1, -1, 1} {
		net[i] = [4]*big.Rat{big.NewRat(int64(3*i), 1), new(big.Rat), big.NewRat(3*z, 1), big.NewRat(3, 1)}
	}
	c, e := NewExactHomogeneousSpline(ctx, 1, net, []float64{-2, -2, 0, 2, 4, 4}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cells, e := c.ModelContactCells(ctx, SurfacePatchOptions{}, 3)
	if e != nil || !cells.Complete || len(cells.Cells) != 3 {
		t.Fatal(cells, e)
	}
	for _, cell := range cells.Cells {
		if cell.HasSurfaceCell() {
			t.Fatal("fabricated support")
		}
	}
	r, e := c.DiscoverPlaneContacts(ctx, [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Events) != 3 {
		t.Fatal(r, e)
	}
	for i, event := range r.Events {
		q, _, _ := NewExactRationalRoot(ctx, big.NewRat(int64(2*i-1), 1), ExactCurvePlaneEventOptions{})
		cmp, e := event.Event.Root().Compare(ctx, q, ExactCurvePlaneEventOptions{})
		if e != nil || !cmp.Proven || cmp.Order != 0 || !event.Local[0].Crossing || event.Cells[0] != i {
			t.Fatal(event, cmp, e)
		}
		enc, e := event.Event.Enclose(ctx, ComposedCurveEndpointOptions{Tolerance: 1e-8})
		if e != nil || enc.Coordinates[2].Lower.Sign() > 0 || enc.Coordinates[2].Upper.Sign() < 0 {
			t.Fatal(enc, e)
		}
	}
	r, e = c.DiscoverPlaneContacts(ctx, [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}, ComposedPlaneContactsOptions{MaxCells: 2})
	if e == nil || r.Complete || len(r.Events) != 0 {
		t.Fatal("atomic spans", r, e)
	}
}
func TestExactModelPlaneContactsClosedKnotTouch(t *testing.T) {
	ctx := context.Background()
	// Closed rational model edge; the two sides meet at the interior knot with
	// different velocities. The plane touches at that knot and no other point.
	net := [][4]*big.Rat{{new(big.Rat), new(big.Rat), big.NewRat(-3, 1), big.NewRat(3, 1)}, {big.NewRat(5, 1), new(big.Rat), new(big.Rat), big.NewRat(5, 1)}, {big.NewRat(6, 1), new(big.Rat), big.NewRat(-3, 1), big.NewRat(3, 1)}, {new(big.Rat), new(big.Rat), big.NewRat(-3, 1), big.NewRat(3, 1)}}
	c, e := NewExactHomogeneousSpline(ctx, 1, net, []float64{-2, -2, 0, 2, 4, 4}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane := [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}
	r, e := c.DiscoverPlaneContacts(ctx, plane, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Events) != 1 || r.Events[0].Junction == nil || !r.Events[0].Junction.Touching || r.Events[0].Junction.SideSigns != [2]int{-1, -1} {
		t.Fatal(r, e)
	}
	r, e = c.DiscoverPlaneContacts(ctx, [4]*big.Rat{nil, big.NewRat(1, 1), nil, nil}, ComposedPlaneContactsOptions{})
	if e != ErrConvergence || r.Complete {
		t.Fatal("coincident", r, e)
	}
	r, e = c.DiscoverPlaneContacts(ctx, plane, ComposedPlaneContactsOptions{EventOptions: ExactCurvePlaneEventOptions{MaxOperations: 1}})
	if e == nil || r.Complete {
		t.Fatal("atomic", r, e)
	}
}
