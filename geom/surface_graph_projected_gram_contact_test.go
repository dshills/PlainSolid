package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestProjectedQuarticGramOriginalCharts(t *testing.T) {
	ctx := context.Background()
	a, b := representedQuarticGramFixture(t, [5]int64{2, 4, 6, 4, 2})
	budget := &polynomialBudget{maxOperations: 8000000, maxBits: 16384}
	var err error
	a, err = permuteGraphPatch(a, [3]int{0, 2, 1}, budget)
	if err != nil {
		t.Fatal(err)
	}
	b, err = permuteGraphPatch(b, [3]int{0, 2, 1}, budget)
	if err != nil {
		t.Fatal(err)
	}
	old := b.net
	b.net = make([]exactHomogeneous, 25)
	for i := range 5 {
		for j := range 5 {
			b.net[5*i+j] = old[5*(4-j)+i]
		}
	}
	// Independent target units: U in [2,4], V in [-3,1].
	for i := range b.knotsU {
		b.knotsU[i] = 2 + 2*b.knotsU[i]
	}
	for i := range b.knotsV {
		b.knotsV[i] = -3 + 4*b.knotsV[i]
	}
	if err = b.computeBounds(budget); err != nil {
		t.Fatal(err)
	}
	uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
	r, e := CertifyProjectedQuarticGramGraphPointContact(ctx, a, b, [2]int{0, 2}, uv, new(big.Rat), GraphPointContactOptions{})
	if e != nil || !r.Proven || r.Side != 1 || r.ResidualAxis != 1 {
		t.Fatal(r, e)
	}
	if r.TargetParameters[0].Cmp(big.NewRat(14, 5)) != 0 || r.TargetParameters[1].Cmp(big.NewRat(-1, 3)) != 0 || r.Point[1].Cmp(big.NewRat(1, 480)) != 0 || r.Tangent[0][1].Cmp(big.NewRat(1, 160)) != 0 {
		t.Fatal(r)
	}
	if _, e = DiscoverProjectedCoupledQuarticGraphPointContact(ctx, a, b, [2]int{0, 2}, GraphPointContactOptions{}); e == nil {
		t.Fatal("even-only discovery accepted odd terms")
	}
	for _, cap := range []int{r.ExactOperations, r.ExactOperations - 1} {
		got, e := CertifyProjectedQuarticGramGraphPointContact(ctx, a, b, [2]int{0, 2}, uv, new(big.Rat), GraphPointContactOptions{MaxOperations: cap})
		if cap == r.ExactOperations {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || got.Proven || got.Point[0] != nil {
			t.Fatal("non-atomic cap", got, e)
		}
	}
	reverse, e := CertifyProjectedQuarticGramGraphPointContact(ctx, b, a, [2]int{0, 2}, r.TargetParameters, new(big.Rat), GraphPointContactOptions{})
	if e != nil || !reverse.Proven || reverse.Side != -1 || reverse.TargetParameters[0].Cmp(uv[0]) != 0 || reverse.TargetParameters[1].Cmp(uv[1]) != 0 {
		t.Fatal("reversed owners", reverse, e)
	}
	wrong := [2]*big.Rat{big.NewRat(1, 2), uv[1]}
	got, e := CertifyProjectedQuarticGramGraphPointContact(ctx, a, b, [2]int{0, 2}, wrong, new(big.Rat), GraphPointContactOptions{})
	if e == nil || got.Proven || got.TargetParameters[0] != nil {
		t.Fatal("wrong original witness accepted", e)
	}
	got, e = CertifyProjectedQuarticGramGraphPointContact(ctx, a, b, [2]int{0, 1}, uv, new(big.Rat), GraphPointContactOptions{})
	if e == nil || got.Proven {
		t.Fatal("singular projection accepted")
	}
}
