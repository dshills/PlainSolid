//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestFilletedFrustumNestedTorusBoreOrdering(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: 6}, Radius: 1}}, o)
	if e != nil {
		t.Fatal(e)
	}
	root := func(v *big.Rat) (*big.Rat, *big.Rat) {
		n := new(big.Int).Quo(new(big.Int).Lsh(new(big.Int).Set(v.Num()), 512), v.Denom())
		n.Sqrt(n)
		d := new(big.Int).Lsh(big.NewInt(1), 256)
		return new(big.Rat).SetFrac(n, d), new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), d)
	}
	// Independent nested-radical enclosure, with truly irrational stock rho.
	S, T := r(592901, 10000), r(28901, 10000)
	rl, rh := root(S)
	tl, th := root(T)
	if fdSquare(rl).Cmp(S) >= 0 || fdSquare(rh).Cmp(S) <= 0 {
		t.Fatal("strict irrational azimuth fixture")
	}
	bl, bh := fdSquare(fdSub(tl, r(1, 1))), fdSquare(fdSub(th, r(1, 1)))
	distances := func(z *big.Rat) (*big.Rat, *big.Rat) {
		b2 := fdSquare(fdSub(z, r(15, 16)))
		dl, _ := root(fdAdd(fdSquare(fdSub(rl, r(123, 16))), b2))
		_, dh := root(fdAdd(fdSquare(fdSub(rh, r(123, 16))), b2))
		return fdSquare(fdSub(r(15, 16), dh)), fdSquare(fdSub(r(15, 16), dl))
	}
	left, right := r(69, 100), r(71, 100)
	for i := 0; i < 80; i++ {
		mid := fdMul(fdAdd(left, right), r(1, 2))
		lo, hi := distances(mid)
		if hi.Cmp(bl) < 0 {
			left = mid
		} else if lo.Cmp(bh) > 0 {
			right = mid
		} else {
			t.Fatal("independent oracle ambiguous")
		}
	}
	for i, z := range []*big.Rat{left, right} {
		p := [3]*big.Rat{r(77, 10), r(1, 100), z}
		opts := FilletedFrustumNestedBoreOptions{Support: FilletedFrustumDistanceOptions{SqrtBits: 8}}
		got, e := q.ClassifyNestedTorusBoreFacesRational(ctx, p, opts)
		sign := -1
		if i == 1 {
			sign = 1
		}
		if e != nil || !got.Complete || got.Comparison != sign || len(got.Tied) != 1 || got.RefinementChecks < 2 || got.MaxBitsUsed <= 8 || got.MaxBitsUsed > 256 {
			t.Fatal("refined strict order", got, e)
		}
		lo, hi := distances(z)
		owner := got.Nested.Face
		if sign > 0 {
			lo, hi = bl, bh
			owner = got.Bore.Face
		}
		// Independently bounded truth must intersect the tighter output interval.
		if got.Tied[0] != owner || got.Lower.Cmp(hi) > 0 || got.Upper.Cmp(lo) < 0 || len(got.Excluded) != len(q.solid.faces)-1 {
			t.Fatal("selected enclosure/owner")
		}
		if got.Nested.AzimuthSquared.Cmp(S) != 0 || got.Bore.Radicand.Cmp(T) != 0 {
			t.Fatal("original radical identity")
		}
		opts.MaxRefinements = 1
		limited, e := q.ClassifyNestedTorusBoreFacesRational(ctx, p, opts)
		if e != nil || limited.Complete || limited.Nested != nil || limited.Bore != nil || len(limited.Unresolved) != len(q.solid.faces) || limited.RefinementChecks > 2 {
			t.Fatal("no guessed coarse order", limited, e)
		}
		legacy, e := q.ClassifyTorusBoreFacesRational(ctx, p, FilletedFrustumTorusBoreOptions{})
		if e != nil || legacy.Complete {
			t.Fatal("old rational-radius domain changed")
		}
	}
	// True equality is deliberately not accepted by interval separation. The
	// symbolic rational-azimuth API independently certifies this exact tie.
	tieP := [3]*big.Rat{r(991, 128), r(0, 1), r(3, 4)}
	exact, e := q.ClassifyTorusBoreFacesRational(ctx, tieP, FilletedFrustumTorusBoreOptions{})
	if e != nil || !exact.Complete || exact.Comparison != 0 {
		t.Fatal("known tie", e)
	}
	no, e := q.ClassifyNestedTorusBoreFacesRational(ctx, tieP, FilletedFrustumNestedBoreOptions{})
	if e != nil || no.Complete || len(no.Tied) != 0 || no.Nested != nil || no.Bore != nil || no.RefinementChecks != 2 {
		t.Fatal("false interval tie", no, e)
	}
	no, e = q.ClassifyNestedTorusBoreFacesRational(ctx, tieP, FilletedFrustumNestedBoreOptions{MaxPairs: 1})
	if e == nil || no.Lower != nil || no.Nested != nil {
		t.Fatal("atomic cumulative pair budget")
	}
	// A genuinely closer second bore defeats the first pair and is selected
	// only after its own retained candidate and all other owners are proved.
	two, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: 6}, Radius: 1}, {Center: geom.Vec2{X: 29. / 4}, Radius: 1. / 8}}, o)
	if e != nil {
		t.Fatal(e)
	}
	near, e := two.ClassifyNestedTorusBoreFacesRational(ctx, [3]*big.Rat{r(77, 10), r(1, 100), left}, FilletedFrustumNestedBoreOptions{})
	if e != nil || !near.Complete || near.Comparison != 1 || near.Tied[0] != two.cuts[1].wall || near.PairChecks != 2 {
		t.Fatal("closer third support", near, e)
	}
	for _, p := range [][3]*big.Rat{{r(123, 16), r(0, 1), r(1, 2)}, {r(63, 8), r(0, 1), r(11, 16)}} {
		endpoint, e := q.ClassifyNestedTorusBoreFacesRational(ctx, p, FilletedFrustumNestedBoreOptions{})
		if e != nil || endpoint.Complete || endpoint.Nested != nil || endpoint.Bore != nil || endpoint.RefinementChecks != 2 {
			t.Fatal("retained endpoint not interior", e)
		}
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	no, e = q.ClassifyNestedTorusBoreFacesRational(cc, tieP, FilletedFrustumNestedBoreOptions{})
	if e == nil || no.Lower != nil {
		t.Fatal("atomic cancellation")
	}
}
