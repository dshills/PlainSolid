package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestSphereCylinderDoubleCapEvents(t *testing.T) {
	r := big.NewRat
	ctx := context.Background()
	// R=2, center radial offset1, tool radius1. Heights1 and3/2 yield
	// projection thresholds -1/2 and1/8: different exact chord hubs.
	q, e := OrderSphereCylinderDoubleCapEvents(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 3}, 1, [2]*big.Rat{r(1, 3), r(1, 2)}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete || len(q.Stations) != 4 || len(q.Sectors) != 4 {
		t.Fatal(q, e)
	}
	want := []*big.Rat{r(1, 8), r(-1, 2), r(-1, 2), r(1, 8)}
	sign := []int{1, 1, -1, -1}
	inside := [][2]bool{{true, false}, {false, false}, {true, false}, {true, true}}
	for i, v := range q.Stations {
		if v.ProjectionValue.Cmp(want[i]) != 0 || v.RootSign != sign[i] || q.Sectors[i].InsideCap != inside[i] {
			t.Fatal(i, v, q.Sectors[i])
		}
		for _, owner := range v.Owners {
			p := q.Caps[owner.Cap].Events.Points[owner.Point]
			if p.RootSign != v.RootSign || q.CapPointStation[owner.Cap][owner.Point] != i {
				t.Fatal(owner, p, v)
			}
		}
	}
	symmetric, e := OrderSphereCylinderDoubleCapEvents(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: -2}, 1, [2]*big.Rat{r(-1, 2), r(1, 2)}, SurfacePatchEvaluationOptions{})
	if e != nil || len(symmetric.Stations) != 2 || len(symmetric.Stations[0].Owners) != 2 {
		t.Fatal(symmetric, e)
	}
	for _, station := range symmetric.Stations {
		a, b := station.Owners[0], station.Owners[1]
		pa, pb := symmetric.Caps[a.Cap].Events.Points[a.Point], symmetric.Caps[b.Cap].Events.Points[b.Point]
		if pa.Base[2].Cmp(pb.Base[2]) == 0 || pa.Base[0].Cmp(pb.Base[0]) != 0 || pa.Radicand.Cmp(pb.Radicand) != 0 {
			t.Fatal(pa, pb)
		}
	}
	if symmetric.Sectors[0].InsideCap != [2]bool{false, false} || symmetric.Sectors[1].InsideCap != [2]bool{true, true} {
		t.Fatal(symmetric.Sectors)
	}
	q, e = OrderSphereCylinderDoubleCapEvents(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(0, 1), r(1, 1)}, SurfacePatchEvaluationOptions{})
	if e != ErrDomain || q.Complete {
		t.Fatal(q, e)
	}
	q, e = OrderSphereCylinderDoubleCapEvents(ctx, Vec3{X: 1}, 2, Vec3{}, Vec3{Z: 3}, 1, [2]*big.Rat{r(1, 3), r(1, 2)}, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete || len(q.Stations) != 0 {
		t.Fatal(q, e)
	}
}
