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

func TestFilletedFrustumWorldConeDistance(t *testing.T) {
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

	for _, xy := range [][2]int64{{10, 1}, {9, 4}} {
		local := [3]*big.Rat{r(xy[0], 1), r(xy[1], 1), r(3, 2)}
		p := world(local)
		original, e := q.BoundBoundaryConeDistanceRational(ctx, local, opts)
		if e != nil {
			t.Fatal(e)
		}
		got, e := placed.BoundBoundaryConeDistanceRational(ctx, p, opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.WorldCone == nil || got.Fallback.Lower != nil || !got.NearestProven || !got.Converged || got.Lower.Cmp(original.Lower) != 0 || got.Upper.Cmp(original.Upper) != 0 || got.WorldCone.Face != q.fillets.Lateral {
			t.Fatal(got)
		}
		n := got.WorldCone
		var combined [3]*big.Rat
		for i := range combined {
			combined[i] = fdAdd(n.RadialVector[i], fdMul(n.Axis[i], n.AxialCoefficient))
		}
		// Independent transpose inverse recovers both coefficients of the exact
		// algebraic point, including a nonbinary translated base.
		for j := 0; j < 3; j++ {
			b, v := new(big.Rat), new(big.Rat)
			for i := 0; i < 3; i++ {
				b.Add(b, fdMul(m[i][j], fdSub(n.Base[i], m[i][3])))
				v.Add(v, fdMul(m[i][j], combined[i]))
			}
			if b.Cmp(original.Cone.Base[j]) != 0 || v.Cmp(original.Cone.RadicalVector[j]) != 0 {
				t.Fatal("inverse world coefficients")
			}
		}
		dot := func(a, b [3]*big.Rat) *big.Rat {
			v := new(big.Rat)
			for i := range a {
				v.Add(v, fdMul(a[i], b[i]))
			}
			return v
		}
		if dot(n.Axis, n.Axis).Cmp(r(1, 1)) != 0 || dot(n.Axis, n.RadialVector).Sign() != 0 {
			t.Fatal("axis/radial metric")
		}
		var d [3]*big.Rat
		for i := range d {
			d[i] = fdSub(p[i], n.Base[i])
		}
		// World squared distance has independently known coefficients
		// 9(S+100)/25 - 36 sqrt(S)/5.
		a := fdAdd(dot(d, d), fdMul(n.Radicand, dot(combined, combined)))
		b := fdMul(r(-2, 1), dot(d, combined))
		if a.Cmp(fdMul(r(9, 25), fdAdd(n.Radicand, r(100, 1)))) != 0 || b.Cmp(r(-36, 5)) != 0 {
			t.Fatal("world radical distance", a, b)
		}
		n.Base[0].SetInt64(999)
		n.RadialVector[0].SetInt64(999)
		n.Root[0].SetInt64(999)
		again, e := placed.BoundBoundaryConeDistanceRational(ctx, p, opts)
		if e != nil || again.WorldCone == nil || again.WorldCone.Root[0].Cmp(r(999, 1)) == 0 {
			t.Fatal("mutable result", e)
		}
	}
	// A toroidal candidate stays a world nested fallback without rerunning it.
	p := world([3]*big.Rat{r(8, 1), r(1, 1), r(3, 16)})
	fallback, e := placed.BoundBoundaryConeDistanceRational(ctx, p, opts)
	if e != nil || fallback.WorldCone != nil || fallback.Fallback.WorldNested == nil {
		t.Fatal("nested fallback", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	failed, e := placed.BoundBoundaryConeDistanceRational(cancelled, p, opts)
	if e == nil || failed.Lower != nil || failed.WorldCone != nil {
		t.Fatal("atomic cancellation")
	}
	opts.MaxNestedChecks = 1
	failed, e = placed.BoundBoundaryConeDistanceRational(ctx, p, opts)
	if e == nil || failed.Lower != nil || failed.WorldCone != nil {
		t.Fatal("atomic work limit")
	}
}
