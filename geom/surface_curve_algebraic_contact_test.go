package geom

import (
	"context"
	"math/big"
	"testing"
)

func composedTouchFixture(t *testing.T, singular bool) *ExactComposedCurveCell {
	t.Helper()
	zs := []*big.Rat{big.NewRat(1, 4), big.NewRat(1, 4), big.NewRat(1, 12), big.NewRat(-1, 4), big.NewRat(1, 4)}
	net := make([][4]*big.Rat, 5)
	for i := range net {
		x := big.NewRat(int64(i), 4)
		if singular {
			x = new(big.Rat)
		}
		net[i] = [4]*big.Rat{x, new(big.Rat), zs[i], big.NewRat(1, 1)}
	}
	c, e := NewExactHomogeneousBezier(context.Background(), net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(context.Background(), c, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return cell
}
func TestComposedPlaneContactIrrationalTouch(t *testing.T) {
	ctx := context.Background()
	plane := [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}
	for _, singular := range []bool{false, true} {
		cell := composedTouchFixture(t, singular)
		hits, e := cell.IntersectRawPlane(ctx, plane, ExactCurvePlaneEventOptions{})
		if e != nil || len(hits.Events) != 1 {
			t.Fatal(hits, e)
		}
		event := hits.Events[0]
		r, e := event.ClassifyPlaneContact(ctx, plane, ExactCurvePlaneEventOptions{})
		if e != nil || !r.Proven || r.Order != 2 || r.Singular != singular || r.Tangent == singular || !r.Touching || r.Crossing || r.SideSigns != [2]int{1, 1} {
			t.Fatal(r, e)
		}
		reversed := plane
		reversed[2] = big.NewRat(-1, 1)
		r, e = event.ClassifyPlaneContact(ctx, reversed, ExactCurvePlaneEventOptions{})
		if e != nil || r.SideSigns != [2]int{-1, -1} {
			t.Fatal(r, e)
		}
		// Exact algebraic restriction changes availability, not continuation order.
		end, _ := cell.EndpointRoot(1)
		cut, e := cell.RestrictAlgebraic(ctx, event.Root(), end, ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		bound, e := cut.Cell.BindRoot(ctx, event.Root(), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		r, e = bound.ClassifyPlaneContact(ctx, plane, ExactCurvePlaneEventOptions{})
		if e != nil || r.EndpointMask != 1 || r.Touching || r.Crossing || r.SideSigns != [2]int{0, 1} {
			t.Fatal(r, e)
		}
		wrong := plane
		wrong[3] = big.NewRat(1, 1)
		r, e = event.ClassifyPlaneContact(ctx, wrong, ExactCurvePlaneEventOptions{})
		if e != ErrDomain || r.Proven {
			t.Fatal(r, e)
		}
		r, e = event.ClassifyPlaneContact(ctx, plane, ExactCurvePlaneEventOptions{MaxOperations: 1})
		if e == nil || r.Proven {
			t.Fatal("atomic", r, e)
		}
	}
}
func TestComposedPlaneContactOddAndCoincident(t *testing.T) {
	ctx := context.Background()
	net := [][4]*big.Rat{}
	for i, z := range []int64{-1, 1, -1, 1} {
		net = append(net, [4]*big.Rat{big.NewRat(int64(i), 3), new(big.Rat), big.NewRat(z, 8), big.NewRat(1, 1)})
	}
	curve, e := NewExactHomogeneousBezier(ctx, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cell, e := NewExactComposedBezierCell(ctx, curve, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	plane := [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}
	hits, e := cell.IntersectRawPlane(ctx, plane, ExactCurvePlaneEventOptions{})
	if e != nil || len(hits.Events) != 1 {
		t.Fatal(hits, e)
	}
	r, e := hits.Events[0].ClassifyPlaneContact(ctx, plane, ExactCurvePlaneEventOptions{})
	if e != nil || r.Order != 3 || !r.Tangent || !r.Crossing || r.SideSigns != [2]int{-1, 1} {
		t.Fatal(r, e)
	}
	r, e = hits.Events[0].ClassifyPlaneContact(ctx, [4]*big.Rat{nil, big.NewRat(1, 1), nil, nil}, ExactCurvePlaneEventOptions{})
	if e != ErrConvergence || r.Proven {
		t.Fatal(r, e)
	}
}
