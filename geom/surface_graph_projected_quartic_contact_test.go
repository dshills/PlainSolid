package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestProjectedCoupledQuarticContact(t *testing.T) {
	ctx := context.Background()
	uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
	a, b := coupledQuarticFixture(t, uv, big.NewRat(-1, 1), false)
	budget := &polynomialBudget{maxOperations: 8000000, maxBits: 16384}
	// Original XYZ=(u,height,v); the second support traverses swapped/reversed UV.
	a, e := permuteGraphPatch(a, [3]int{0, 2, 1}, budget)
	if e != nil {
		t.Fatal(e)
	}
	b, e = permuteGraphPatch(b, [3]int{0, 2, 1}, budget)
	if e != nil {
		t.Fatal(e)
	}
	old := b.net
	b.net = make([]exactHomogeneous, 25)
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			b.net[5*i+j] = old[5*(4-j)+i]
		}
	}
	if e = b.computeBounds(budget); e != nil {
		t.Fatal(e)
	}
	r, e := DiscoverProjectedCoupledQuarticGraphPointContact(ctx, a, b, [2]int{0, 2}, GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Side != 1 || r.ResidualAxis != 1 {
		t.Fatal(r, e)
	}
	if r.TargetParameters[0].Cmp(uv[1]) != 0 || r.TargetParameters[1].Cmp(big.NewRat(2, 3)) != 0 || r.Point[0].Cmp(uv[0]) != 0 || r.Point[2].Cmp(uv[1]) != 0 || r.Point[1].Sign() != 0 {
		t.Fatal(r)
	}
	if _, e := DiscoverProjectedCoupledQuarticGraphPointContact(ctx, a, b, [2]int{0, 2}, GraphPointContactOptions{MaxOperations: r.ExactOperations - 1}); e == nil {
		t.Fatal("cumulative cap accepted")
	}
	if _, e := DiscoverProjectedCoupledQuarticGraphPointContact(ctx, a, b, [2]int{0, 1}, GraphPointContactOptions{}); e == nil {
		t.Fatal("singular projection accepted")
	}
}
