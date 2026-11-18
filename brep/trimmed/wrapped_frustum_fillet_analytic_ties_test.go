package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestFilletedFrustumAnalyticNearestFaces(t *testing.T) {
	slowtest.Skip(t)
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
		p    [3]*big.Rat
		face int
		kind FilletedFrustumClosestKind
	}{
		{[3]*big.Rat{r(10, 1), r(1, 1), r(3, 2)}, q.fillets.Lateral, FilletedFrustumClosestCone},
		{[3]*big.Rat{r(77, 16), r(77, 12), r(3, 16)}, q.fillets.Blends[0], FilletedFrustumClosestToroid},
		{[3]*big.Rat{r(8, 1), r(1, 1), r(3, 16)}, q.fillets.Blends[0], FilletedFrustumClosestNestedToroid},
		{[3]*big.Rat{r(-19, 10), r(1, 10), r(3, 1)}, q.cuts[0].wall, FilletedFrustumClosestBore},
	} {
		got, e := q.ClassifyAnalyticNearestFacesRational(ctx, tc.p, opts)
		if e != nil || !got.Complete || len(got.Tied) != 1 || got.Tied[0] != tc.face || len(got.Unresolved) != 0 || got.Proof.Kind != tc.kind {
			t.Fatal(got, e)
		}
		if got.Proof.Lower.Cmp(got.Proof.Upper) >= 0 {
			t.Fatal("fixture must retain irrational interval")
		}
		seen := map[int]bool{tc.face: true}
		for _, id := range got.Excluded {
			if seen[id] {
				t.Fatal("duplicate owner")
			}
			seen[id] = true
			found := false
			for _, f := range got.Proof.Proof.Baseline.Baseline.Baseline.Baseline.PerFace {
				if f.Face == id {
					found = f.SquaredLower.Cmp(got.Proof.Upper) > 0
				}
			}
			if !found {
				t.Fatal("unseparated exclusion")
			}
		}
		if len(seen) != len(q.solid.faces) {
			t.Fatal("missing original owners")
		}
		direct, e := q.ClosestBoundaryRational(ctx, tc.p, opts)
		if e != nil || direct.Lower.Cmp(got.Proof.Lower) != 0 || direct.Upper.Cmp(got.Proof.Upper) != 0 || direct.BoreProof.BoreChecks != got.Proof.BoreProof.BoreChecks {
			t.Fatal("repeated/changed proof")
		}
	}
	for _, p := range [][3]*big.Rat{{r(2, 1), r(0, 1), r(3, 2)}} {
		got, e := q.ClassifyAnalyticNearestFacesRational(ctx, p, opts)
		if e != nil || got.Complete || len(got.Tied) != 0 || len(got.Unresolved) != len(q.solid.faces) {
			t.Fatal("unsupported tie promoted", e)
		}
	}
	badOpts := opts
	badOpts.Original.Rims.Support.MaxSupportChecks = 1
	bad, e := q.ClassifyAnalyticNearestFacesRational(ctx, [3]*big.Rat{r(10, 1), r(1, 1), r(3, 2)}, badOpts)
	if e == nil || bad.Proof.Lower != nil || len(bad.Tied) != 0 {
		t.Fatal("atomic budget")
	}
}
