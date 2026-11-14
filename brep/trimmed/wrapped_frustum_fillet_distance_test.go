package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
	"sort"
	"testing"
)

func TestFilletedFrustumAllSupportDistance(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	r := big.NewRat
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
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	check := func(p [3]*big.Rat, want *big.Rat, owners []int) {
		t.Helper()
		got, e := q.BoundBoundaryDistanceRational(ctx, p, FilletedFrustumDistanceOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if !got.Exact || got.Lower.Cmp(want) != 0 || got.Upper.Cmp(want) != 0 || !reflect.DeepEqual(got.WitnessFaces, owners) {
			t.Fatalf("distance %+v want %v owners %v", got, want, owners)
		}
		v, e := q.ClassifyRational(ctx, got.Witness)
		if e != nil || v.Location != geom.BoreBoundary || !reflect.DeepEqual(v.Faces, got.WitnessFaces) {
			t.Fatal(v, e)
		}
		if got.SupportChecks != 10 || len(got.PerFace) != 10 {
			t.Fatal(got)
		}
	}
	check([3]*big.Rat{r(2, 1), r(0, 1), r(9, 8)}, r(1, 64), []int{q.cuts[1].floor})
	check([3]*big.Rat{r(0, 1), r(0, 1), r(-1, 8)}, r(1, 64), []int{q.fillets.Caps[0]})
	// Rational normal ray of the lower rolling circle: nearest original torus
	// point has meridian normal(5/13,-12/13), strictly inside its retained arc.
	for _, sign := range []int64{-1, 1} {
		c := q.fillets.Circles[0]
		radius := fdAdd(c.Radius, r(sign, 64))
		p := [3]*big.Rat{fdAdd(c.Center[0], fdMul(radius, r(5, 13))), r(0, 1), fdAdd(c.Center[1], fdMul(radius, r(-12, 13)))}
		check(p, r(1, 4096), []int{q.fillets.Blends[0]})
	}
	check([3]*big.Rat{r(2, 1), r(0, 1), r(15, 8)}, r(1, 64), []int{q.cuts[2].floor})
	check([3]*big.Rat{r(-7, 5), r(0, 1), r(3, 1)}, r(1, 100), []int{q.cuts[0].wall})
	check([3]*big.Rat{fdAdd(r(10, 1), r(3, 320)), r(0, 1), fdSub(r(3, 2), r(1, 80))}, r(1, 4096), []int{q.fillets.Lateral})
	c := q.fillets.Circles[1]
	rr := fdAdd(c.Radius, r(1, 64))
	check([3]*big.Rat{fdAdd(c.Center[0], fdMul(rr, r(4, 5))), r(0, 1), fdAdd(c.Center[1], fdMul(rr, r(3, 5)))}, r(1, 4096), []int{q.fillets.Blends[1]})
	// Same-owner hole clearance removes the old zero lower bound from the
	// discarded full-plane projection. The retained rim owns cap and bore.
	check([3]*big.Rat{r(-2, 1), r(0, 1), r(0, 1)}, r(1, 4), []int{q.fillets.Caps[0], q.cuts[0].wall})
	check([3]*big.Rat{r(-2, 1), r(0, 1), r(-1, 8)}, r(17, 64), []int{q.fillets.Caps[0], q.cuts[0].wall})
	// Disk clipping also excludes the unrelated blind floor beside this cone.
	check([3]*big.Rat{fdAdd(r(28, 3), r(3, 320)), r(0, 1), fdSub(r(1, 1), r(1, 80))}, r(1, 4096), []int{q.fillets.Lateral})
	for end, c := range q.fillets.Circles {
		p := [3]*big.Rat{fdAdd(c.Tangency[0], fdMul(r(1, 64), c.Normal[0])), r(0, 1), fdAdd(c.Tangency[1], fdMul(r(1, 64), c.Normal[1]))}
		owners, e := q.ClassifyRational(ctx, [3]*big.Rat{c.Tangency[0], r(0, 1), c.Tangency[1]})
		if e != nil || len(owners.Faces) != 2 {
			t.Fatal(owners, e)
		}
		expected := []int{q.fillets.Lateral, q.fillets.Blends[end]}
		sort.Ints(expected)
		check(p, r(1, 4096), expected)
	}
	var p [3]*big.Rat
	// Non-Pythagorean azimuth: increasingly precise sqrt enclosures retain
	// validity and cannot lower the all-support lower bound.
	p = [3]*big.Rat{r(1, 1), r(1, 1), r(3, 1)}
	coarse, e := q.BoundBoundaryDistanceRational(ctx, p, FilletedFrustumDistanceOptions{SqrtBits: 8})
	if e != nil {
		t.Fatal(e)
	}
	fine, e := q.BoundBoundaryDistanceRational(ctx, p, FilletedFrustumDistanceOptions{SqrtBits: 128})
	if e != nil {
		t.Fatal(e)
	}
	if fine.Lower.Cmp(coarse.Lower) < 0 || fine.Upper.Cmp(coarse.Upper) != 0 || fine.Lower.Sign() < 0 {
		t.Fatal(coarse, fine)
	}
	// 1/sqrt(radial) intervals do not fabricate rational contact witnesses.
	if fine.Exact {
		t.Fatal("unexpected trim resolution")
	}
	// A lower blind hole must never exclude material from the intact upper
	// cap. This fails if hole clearance is applied without exact cap ownership.
	one, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}}, o)
	if e != nil {
		t.Fatal(e)
	}
	intact, e := one.BoundBoundaryDistanceRational(ctx, [3]*big.Rat{r(2, 1), r(0, 1), r(49, 8)}, FilletedFrustumDistanceOptions{})
	if e != nil || !intact.Exact || intact.Lower.Cmp(r(1, 64)) != 0 || !reflect.DeepEqual(intact.WitnessFaces, []int{one.fillets.Caps[1]}) {
		t.Fatal(intact, e)
	}
	for _, opts := range []FilletedFrustumDistanceOptions{{MaxSupportChecks: 9}, {SqrtBits: 257}, {SquaredTolerance: r(-1, 1)}} {
		v, e := q.BoundBoundaryDistanceRational(ctx, p, opts)
		if e == nil || v.Lower != nil || v.Upper != nil || v.PerFace != nil {
			t.Fatal("atomic error", v, e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	v, e := q.BoundBoundaryDistanceRational(cancelled, p, FilletedFrustumDistanceOptions{})
	if e != context.Canceled || v.Upper != nil {
		t.Fatal(v, e)
	}
	p[0] = new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 4097))
	if v, e = q.BoundBoundaryDistanceRational(ctx, p, FilletedFrustumDistanceOptions{}); e == nil || v.Upper != nil {
		t.Fatal(v, e)
	}
}

func TestFilletedDistanceSquareRootEnclosures(t *testing.T) {
	for _, x := range []*big.Rat{big.NewRat(0, 1), big.NewRat(25, 169), big.NewRat(2, 3), big.NewRat(700, 13)} {
		lo, hi := fdSqrt(x, 8)
		if lo.Sign() < 0 || fdSquare(lo).Cmp(x) > 0 || fdSquare(hi).Cmp(x) < 0 {
			t.Fatal(x, lo, hi)
		}
		a, b := fdSqrt(x, 64)
		if a.Cmp(lo) < 0 || b.Cmp(hi) > 0 {
			t.Fatal("nested", x)
		}
	}
}
