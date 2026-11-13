package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestExactModelFiniteLineContacts(t *testing.T) {
	ctx := context.Background()
	net := make([][4]*big.Rat, 4)
	for i, z := range []int64{-1, 1, -1, 1} {
		net[i] = [4]*big.Rat{big.NewRat(int64(i), 1), new(big.Rat), big.NewRat(z, 1), big.NewRat(1, 1)}
	}
	c, e := NewExactHomogeneousSpline(ctx, 1, net, []float64{-2, -2, 0, 2, 4, 4}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	origin := [3]*big.Rat{big.NewRat(1, 3), new(big.Rat), new(big.Rat)}
	dir := [3]*big.Rat{big.NewRat(-2, 3), new(big.Rat), new(big.Rat)}
	limits := [2]*big.Rat{big.NewRat(-4, 1), big.NewRat(1, 1)}
	r, e := c.DiscoverLineContacts(ctx, origin, dir, limits, ComposedLineContactsOptions{})
	if e != nil || !r.Complete || len(r.Contacts) != 3 || r.CellCount != 3 {
		t.Fatal(r, e)
	}
	for i, h := range r.Contacts {
		// Model X=i+1/2, line parameter=(X-1/3)/(-2/3)=-(6i+1)/4.
		for j, root := range []*ExactAlgebraicRoot{h.Event.Root(), h.LineRoot} {
			value := big.NewRat(int64(2*i-1), 1)
			if j == 1 {
				value = big.NewRat(int64(-6*i-1), 4)
			}
			q, _, _ := NewExactRationalRoot(ctx, value, ExactCurvePlaneEventOptions{})
			cmp, e := root.Compare(ctx, q, ExactCurvePlaneEventOptions{})
			if e != nil || !cmp.Proven || cmp.Order != 0 {
				t.Fatal(i, j, cmp, e)
			}
		}
	}
	r, e = c.DiscoverLineContacts(ctx, origin, dir, limits, ComposedLineContactsOptions{MaxEvents: 1})
	if e == nil || r.Complete || len(r.Contacts) != 0 {
		t.Fatal("atomic", r, e)
	}
}
func TestComposedLineContactsKnotAndIrrational(t *testing.T) {
	ctx := context.Background()
	plane := [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}
	cell := composedTouchFixture(t, false)
	hits, e := cell.IntersectRawPlane(ctx, plane, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	root := hits.Events[0].Root()
	a, b := junctionEndpoint(t, cell, root, true), junctionEndpoint(t, cell, root, false)
	cells := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{a.cell, b.cell}}
	origin := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	dir := [3]*big.Rat{big.NewRat(2, 1), new(big.Rat), new(big.Rat)}
	limits := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	r, e := cells.DiscoverLineContacts(ctx, origin, dir, limits, ComposedLineContactsOptions{})
	if e != nil || !r.Complete || len(r.Contacts) != 1 || len(r.Contacts[0].Cells) != 2 {
		t.Fatal(r, e)
	}
	expected, _, e := root.AffineMap(ctx, new(big.Rat), big.NewRat(1, 2), ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cmp, e := r.Contacts[0].LineRoot.Compare(ctx, expected, ExactCurvePlaneEventOptions{})
	if e != nil || !cmp.Proven || cmp.Order != 0 {
		t.Fatal(cmp, e)
	}
	// A straight model cell on the supporting line remains outside strict scope.
	line := junctionLinearCell(t, 1, 0)
	lineOrigin := [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(-1, 2)}
	lineDirection := [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}
	r, e = (SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{line}}).DiscoverLineContacts(ctx, lineOrigin, lineDirection, limits, ComposedLineContactsOptions{})
	if e != ErrConvergence || r.Complete {
		t.Fatal("coincident", r, e)
	}
}
