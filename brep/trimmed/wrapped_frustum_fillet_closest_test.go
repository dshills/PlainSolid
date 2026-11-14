package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumClosestDispatcher(t *testing.T) {
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
	var encoded bytes.Buffer
	if e = WriteFilletedFrustumMaterialJSON(ctx, &encoded, q); e != nil {
		t.Fatal(e)
	}
	q, e = ReadFilletedFrustumMaterialJSON(ctx, &encoded, FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m := [3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	placed, e := q.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	world := func(p [3]*big.Rat) [3]*big.Rat {
		var v [3]*big.Rat
		for i := range v {
			v[i] = new(big.Rat).Set(m[i][3])
			for j := range p {
				v[i].Add(v[i], fdMul(m[i][j], p[j]))
			}
		}
		return v
	}
	opts := FilletedFrustumNestedDistanceOptions{Original: FilletedFrustumToroidDistanceOptions{Rims: FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}}}

	rho := fdAdd(q.fillets.Circles[0].Center[0], r(1, 3))
	cases := []struct {
		p       [3]*big.Rat
		kind    FilletedFrustumClosestKind
		nearest bool
	}{
		{[3]*big.Rat{r(0, 1), r(0, 1), r(-1, 1)}, FilletedFrustumClosestRational, true},
		{[3]*big.Rat{r(-19, 10), r(1, 10), r(-1, 8)}, FilletedFrustumClosestRim, true},
		{[3]*big.Rat{fdMul(rho, r(3, 5)), fdMul(rho, r(4, 5)), r(3, 16)}, FilletedFrustumClosestToroid, true},
		{[3]*big.Rat{r(8, 1), r(1, 1), r(3, 16)}, FilletedFrustumClosestNestedToroid, true},
		{[3]*big.Rat{r(10, 1), r(1, 1), r(3, 2)}, FilletedFrustumClosestCone, true},
		{[3]*big.Rat{r(-19, 10), r(1, 10), r(3, 1)}, FilletedFrustumClosestBore, true},
	}
	for _, tc := range cases {
		got, e := q.ClosestBoundaryRational(ctx, tc.p, opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.Kind != tc.kind || got.NearestProven != tc.nearest || len(got.Faces) == 0 {
			t.Fatalf("kind=%v expected=%v nearest=%v", got.Kind, tc.kind, got.NearestProven)
		}
		direct, e := q.BoundBoundaryBoreDistanceRational(ctx, tc.p, FilletedFrustumBoreDistanceOptions{Original: opts})
		if e != nil {
			t.Fatal(e)
		}
		if got.Lower.Cmp(direct.Lower) != 0 || got.Upper.Cmp(direct.Upper) != 0 || got.Converged != direct.Converged {
			t.Fatal("changed proof")
		}
		// Fixed counts confirm one bounded chain, with no repeated support search.
		if got.Proof.Baseline.NestedChecks != 2 || got.Proof.Baseline.Baseline.ToroidChecks != 2 || got.Proof.Baseline.Baseline.Baseline.RimChecks != 10 {
			t.Fatal("work ledger")
		}
		placedResult, e := placed.ClosestBoundaryRational(ctx, world(tc.p), opts)
		if e != nil {
			t.Fatal(e)
		}
		if placedResult.Kind != got.Kind || placedResult.NearestProven != got.NearestProven || placedResult.Lower.Cmp(got.Lower) != 0 || placedResult.Upper.Cmp(got.Upper) != 0 {
			t.Fatal("world selection")
		}
		if !tc.nearest {
			loose := opts
			loose.Original.Rims.Support.SquaredTolerance = r(1000000, 1)
			v, e := q.ClosestBoundaryRational(ctx, tc.p, loose)
			if e != nil || !v.Converged || v.NearestProven {
				t.Fatal("tolerance promoted nearest", e)
			}
		}
		got.Faces[0] = -999
		got.Lower.SetInt64(-1)
		if got.Proof.Lower.Sign() < 0 {
			t.Fatal("scalar alias")
		}
		fresh, e := q.ClosestBoundaryRational(ctx, tc.p, opts)
		if e != nil || fresh.Faces[0] < 0 {
			t.Fatal("source ownership", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := q.ClosestBoundaryRational(cancelled, cases[0].p, opts)
	if e == nil || bad.Kind != FilletedFrustumClosestNone || bad.Lower != nil {
		t.Fatal("atomic cancelled selection")
	}
	opts.Original.Rims.MaxRimChecks = 1
	bad, e = q.ClosestBoundaryRational(ctx, cases[0].p, opts)
	if e == nil || bad.Kind != FilletedFrustumClosestNone || bad.Lower != nil {
		t.Fatal("atomic budget selection")
	}
}
