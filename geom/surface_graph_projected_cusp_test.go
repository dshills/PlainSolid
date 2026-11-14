package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestProjectedCuspIndependentOriginalCharts(t *testing.T) {
	ctx := context.Background()
	a, c := semicubicalFixture(t)
	budget := &polynomialBudget{maxOperations: 8000000, maxBits: 16384}
	var e error
	a, e = permuteGraphPatch(a, [3]int{0, 2, 1}, budget)
	if e != nil {
		t.Fatal(e)
	}
	c, e = permuteGraphPatch(c, [3]int{0, 2, 1}, budget)
	if e != nil {
		t.Fatal(e)
	}
	// Target U maps to source V; target V is reversed source U. Different degrees.
	old := c.net
	c.net = make([]exactHomogeneous, 12)
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			c.net[i*4+j] = old[(3-j)*3+i]
		}
	}
	c.degreeU, c.degreeV = c.degreeV, c.degreeU
	c.countU, c.countV = c.countV, c.countU
	c.knotsU, c.knotsV = c.knotsV, c.knotsU
	for i := range c.knotsU {
		c.knotsU[i] = 2 + 2*c.knotsU[i]
	}
	for i := range c.knotsV {
		c.knotsV[i] = -3 + 4*c.knotsV[i]
	}
	if e = c.computeBounds(budget); e != nil {
		t.Fatal(e)
	}
	call := func(cap int) (*ProjectedGraphCusp, error) {
		return CertifyProjectedGraphSemicubicalCusp(ctx, a, c, [2]int{0, 2}, big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 2), GraphPointContactOptions{MaxOperations: cap})
	}
	proof, e := call(0)
	if e != nil || !proof.Binds(a, c) || proof.Binds(c, a) {
		t.Fatal(proof, e)
	}
	params, _ := proof.Parameters(1)
	want := [3][2]*big.Rat{{big.NewRat(3, 1), new(big.Rat)}, {big.NewRat(9, 4), big.NewRat(-3, 1)}, {big.NewRat(15, 4), big.NewRat(-3, 1)}}
	for i := range want {
		for j := range want[i] {
			if params[i][j].Cmp(want[i][j]) != 0 {
				t.Fatal(params)
			}
		}
	}
	pts, _ := proof.Points()
	if pts[0][1].Cmp(big.NewRat(1, 512)) != 0 {
		t.Fatal(pts)
	}
	// Both terminal owners are target VMin, not UMax; preserve original maps.
	mapping, _ := proof.TargetMap()
	if mapping[0][1].Cmp(big.NewRat(2, 1)) != 0 || mapping[1][0].Cmp(big.NewRat(-4, 1)) != 0 {
		t.Fatal(mapping)
	}
	mapping[0][1].SetInt64(99)
	params[0][0].SetInt64(99)
	pts[0][0].SetInt64(99)
	fresh, _ := proof.Parameters(1)
	if fresh[0][0].Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("alias")
	}
	if _, e = call(proof.ExactOperations()); e != nil {
		t.Fatal(e)
	}
	if got, e := call(proof.ExactOperations() - 1); e == nil || got != nil {
		t.Fatal("budget", e)
	}
	if got, e := CertifyProjectedGraphSemicubicalCusp(ctx, a, c, [2]int{0, 1}, big.NewRat(1, 4), big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 2), GraphPointContactOptions{}); e == nil || got != nil {
		t.Fatal("wrong projection")
	}
	c.net[0][1] = new(big.Rat).Add(c.net[0][1], big.NewRat(1, 1024))
	if got, e := call(0); e == nil || got != nil {
		t.Fatal("changed original source")
	}
	var zero ProjectedGraphCusp
	if _, e = zero.Points(); e == nil {
		t.Fatal("forged zero")
	}
}
