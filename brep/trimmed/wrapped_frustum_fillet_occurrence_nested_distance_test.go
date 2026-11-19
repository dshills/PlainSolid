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

func TestFilletedFrustumWorldNestedDistance(t *testing.T) {
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
	local := [3]*big.Rat{r(8, 1), r(1, 1), r(3, 16)}
	p := world(local)
	original, e := q.BoundBoundaryNestedDistanceRational(ctx, local, opts)
	if e != nil {
		t.Fatal(e)
	}
	got, e := placed.BoundBoundaryNestedDistanceRational(ctx, p, opts)
	if e != nil {
		t.Fatal(e)
	}
	if got.WorldNested == nil || got.Fallback.Lower != nil || !got.NearestProven || !got.Converged || got.Lower.Cmp(original.Lower) != 0 || got.Upper.Cmp(original.Upper) != 0 || got.WorldNested.Face != q.fillets.Blends[0] {
		t.Fatal(got)
	}
	n := got.WorldNested
	for i, v := range [3]*big.Rat{r(157, 364), r(55, 117), r(147, 176)} {
		if n.Base[i].Cmp(v) != 0 {
			t.Fatal("world base", n.Base)
		}
	}
	for i, v := range [3]*big.Rat{r(12, 13), r(313, 65), r(-32, 5)} {
		if n.RadialVector[i].Cmp(v) != 0 {
			t.Fatal("world radial", n.RadialVector)
		}
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		v := new(big.Rat)
		for i := range a {
			v.Add(v, fdMul(a[i], b[i]))
		}
		return v
	}
	if dot(n.Axis, n.Axis).Cmp(r(1, 1)) != 0 || dot(n.Axis, n.RadialVector).Sign() != 0 || dot(n.RadialVector, n.RadialVector).Cmp(r(65, 1)) != 0 {
		t.Fatal("world metric")
	}
	var delta [3]*big.Rat
	for i := range p {
		delta[i] = fdSub(p[i], n.Base[i])
		if delta[i].Cmp(fdAdd(n.RadialVector[i], fdMul(n.Axis[i], n.HeightOffset))) != 0 {
			t.Fatal("world query formula")
		}
	}
	// Derive the nested quartic solely from world metric and native meridian
	// radii: D=A-B*sqrt(S), t=(sqrt(D)-r)^2.
	S := dot(n.RadialVector, n.RadialVector)
	A := fdAdd(dot(delta, delta), fdSquare(n.MajorRadius))
	B := fdMul(r(2, 1), n.MajorRadius)
	rr := fdSquare(n.MinorRadius)
	if S.Cmp(r(65, 1)) != 0 || A.Cmp(r(31913, 256)) != 0 || B.Cmp(r(123, 8)) != 0 || rr.Cmp(r(225, 256)) != 0 {
		t.Fatal(S, A, B, rr)
	}
	poly := func(t *big.Rat) *big.Rat {
		sum := fdAdd(rr, t)
		C := fdAdd(fdAdd(fdSquare(A), fdMul(fdSquare(B), S)), fdSquare(fdSub(rr, t)))
		C.Sub(C, fdMul(r(2, 1), fdMul(A, sum)))
		E := fdMul(fdMul(r(2, 1), B), fdSub(sum, A))
		return fdSub(fdSquare(C), fdMul(S, fdSquare(E)))
	}
	if poly(got.Lower).Sign()*poly(got.Upper).Sign() > 0 {
		t.Fatal("independent world quartic")
	}
	back, e := pose.TransformPoint(ctx, n.Base, true, 4096)
	if e != nil {
		t.Fatal(e)
	}
	radial, e := pose.TransformVector(ctx, n.RadialVector, true, 4096)
	if e != nil {
		t.Fatal(e)
	}
	axis, e := pose.TransformVector(ctx, n.Axis, true, 4096)
	if e != nil {
		t.Fatal(e)
	}
	for i := range back {
		if back[i].Cmp(original.Nested.Center[i]) != 0 {
			t.Fatal("inverse base")
		}
	}
	if radial[0].Cmp(r(8, 1)) != 0 || radial[1].Cmp(r(1, 1)) != 0 || radial[2].Sign() != 0 || axis[0].Sign() != 0 || axis[1].Sign() != 0 || axis[2].Cmp(r(1, 1)) != 0 {
		t.Fatal("inverse formula", radial, axis)
	}
	// Reuse each prior witness branch without additional source queries.
	torus, e := placed.BoundBoundaryNestedDistanceRational(ctx, world([3]*big.Rat{r(77, 16), r(77, 12), r(3, 16)}), opts)
	if e != nil || torus.WorldNested != nil || torus.Fallback.WorldToroid == nil || !torus.NearestProven {
		t.Fatal(torus, e)
	}
	rim, e := placed.BoundBoundaryNestedDistanceRational(ctx, world([3]*big.Rat{r(-19, 10), r(1, 10), r(-1, 8)}), opts)
	if e != nil || rim.WorldNested != nil || rim.Fallback.Fallback.WorldRim == nil || !rim.NearestProven {
		t.Fatal(rim, e)
	}
	floor, e := placed.BoundBoundaryNestedDistanceRational(ctx, world([3]*big.Rat{r(2, 1), r(0, 1), r(9, 8)}), opts)
	if e != nil || floor.WorldNested != nil || floor.Fallback.Fallback.WorldRationalWitness[0] == nil || !floor.NearestProven {
		t.Fatal(floor, e)
	}
	ambiguous, e := placed.BoundBoundaryNestedDistanceRational(ctx, world([3]*big.Rat{r(1, 1), r(1, 1), r(3, 1)}), opts)
	if e != nil || ambiguous.WorldNested != nil || ambiguous.NearestProven {
		t.Fatal("false promotion", ambiguous, e)
	}
	n.RadialVector[0].SetInt64(100)
	n.AzimuthRoot[0].SetInt64(100)
	again, e := placed.BoundBoundaryNestedDistanceRational(ctx, p, opts)
	if e != nil || again.WorldNested.RadialVector[0].Cmp(r(12, 13)) != 0 {
		t.Fatal("ownership", again, e)
	}
	limited := opts
	limited.MaxNestedChecks = 1
	if v, e := placed.BoundBoundaryNestedDistanceRational(ctx, p, limited); e == nil || v.Lower != nil || v.WorldNested != nil || v.Original.Lower != nil {
		t.Fatal("atomic budget", v, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := placed.BoundBoundaryNestedDistanceRational(cancelled, p, opts); e != context.Canceled || v.Lower != nil {
		t.Fatal(v, e)
	}
}
