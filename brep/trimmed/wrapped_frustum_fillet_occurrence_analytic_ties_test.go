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

func TestFilletedFrustumWorldAnalyticNearestFaces(t *testing.T) {
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
	for _, local := range [][3]*big.Rat{{r(10, 1), r(1, 1), r(3, 2)}, {r(77, 16), r(77, 12), r(3, 16)}, {r(8, 1), r(1, 1), r(3, 16)}, {r(-19, 10), r(1, 10), r(3, 1)}, {r(-19, 10), r(1, 10), r(-1, 8)}, {r(2, 1), r(0, 1), r(3, 2)}} {
		original, e := q.ClassifyAnalyticNearestFacesRational(ctx, local, opts)
		if e != nil {
			t.Fatal(e)
		}
		got, e := placed.ClassifyAnalyticNearestFacesRational(ctx, world(local), opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.Complete != original.Complete || got.Kind != original.Proof.Kind || len(got.Tied) != len(original.Tied) || len(got.Excluded) != len(original.Excluded) || len(got.Unresolved) != len(original.Unresolved) || got.WorldProof.Lower.Cmp(original.Proof.Lower) != 0 || got.WorldProof.Upper.Cmp(original.Proof.Upper) != 0 {
			t.Fatal("changed classification")
		}
		for i, f := range got.Tied {
			if f != original.Tied[i] {
				t.Fatal("tied ID")
			}
		}
		for i, f := range got.Excluded {
			if f != original.Excluded[i] {
				t.Fatal("excluded ID")
			}
		}
		for i, f := range got.Unresolved {
			if f != original.Unresolved[i] {
				t.Fatal("unresolved ID")
			}
		}
		// Independent coefficient transform of the selected exact symbolic point.
		vector := func(v [3]*big.Rat) [3]*big.Rat {
			var a [3]*big.Rat
			for i := range a {
				a[i] = new(big.Rat)
				for j := range v {
					a[i].Add(a[i], fdMul(m[i][j], v[j]))
				}
			}
			return a
		}
		check := func(got, want [3]*big.Rat) {
			for i := range got {
				if got[i].Cmp(want[i]) != 0 {
					t.Fatal("world witness coefficients")
				}
			}
		}
		switch got.Kind {
		case FilletedFrustumClosestCone:
			a := original.Proof.Proof.Cone
			b := got.WorldProof.Fallback.WorldCone
			if b == nil || b.Face != a.Face || b.Radicand.Cmp(a.Radicand) != 0 {
				t.Fatal("cone identity")
			}
			check(b.Base, world(a.Base))
			var v [3]*big.Rat
			for i := range v {
				v[i] = fdAdd(b.RadialVector[i], fdMul(b.Axis[i], b.AxialCoefficient))
			}
			check(v, vector(a.RadicalVector))
		case FilletedFrustumClosestToroid:
			a := original.Proof.Proof.Baseline.Baseline.Toroid
			b := got.WorldProof.Fallback.Fallback.Fallback.WorldToroid
			if b == nil || b.Face != a.Face || b.Radicand.Cmp(a.Radicand) != 0 {
				t.Fatal("torus identity")
			}
			check(b.Base, world(a.Center))
			check(b.Numerator, vector(a.Numerator))
		case FilletedFrustumClosestNestedToroid:
			a := original.Proof.Proof.Baseline.Nested
			b := got.WorldProof.Fallback.Fallback.WorldNested
			if b == nil || b.Face != a.Face || b.AzimuthSquared.Cmp(a.AzimuthSquared) != 0 || b.MajorRadius.Cmp(a.MajorRadius) != 0 || b.MinorRadius.Cmp(a.MinorRadius) != 0 || b.HeightOffset.Cmp(a.HeightOffset) != 0 {
				t.Fatal("nested radical identity")
			}
			check(b.Base, world(a.Center))
			check(b.RadialVector, vector([3]*big.Rat{a.Azimuth[0], a.Azimuth[1], r(0, 1)}))
			check(b.Axis, vector([3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}))
		case FilletedFrustumClosestBore:
			a := original.Proof.BoreProof.Bore
			b := got.WorldProof.WorldBore
			if b == nil || b.Face != a.Face || b.Radicand.Cmp(a.Radicand) != 0 {
				t.Fatal("bore identity")
			}
			check(b.Center, world(a.Center))
			check(b.RadialNumerator, vector(a.Numerator))
		case FilletedFrustumClosestRim:
			a := original.Proof.Proof.Baseline.Baseline.Baseline.Rim
			b := got.WorldProof.Fallback.Fallback.Fallback.Fallback.WorldRim
			if b == nil || !got.Complete || len(got.Tied) != 2 || b.AzimuthSquared.Cmp(a.AzimuthSquared) != 0 {
				t.Fatal("two owner rim")
			}
			check(b.Center, world([3]*big.Rat{a.Center[0], a.Center[1], a.Z}))
			check(b.RadialNumerator, vector([3]*big.Rat{fdMul(a.Radius, a.Azimuth[0]), fdMul(a.Radius, a.Azimuth[1]), r(0, 1)}))
		default:
			if got.Complete || len(got.Tied) != 0 || len(got.Unresolved) != len(q.solid.faces) {
				t.Fatal("unsupported world promotion")
			}
		}
		if len(got.Tied) > 0 {
			got.Tied[0] = -999
			if got.Original.Tied[0] < 0 {
				t.Fatal("owner alias")
			}
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := placed.ClassifyAnalyticNearestFacesRational(cancelled, world([3]*big.Rat{r(10, 1), r(1, 1), r(3, 2)}), opts)
	if e == nil || bad.WorldProof.Lower != nil || len(bad.Tied) != 0 {
		t.Fatal("atomic cancellation")
	}
}
