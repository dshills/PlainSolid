package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumTwoOwnerIrrationalRims(t *testing.T) {
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

	opts := FilletedFrustumNestedDistanceOptions{Original: FilletedFrustumToroidDistanceOptions{Rims: FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}}}
	for _, tc := range []struct {
		p     [3]*big.Rat
		faces [2]int
		A, S  *big.Rat
	}{
		{[3]*big.Rat{r(-19, 10), r(1, 10), r(-1, 8)}, [2]int{q.cuts[0].wall, q.fillets.Caps[0]}, r(457, 1600), r(1, 50)},
		{[3]*big.Rat{r(-19, 10), r(1, 10), r(49, 8)}, [2]int{q.cuts[0].wall, q.fillets.Caps[1]}, r(457, 1600), r(1, 50)},
		{[3]*big.Rat{r(5, 2), r(1, 2), r(9, 8)}, [2]int{q.cuts[1].wall, q.cuts[1].floor}, r(49, 64), r(1, 2)},
		{[3]*big.Rat{r(5, 2), r(1, 2), r(15, 8)}, [2]int{q.cuts[2].wall, q.cuts[2].floor}, r(49, 64), r(1, 2)},
	} {
		got, e := q.ClassifyAnalyticNearestFacesRational(ctx, tc.p, opts)
		if e != nil || !got.Complete || len(got.Tied) != 2 || len(got.Unresolved) != 0 || got.Proof.Kind != FilletedFrustumClosestRim {
			t.Fatal(got, e)
		}
		owns := map[int]bool{}
		for _, f := range got.Tied {
			owns[f] = true
		}
		if !owns[tc.faces[0]] || !owns[tc.faces[1]] {
			t.Fatal("paired owners")
		}
		rim := got.Proof.Proof.Baseline.Baseline.Baseline.Rim
		if len(rim.RimEdges) != 4 || rim.AzimuthSquared.Cmp(tc.S) != 0 {
			t.Fatal("exact circle identity")
		}
		// Independently D=A-sqrt(S); both adjacent native supports attain it.
		poly := func(d *big.Rat) *big.Rat { return fdSub(fdSquare(fdSub(tc.A, d)), tc.S) }
		if got.Proof.Upper.Cmp(tc.A) >= 0 || poly(got.Proof.Lower).Sign() < 0 || poly(got.Proof.Upper).Sign() > 0 {
			t.Fatal("distance identity")
		}
		if len(got.Excluded) != len(q.solid.faces)-2 {
			t.Fatal("missing original owner")
		}
		for _, id := range got.Excluded {
			ok := false
			for _, f := range got.Proof.Proof.Baseline.Baseline.Baseline.Baseline.PerFace {
				if f.Face == id {
					ok = f.SquaredLower.Cmp(got.Proof.Upper) > 0
				}
			}
			if !ok {
				t.Fatal("not strictly excluded")
			}
		}
	}
	// Two opposite blind floors separated by1/4 produce FOUR attained owners
	// at the midpoint. The two-owner report must not discard the other rim.
	near, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 19. / 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	ambiguous, e := near.ClassifyAnalyticNearestFacesRational(ctx, [3]*big.Rat{r(5, 2), r(1, 2), r(9, 8)}, opts)
	if e != nil || ambiguous.Complete || len(ambiguous.Tied) != 0 || len(ambiguous.Unresolved) != len(near.solid.faces) {
		t.Fatal("unseparated extra owners", ambiguous, e)
	}
	if ambiguous.Proof.Kind != FilletedFrustumClosestRim || ambiguous.Proof.NearestProven {
		t.Fatal("false pair-only proof")
	}
}
