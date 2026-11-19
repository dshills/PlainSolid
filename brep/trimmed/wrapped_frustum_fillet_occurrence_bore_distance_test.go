//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestFilletedFrustumWorldBoreDispatcher(t *testing.T) {
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

	local := [3]*big.Rat{r(-19, 10), r(1, 10), r(3, 1)}
	p := world(local)
	original, e := q.ClosestBoundaryRational(ctx, local, opts)
	if e != nil {
		t.Fatal(e)
	}
	got, e := placed.ClosestBoundaryRational(ctx, p, opts)
	if e != nil {
		t.Fatal(e)
	}
	if got.Kind != FilletedFrustumClosestBore || !got.NearestProven || !got.Converged || got.BoreProof == nil || got.BoreProof.WorldBore == nil || got.Lower.Cmp(original.Lower) != 0 || got.Upper.Cmp(original.Upper) != 0 {
		t.Fatal(got)
	}
	witness := got.BoreProof.WorldBore
	if witness.Face != q.cuts[0].wall || got.Faces[0] != witness.Face || witness.Radicand.Cmp(r(1, 50)) != 0 {
		t.Fatal("owner/radical")
	}
	// Exact independent inverse of the world point's rational coefficients.
	for j := 0; j < 3; j++ {
		c, v := new(big.Rat), new(big.Rat)
		for i := 0; i < 3; i++ {
			c.Add(c, fdMul(m[i][j], fdSub(witness.Center[i], m[i][3])))
			v.Add(v, fdMul(m[i][j], witness.RadialNumerator[i]))
		}
		wantC := [3]*big.Rat{r(-2, 1), r(0, 1), r(3, 1)}
		wantV := [3]*big.Rat{r(1, 20), r(1, 20), r(0, 1)}
		if c.Cmp(wantC[j]) != 0 || v.Cmp(wantV[j]) != 0 {
			t.Fatal("inverse coefficients")
		}
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := range a {
			v.Add(v, fdMul(a[i], b[i]))
		}
		return v
	}
	if dot(witness.Axis, witness.Axis).Cmp(r(1, 1)) != 0 || dot(witness.Axis, witness.RadialNumerator).Sign() != 0 || dot(witness.RadialNumerator, witness.RadialNumerator).Cmp(r(1, 200)) != 0 {
		t.Fatal("cylindrical metric")
	}
	var d [3]*big.Rat
	for i := range d {
		d[i] = fdSub(p[i], witness.Center[i])
	}
	// Exact world squared distance =27/100-sqrt(1/50).
	if fdAdd(dot(d, d), new(big.Rat).Quo(dot(witness.RadialNumerator, witness.RadialNumerator), witness.Radicand)).Cmp(r(27, 100)) != 0 || new(big.Rat).Quo(fdMul(r(-2, 1), dot(d, witness.RadialNumerator)), witness.Radicand).Cmp(r(-1, 1)) != 0 {
		t.Fatal("world distance expression")
	}
	// The old explicit cone chain remains conservative for this unsupported
	// cylindrical candidate. The common dispatcher alone adds its new proof.
	legacy, e := placed.BoundBoundaryConeDistanceRational(ctx, p, opts)
	if e != nil || legacy.NearestProven {
		t.Fatal("legacy semantic change", e)
	}
	floor, e := placed.ClosestBoundaryRational(ctx, world([3]*big.Rat{r(21, 10), r(1, 10), r(99, 100)}), opts)
	if e != nil || floor.Kind != FilletedFrustumClosestRational || floor.BoreProof.WorldBore != nil || !floor.NearestProven || floor.Lower.Cmp(r(1, 10000)) != 0 || floor.Upper.Cmp(r(1, 10000)) != 0 {
		t.Fatal("closer world floor", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := placed.ClosestBoundaryRational(cancelled, p, opts)
	if e == nil || bad.Kind != FilletedFrustumClosestNone || bad.Lower != nil {
		t.Fatal("atomic cancellation")
	}
	failed, e := placed.BoundBoundaryBoreDistanceRational(ctx, p, FilletedFrustumBoreDistanceOptions{Original: opts, MaxBoreChecks: 1})
	if e == nil || failed.Lower != nil || failed.WorldBore != nil {
		t.Fatal("atomic bore budget")
	}
}
