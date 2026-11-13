package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalMeridianSliceClearance(t *testing.T) {
	ctx := context.Background()
	f, _ := NewFrame(Vec3{}, Vec3{1, 0, 0}, Vec3{0, 0, 1})
	a, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, .5}, {2, 1}}, 2, RightHanded, -1, 2)
	c, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, .5}, {2, 1}}, 2, RightHanded, 0, 3)
	for _, test := range []struct{ shift, want *big.Rat }{{big.NewRat(2, 3), big.NewRat(1, 6)}, {big.NewRat(1, 2), big.NewRat(0, 1)}, {big.NewRat(1, 3), big.NewRat(-1, 6)}, {big.NewRat(-2, 3), big.NewRat(1, 6)}} {
		r, e := CompareHelicalMeridianSlices(ctx, a, c, big.NewRat(5, 2), test.shift, SurfacePatchEvaluationOptions{})
		if e != nil || !r.Complete || !r.Comparable || r.SignedClearance.Cmp(test.want) != 0 || r.A.Lower.Cmp(big.NewRat(1, 4)) != 0 || r.A.Upper.Cmp(big.NewRat(3, 4)) != 0 || r.CommonTurns[0].Sign() != 0 || r.CommonTurns[1].Cmp(big.NewRat(2, 1)) != 0 {
			t.Fatal(r, e)
		}
	}
	r, e := CompareHelicalMeridianSlices(ctx, a, c, big.NewRat(3, 1), big.NewRat(0, 1), SurfacePatchEvaluationOptions{})
	if e != nil || !r.Comparable || r.SignedClearance.Sign() != 0 || len(r.A.LowerEdges) != 2 {
		t.Fatal(r, e)
	}
	outside, e := CompareHelicalMeridianSlices(ctx, a, c, big.NewRat(4, 1), big.NewRat(0, 1), SurfacePatchEvaluationOptions{})
	if e != nil || !outside.Complete || outside.Comparable {
		t.Fatal(outside, e)
	}
	// Containment clearance is translation to separate, not overlap length.
	wide, _ := NewHelicalPolygon(f, []Vec2{{2, -2}, {3, -2}, {3, 2}, {2, 2}}, 2, RightHanded, 0, 2)
	inside, e := CompareHelicalMeridianSlices(ctx, wide, c, big.NewRat(5, 2), big.NewRat(-1, 2), SurfacePatchEvaluationOptions{})
	if e != nil || inside.SignedClearance.Cmp(big.NewRat(-9, 4)) != 0 {
		t.Fatal(inside, e)
	}
	incompatible, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, .5}, {2, 1}}, 3, RightHanded, 0, 2)
	if _, e := CompareHelicalMeridianSlices(ctx, a, incompatible, big.NewRat(5, 2), big.NewRat(0, 1), SurfacePatchEvaluationOptions{}); e == nil {
		t.Fatal("different lead")
	}
	if r, e := CompareHelicalMeridianSlices(ctx, a, c, big.NewRat(5, 2), big.NewRat(0, 1), SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil || r.Complete || r.Comparable {
		t.Fatal("budget", r, e)
	}
}
