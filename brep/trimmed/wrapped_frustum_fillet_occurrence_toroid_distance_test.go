package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumWorldToroidDistance(t *testing.T) {
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
	// Product of independent3/5–4/5 and5/13–12/13 rotations: all three
	// world coordinates participate, with genuinely nonbinary translation.
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
	local := [3]*big.Rat{r(77, 16), r(77, 12), r(3, 16)}
	p := world(local)
	opts := FilletedFrustumToroidDistanceOptions{Rims: FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}}
	source, e := q.BoundBoundaryToroidDistanceRational(ctx, local, opts)
	if e != nil {
		t.Fatal(e)
	}
	got, e := placed.BoundBoundaryToroidDistanceRational(ctx, p, opts)
	if e != nil {
		t.Fatal(e)
	}
	if got.WorldToroid == nil || got.Fallback.Lower != nil || !got.NearestProven || !got.Converged || got.Lower.Cmp(source.Lower) != 0 || got.Upper.Cmp(source.Upper) != 0 || got.WorldToroid.Face != q.fillets.Blends[0] {
		t.Fatal(got)
	}
	wt := got.WorldToroid
	dot := func(a, b [3]*big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := range a {
			v.Add(v, fdMul(a[i], b[i]))
		}
		return v
	}
	var delta [3]*big.Rat
	for i := range p {
		delta[i] = fdSub(p[i], wt.Base[i])
	}
	// Independent world-space radical distance A+B/sqrt(D), retaining the
	// analytic exact polynomial for3577/2304-5sqrt(97)/32.
	a := fdAdd(dot(delta, delta), new(big.Rat).Quo(dot(wt.Numerator, wt.Numerator), wt.Radicand))
	b := fdMul(r(-2, 1), dot(delta, wt.Numerator))
	if a.Cmp(r(3577, 2304)) != 0 || b.Cmp(r(-485, 384)) != 0 || new(big.Rat).Quo(fdSquare(b), wt.Radicand).Cmp(r(2425, 1024)) != 0 {
		t.Fatal(a, b, wt.Radicand)
	}
	// Reversing the exact pose recovers every original witness coefficient.
	back, e := pose.TransformPoint(ctx, wt.Base, true, 4096)
	if e != nil {
		t.Fatal(e)
	}
	vector, e := pose.TransformVector(ctx, wt.Numerator, true, 4096)
	if e != nil {
		t.Fatal(e)
	}
	for i := range back {
		if back[i].Cmp(source.Toroid.Center[i]) != 0 || vector[i].Cmp(source.Toroid.Numerator[i]) != 0 {
			t.Fatal(back, vector)
		}
	}
	if wt.Radicand.Cmp(r(97, 144)) != 0 || fdSquare(wt.Root[0]).Cmp(wt.Radicand) > 0 || fdSquare(wt.Root[1]).Cmp(wt.Radicand) < 0 {
		t.Fatal(wt)
	}
	// Both previous fallback types are propagated from the same source query.
	floor, e := placed.BoundBoundaryToroidDistanceRational(ctx, world([3]*big.Rat{r(2, 1), r(0, 1), r(9, 8)}), opts)
	if e != nil || floor.WorldToroid != nil || floor.Fallback.WorldRationalWitness[0] == nil || !floor.NearestProven {
		t.Fatal(floor, e)
	}
	rim, e := placed.BoundBoundaryToroidDistanceRational(ctx, world([3]*big.Rat{r(-19, 10), r(1, 10), r(-1, 8)}), opts)
	if e != nil || rim.WorldToroid != nil || rim.Fallback.WorldRim == nil || !rim.NearestProven {
		t.Fatal(rim, e)
	}
	ambiguous, e := placed.BoundBoundaryToroidDistanceRational(ctx, world([3]*big.Rat{r(1, 1), r(1, 1), r(3, 1)}), opts)
	if e != nil || ambiguous.NearestProven || ambiguous.WorldToroid != nil {
		t.Fatal(ambiguous, e)
	}
	wt.Numerator[0].SetInt64(100)
	again, e := placed.BoundBoundaryToroidDistanceRational(ctx, p, opts)
	if e != nil || again.WorldToroid.Numerator[0].Cmp(r(100, 1)) == 0 {
		t.Fatal("ownership", again, e)
	}
	limited := opts
	limited.MaxToroidChecks = 1
	if v, e := placed.BoundBoundaryToroidDistanceRational(ctx, p, limited); e == nil || v.Lower != nil || v.WorldToroid != nil || v.Original.Lower != nil {
		t.Fatal("atomic budget", v, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := placed.BoundBoundaryToroidDistanceRational(cancelled, p, opts); e != context.Canceled || v.Lower != nil {
		t.Fatal(v, e)
	}
}
