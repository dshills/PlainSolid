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

func TestTwoPositiveRadicalOrder(t *testing.T) {
	r := big.NewRat
	// Includes the L<0 and L=0 cases that would give a false sign if squared.
	for _, tc := range []struct {
		a, b, s, c, d *big.Rat
		sign          int
	}{
		{r(-3, 1), r(1, 1), r(1, 1), r(1, 1), r(1, 1), -1},
		{r(-1, 1), r(1, 1), r(1, 1), r(1, 1), r(1, 1), -1},
		{r(0, 1), r(1, 1), r(2, 1), r(1, 1), r(3, 1), 1},
		{r(0, 1), r(2, 1), r(2, 1), r(1, 1), r(8, 1), 0},
		{r(1, 1), r(3, 1), r(1, 1), r(1, 1), r(1, 1), -1},
		{r(-1, 1), r(1, 1), r(1, 1), r(3, 1), r(1, 1), 1},
	} {
		if got := fdTwoPositiveRadicalsSign(tc.a, tc.b, tc.s, tc.c, tc.d); got != tc.sign {
			t.Fatal(tc, got)
		}
	}
}

func TestFilletedFrustumIndependentBlindRims(t *testing.T) {
	slowtest.Skip(t)
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
	cuts := []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 9. / 4}, Radius: .5, Depth: 19. / 4, FromUpper: true}}
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteFilletedFrustumMaterialJSON(ctx, &native, q); e != nil {
		t.Fatal(e)
	}
	q, e = ReadFilletedFrustumMaterialJSON(ctx, &native, FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	// Independent exact rational brackets around sqrt(1/2) and sqrt(5/16).
	sqrtBounds := func(numerator int64, shift uint) (*big.Rat, *big.Rat) {
		scale := new(big.Int).Lsh(big.NewInt(1), 128)
		n := new(big.Int).Sqrt(new(big.Int).Lsh(big.NewInt(numerator), 256-shift))
		return new(big.Rat).SetFrac(n, scale), new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), scale)
	}
	slo, shi := sqrtBounds(1, 1)
	tlo, thi := sqrtBounds(5, 4)
	// Difference D_lower-D_upper=z/2-3/8-sqrt(1/2)+sqrt(5/16).
	zlo := fdAdd(r(3, 4), fdMul(r(2, 1), fdSub(slo, thi)))
	zhi := fdAdd(r(3, 4), fdMul(r(2, 1), fdSub(shi, tlo)))
	for i, z := range []*big.Rat{zlo, zhi} {
		p := [3]*big.Rat{r(5, 2), r(1, 2), z}
		got, e := q.ClassifyIndependentBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{Support: FilletedFrustumDistanceOptions{SqrtBits: 8}})
		sign := -1
		if i == 1 {
			sign = 1
		}
		if e != nil || !got.Complete || got.Comparison != sign || len(got.Tied) != 2 || len(got.Excluded) != len(q.solid.faces)-2 {
			t.Fatal("two-radical ordering", got, e)
		}
		if got.Rims[0].AzimuthSquared.Cmp(r(1, 2)) != 0 || got.Rims[1].AzimuthSquared.Cmp(r(5, 16)) != 0 {
			t.Fatal("distinct original azimuths")
		}
		var lo, hi [2]*big.Rat
		for j, rim := range got.Rims {
			lo[j], hi[j] = fdSquareRange(fdSub(rim.AzimuthRoot[0], rim.Radius), fdSub(rim.AzimuthRoot[1], rim.Radius))
			h2 := fdSquare(fdSub(z, rim.Z))
			lo[j].Add(lo[j], h2)
			hi[j].Add(hi[j], h2)
			if len(rim.RimEdges) != 4 {
				t.Fatal("original circle ancestry")
			}
		}
		if lo[0].Cmp(hi[1]) > 0 || lo[1].Cmp(hi[0]) > 0 {
			t.Fatal("must overlap numerically")
		}
		winner := 0
		if i == 1 {
			winner = 1
		}
		if got.Lower.Cmp(lo[winner]) != 0 || got.Upper.Cmp(hi[winner]) != 0 {
			t.Fatal("selected enclosure")
		}
		for _, f := range got.Tied {
			if f != got.Rims[winner].Faces[0] && f != got.Rims[winner].Faces[1] {
				t.Fatal("selected owners")
			}
		}
		old, e := q.ClassifyOrderedBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{})
		if e != nil || old.Complete || old.Rims[0] != nil {
			t.Fatal("legacy coaxial gate changed")
		}
	}
	// The two radicals can also be exactly dependent: S=1/2,T=2,
	// radii1/2,1/4 make the radical terms cancel. Independent axial offsets
	// at z101/64 give identical constants, proving four original owners.
	tiedCuts := []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: .25}, {Center: geom.Vec2{X: 7. / 2, Y: 3. / 2}, Radius: .25, Depth: 15. / 4, FromUpper: true}}
	tieQ, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, tiedCuts, o)
	if e != nil {
		t.Fatal(e)
	}
	tieP := [3]*big.Rat{r(5, 2), r(1, 2), r(101, 64)}
	tie, e := tieQ.ClassifyIndependentBlindRimsRational(ctx, tieP, FilletedFrustumOpposingRimOptions{})
	if e != nil || !tie.Complete || tie.Comparison != 0 || len(tie.Tied) != 4 {
		t.Fatal("dependent radical tie", tie, e)
	}
	// Proper non-axis pose preserves each distinct center and radial numerator.
	m := [3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	posed, e := q.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	p := [3]*big.Rat{r(5, 2), r(1, 2), zhi}
	world, e := pose.TransformPoint(ctx, p, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	wr, e := posed.ClassifyIndependentBlindRimsRational(ctx, world, FilletedFrustumOpposingRimOptions{})
	if e != nil || !wr.Original.Complete || wr.Original.Comparison != 1 {
		t.Fatal("placed distinct radicals", e)
	}
	for i, rim := range wr.Original.Rims {
		for k := 0; k < 3; k++ {
			center := new(big.Rat).Set(m[k][3])
			center.Add(center, fdMul(m[k][0], rim.Center[0]))
			center.Add(center, fdMul(m[k][1], rim.Center[1]))
			center.Add(center, fdMul(m[k][2], rim.Z))
			radial := fdAdd(fdMul(m[k][0], fdMul(rim.Radius, rim.Azimuth[0])), fdMul(m[k][1], fdMul(rim.Radius, rim.Azimuth[1])))
			if wr.WorldRims[i].Center[k].Cmp(center) != 0 || wr.WorldRims[i].RadialNumerator[k].Cmp(radial) != 0 {
				t.Fatal("world coefficients")
			}
		}
	}
	obstructedCuts := append(append([]FilletedFrustumBoreCut(nil), cuts...), FilletedFrustumBoreCut{Center: geom.Vec2{X: 21. / 8, Y: 5. / 8}, Radius: 1. / 8})
	obstructed, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, obstructedCuts, o)
	if e != nil {
		t.Fatal(e)
	}
	blocked, e := obstructed.ClassifyIndependentBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{})
	if e != nil || blocked.Complete || len(blocked.Tied) != 0 || blocked.Rims[0] != nil || len(blocked.Unresolved) != len(obstructed.solid.faces) {
		t.Fatal("unrelated closer wall", e)
	}
	bad, e := q.ClassifyIndependentBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{MaxPairs: 1})
	if e == nil || bad.Lower != nil || bad.Rims[0] != nil {
		t.Fatal("atomic pair budget")
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = q.ClassifyIndependentBlindRimsRational(cc, p, FilletedFrustumOpposingRimOptions{})
	if e == nil || bad.Lower != nil {
		t.Fatal("atomic cancel")
	}
}
