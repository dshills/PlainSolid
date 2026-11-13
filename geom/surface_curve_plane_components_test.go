package geom

import (
	"context"
	"math/big"
	"testing"
)

func coplanarModelFixture(t *testing.T) *ExactSplineCurve {
	t.Helper()
	net := make([][4]*big.Rat, 5)
	for i, z := range []int64{-1, 0, 0, 0, 1} {
		net[i] = [4]*big.Rat{big.NewRat(int64(i), 1), new(big.Rat), big.NewRat(z, 1), big.NewRat(1, 1)}
	}
	c, e := NewExactHomogeneousSpline(context.Background(), 1, net, []float64{-2, -2, 0, 1, 2, 4, 4}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func TestComposedPlaneCoincidentComponents(t *testing.T) {
	ctx := context.Background()
	c := coplanarModelFixture(t)
	plane := [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}
	r, e := c.DiscoverPlaneContactComponents(ctx, plane, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 {
		t.Fatal(r, e)
	}
	v := r.Components[0]
	if v.Point != nil || len(v.Cells) != 2 || v.Cells[0] != 1 || v.Cells[1] != 2 || v.Start.Interval().Lower.Sign() != 0 || v.End.Interval().Lower.Cmp(big.NewRat(2, 1)) != 0 || v.Before == nil || v.After == nil || v.Before.SideSigns != [2]int{-1, 0} || v.After.SideSigns != [2]int{0, 1} {
		t.Fatal(v)
	}
	strict, e := c.DiscoverPlaneContacts(ctx, plane, ComposedPlaneContactsOptions{})
	if e != ErrConvergence || strict.Complete {
		t.Fatal("legacy strict", strict, e)
	}
	r, e = c.DiscoverPlaneContactComponents(ctx, plane, ComposedPlaneContactsOptions{MaxEvents: 1})
	if e == nil || r.Complete || len(r.Components) != 0 {
		t.Fatal("atomic", r, e)
	}
	// A complete curve lying in the plane is one interval with no outside germs.
	r, e = c.DiscoverPlaneContactComponents(ctx, [4]*big.Rat{nil, big.NewRat(1, 1), nil, nil}, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Components) != 1 || len(r.Components[0].Cells) != 4 || r.Components[0].Before != nil || r.Components[0].After != nil {
		t.Fatal(r, e)
	}
}
func TestComposedPlaneComponentsRejectBadSeam(t *testing.T) {
	ctx := context.Background()
	root, _, _ := NewExactRationalRoot(ctx, big.NewRat(1, 2), ExactCurvePlaneEventOptions{})
	a := junctionEndpoint(t, junctionLinearCell(t, 1, 0), root, true)
	b := junctionEndpoint(t, junctionLinearCell(t, 1, 1), root, false)
	cells := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{a.cell, b.cell}}
	r, e := cells.DiscoverPlaneContactComponents(ctx, [4]*big.Rat{nil, big.NewRat(1, 1), nil, nil}, ComposedPlaneContactsOptions{})
	if e != ErrDomain || r.Complete || len(r.Components) != 0 {
		t.Fatal(r, e)
	}
}
