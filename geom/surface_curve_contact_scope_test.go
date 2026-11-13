package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestPlaneContactCollectionExplicitScope(t *testing.T) {
	ctx := context.Background()
	curve := coplanarModelFixture(t)
	cells, e := curve.ModelContactCells(ctx, SurfacePatchOptions{}, 10)
	if e != nil {
		t.Fatal(e)
	}
	subset := SurfaceCurveCompositionCells{Complete: true, Cells: append([]*ExactComposedCurveCell(nil), cells.Cells[1:3]...)}
	plane := [4]*big.Rat{nil, big.NewRat(1, 1), nil, big.NewRat(1, 1)}
	r, e := subset.DiscoverPlaneContacts(ctx, plane, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || r.CellCount != 2 || r.Domain[0].Interval().Lower.Sign() != 0 || r.Domain[1].Interval().Lower.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal(r, e)
	}
	components, e := subset.DiscoverPlaneContactComponents(ctx, [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}, ComposedPlaneContactsOptions{})
	if e != nil || !components.Complete || components.CellCount != 2 || components.Domain[0].Interval().Lower.Sign() != 0 || components.Domain[1].Interval().Lower.Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal(components, e)
	}
	// A forged Complete flag with an omitted interior cell cannot cover the gap.
	forged := SurfaceCurveCompositionCells{Complete: true, Cells: []*ExactComposedCurveCell{cells.Cells[0], cells.Cells[2]}}
	r, e = forged.DiscoverPlaneContacts(ctx, plane, ComposedPlaneContactsOptions{})
	if e != ErrDomain || r.Complete || r.Domain[0] != nil || r.CellCount != 0 {
		t.Fatal(r, e)
	}
	// Count guard rejects before dereferencing invalid cells or starting proof.
	oversized := SurfaceCurveCompositionCells{Complete: true, Cells: make([]*ExactComposedCurveCell, 3)}
	r, e = oversized.DiscoverPlaneContacts(ctx, plane, ComposedPlaneContactsOptions{MaxCells: 2})
	if e != ErrConvergence || r.Complete || r.Domain[0] != nil {
		t.Fatal(r, e)
	}
}
func TestPlaneContactClosedEndpointsRemainParameterOccurrences(t *testing.T) {
	ctx := context.Background()
	net := [][4]*big.Rat{}
	for i, z := range []int64{0, 1, -1, 0} {
		x := int64(1)
		if i == 0 || i == 3 {
			x = 0
		}
		net = append(net, [4]*big.Rat{big.NewRat(x, 1), new(big.Rat), big.NewRat(z, 1), big.NewRat(1, 1)})
	}
	c, e := NewExactHomogeneousSpline(ctx, 1, net, []float64{0, 0, 1, 2, 3, 3}, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := c.DiscoverPlaneContacts(ctx, [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}, ComposedPlaneContactsOptions{})
	if e != nil || !r.Complete || len(r.Events) != 3 || r.CellCount != 3 {
		t.Fatal(r, e)
	}
	first, last := r.Events[0].Event, r.Events[2].Event
	cmp, e := first.CompareParameter(ctx, last, ExactCurvePlaneEventOptions{})
	if e != nil || !cmp.Proven || cmp.Order != -1 {
		t.Fatal(cmp, e)
	}
	equal, _, e := first.CompareModelPoint(ctx, last, ExactCurvePlaneEventOptions{})
	if e != nil || !equal {
		t.Fatal(equal, e)
	}
	if r.Domain[0].Interval().Lower.Sign() != 0 || r.Domain[1].Interval().Lower.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal(r.Domain)
	}
}
