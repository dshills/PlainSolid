package faceclip

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestWindowRetainsHole(t *testing.T) {
	h := []Halfspace{{geom.Vec3{X: 1}, 3.5}, {geom.Vec3{X: -1}, -.5}, {geom.Vec3{Y: 1}, 3.5}, {geom.Vec3{Y: -1}, -.5}}
	r, e := ClipRegion(context.Background(), holed(t), 1, h, RegionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r.ProjectedArea.Cmp(big.NewRat(5, 1)) != 0 || r.Centroid[0].Cmp(big.NewRat(2, 1)) != 0 || r.Centroid[1].Cmp(big.NewRat(2, 1)) != 0 {
		t.Fatal(r.ProjectedArea, r.Centroid)
	}
	b, e := MergeLoops(context.Background(), r, LoopOptions{})
	if e != nil || len(b.Loops) != 2 {
		t.Fatal(e, len(b.Loops))
	}
}
func TestWindowExactRepeatedIntersections(t *testing.T) {
	h := []Halfspace{{geom.Vec3{X: 3}, 1}, {geom.Vec3{Y: 3}, 2}}
	s := holed(t)
	for i := 0; i < 2; i++ {
		r, e := ClipRegion(context.Background(), s, 1, h, RegionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if r.ProjectedArea.Cmp(big.NewRat(2, 9)) != 0 || r.Centroid[0].Cmp(big.NewRat(1, 6)) != 0 || r.Centroid[1].Cmp(big.NewRat(1, 3)) != 0 {
			t.Fatal(r.ProjectedArea, r.Centroid)
		}
		b, e := MergeLoops(context.Background(), r, LoopOptions{})
		if e != nil || len(b.Loops) != 1 {
			t.Fatal(e)
		}
		h[0], h[1] = h[1], h[0]
	}
}
func TestWindowEmptyFullAndLimits(t *testing.T) {
	s := holed(t)
	r, e := ClipRegion(context.Background(), s, 1, nil, RegionOptions{})
	if e != nil || r.ProjectedArea.Cmp(big.NewRat(12, 1)) != 0 {
		t.Fatal(e)
	}
	h := []Halfspace{{geom.Vec3{X: 1}, 1}, {geom.Vec3{X: -1}, -2}}
	r, e = ClipRegion(context.Background(), s, 1, h, RegionOptions{})
	if e != nil || r.ProjectedArea.Sign() != 0 {
		t.Fatal(e)
	}
	if _, e = ClipRegion(context.Background(), s, 1, h, RegionOptions{MaxHalfspaces: 1}); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	h = append(h, Halfspace{})
	if _, e = ClipRegion(context.Background(), s, 1, h, RegionOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
