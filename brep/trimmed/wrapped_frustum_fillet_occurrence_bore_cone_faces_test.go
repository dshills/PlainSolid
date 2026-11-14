package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumOccurrenceBoreConeOrdering(t *testing.T) {
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
	m := [3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	posed, e := q.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	transform := func(p [3]*big.Rat, point bool) [3]*big.Rat {
		var out [3]*big.Rat
		for i := range out {
			out[i] = new(big.Rat)
			if point {
				out[i].Set(m[i][3])
			}
			for j := range p {
				out[i].Add(out[i], fdMul(m[i][j], p[j]))
			}
		}
		return out
	}
	for _, tc := range []struct {
		p    [3]*big.Rat
		sign int
	}{{[3]*big.Rat{r(65, 8), r(1, 8), r(3, 2)}, -1}, {[3]*big.Rat{r(65, 8), r(1, 8), r(49, 32)}, 1}, {[3]*big.Rat{r(65, 8), r(0, 1), r(3, 2)}, 0}} {
		local, e := q.ClassifyBoreConeFacesRational(ctx, tc.p, FilletedFrustumBoreConeOptions{})
		if e != nil {
			t.Fatal(e)
		}
		world := transform(tc.p, true)
		got, e := posed.ClassifyBoreConeFacesRational(ctx, world, FilletedFrustumBoreConeOptions{})
		if e != nil || !got.Original.Complete || got.Original.Comparison != tc.sign || got.WorldCone == nil || got.WorldBore == nil {
			t.Fatal(got, e)
		}
		if got.Original.Lower.Cmp(local.Lower) != 0 || got.Original.Upper.Cmp(local.Upper) != 0 || got.Original.Baseline.SupportChecks != local.Baseline.SupportChecks || got.Original.BoreChecks != local.BoreChecks {
			t.Fatal("changed source proof")
		}
		if len(got.Original.Tied) != len(local.Tied) || len(got.Original.Excluded) != len(local.Excluded) {
			t.Fatal("owner counts")
		}
		for i, f := range local.Tied {
			if got.Original.Tied[i] != f {
				t.Fatal("original owner ID")
			}
		}
		cb := transform(local.Cone.Base, true)
		cv := transform(local.Cone.RadicalVector, false)
		bc := transform(local.Bore.Center, true)
		bv := transform(local.Bore.Numerator, false)
		for k := 0; k < 3; k++ {
			if got.WorldCone.Base[k].Cmp(cb[k]) != 0 || fdAdd(got.WorldCone.RadialVector[k], fdMul(got.WorldCone.Axis[k], got.WorldCone.AxialCoefficient)).Cmp(cv[k]) != 0 || got.WorldBore.Center[k].Cmp(bc[k]) != 0 || got.WorldBore.RadialNumerator[k].Cmp(bv[k]) != 0 {
				t.Fatal("independent world coefficients")
			}
			if got.WorldCone.Axis[k].Cmp(m[k][2]) != 0 || got.WorldBore.Axis[k].Cmp(m[k][2]) != 0 {
				t.Fatal("world axis")
			}
		}
		if got.WorldCone.Face != local.Cone.Face || got.WorldBore.Face != local.Bore.Face || got.WorldCone.Radicand.Cmp(local.Cone.Radicand) != 0 || got.WorldBore.Radicand.Cmp(local.Bore.Radicand) != 0 {
			t.Fatal("source radical/owner")
		}
		if tc.sign == 0 {
			for _, foot := range [][3]*big.Rat{{r(44, 5), r(0, 1), r(3, 5)}, {r(7, 1), r(0, 1), r(3, 2)}} {
				wf := transform(foot, true)
				d := new(big.Rat)
				for k := range wf {
					d.Add(d, fdSquare(fdSub(wf[k], world[k])))
				}
				if d.Cmp(r(81, 64)) != 0 {
					t.Fatal("independent world squared tie")
				}
			}
		}
		// Returned world coefficients are owned separately from local proof data.
		got.Original.Cone.Root[0].SetInt64(-1)
		if got.WorldCone.Root[0].Sign() < 0 {
			t.Fatal("aliased radical enclosure")
		}
	}
	unsupported := transform([3]*big.Rat{r(65, 8), r(1, 8), r(1, 16)}, true)
	no, e := posed.ClassifyBoreConeFacesRational(ctx, unsupported, FilletedFrustumBoreConeOptions{})
	if e != nil || no.Original.Complete || no.WorldCone != nil || no.WorldBore != nil {
		t.Fatal("placed trim rejection")
	}
	point := transform([3]*big.Rat{r(65, 8), r(0, 1), r(3, 2)}, true)
	no, e = posed.ClassifyBoreConeFacesRational(ctx, point, FilletedFrustumBoreConeOptions{Support: FilletedFrustumDistanceOptions{MaxSupportChecks: 1}})
	if e == nil || no.Original.Lower != nil || no.WorldCone != nil {
		t.Fatal("atomic budget")
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	no, e = posed.ClassifyBoreConeFacesRational(cc, point, FilletedFrustumBoreConeOptions{})
	if e == nil || no.Original.Lower != nil {
		t.Fatal("atomic cancellation")
	}
}
