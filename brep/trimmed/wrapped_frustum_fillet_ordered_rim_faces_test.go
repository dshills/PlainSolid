//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestFilletedFrustumOrderedBlindRims(t *testing.T) {
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
	build := func(radius float64) FilletedFrustumMaterial {
		q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{
			{Center: geom.Vec2{X: -2}, Radius: .5},
			{Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1},
			{Center: geom.Vec2{X: 2}, Radius: radius, Depth: 19. / 4, FromUpper: true},
		}, o)
		if e != nil {
			t.Fatal(e)
		}
		return q
	}
	q := build(5. / 8)
	// Different formulas: lower minus upper = z/2-45/64+sqrt(1/2)/4.
	// At21/20 this is negative; at53/50 positive, proved by rational squares.
	for _, tc := range []struct {
		z    *big.Rat
		sign int
	}{{r(21, 20), -1}, {r(53, 50), 1}} {
		p := [3]*big.Rat{r(5, 2), r(1, 2), tc.z}
		got, e := q.ClassifyOrderedBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{})
		if e != nil || !got.Complete || got.Comparison != tc.sign || len(got.Tied) != 2 || len(got.Excluded) != len(q.solid.faces)-2 {
			t.Fatal(got, e)
		}
		threshold := fdMul(r(4, 1), fdSub(r(45, 64), fdMul(tc.z, r(1, 2))))
		if (fdSquare(threshold).Cmp(r(1, 2)) > 0) != (tc.sign < 0) {
			t.Fatal("independent order")
		}
		i := 0
		if tc.sign > 0 {
			i = 1
		}
		for _, f := range got.Tied {
			if f != got.Rims[i].Faces[0] && f != got.Rims[i].Faces[1] {
				t.Fatal("wrong selected owner")
			}
		}
		for _, rim := range got.Rims {
			if rim == nil || len(rim.RimEdges) != 4 {
				t.Fatal("missing candidate ancestry")
			}
		}
		// Independent minimum polynomial: D=S+r²+h²-2r*sqrt(S).
		radius := got.Rims[i].Radius
		h := fdSub(p[2], got.Rims[i].Z)
		a := fdAdd(r(1, 2), fdAdd(fdSquare(radius), fdSquare(h)))
		poly := func(d *big.Rat) *big.Rat { return fdSub(fdSquare(fdSub(a, d)), fdMul(r(2, 1), fdSquare(radius))) }
		if got.Upper.Cmp(a) >= 0 || poly(got.Lower).Sign() < 0 || poly(got.Upper).Sign() > 0 {
			t.Fatal("minimum enclosure")
		}
	}
	// Enclose the irrational crossover height by independent dyadic square
	// roots. Both candidate distance intervals at8 bits overlap, but their
	// exact difference has opposite signs on the two rational probes.
	scale := new(big.Int).Lsh(big.NewInt(1), 128)
	scaledHalf := new(big.Int).Lsh(big.NewInt(1), 255)
	n := new(big.Int).Sqrt(scaledHalf)
	rhoLo := new(big.Rat).SetFrac(n, scale)
	rhoHi := new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), scale)
	for i, rho := range []*big.Rat{rhoHi, rhoLo} {
		z := fdSub(r(45, 32), fdMul(rho, r(1, 2)))
		p := [3]*big.Rat{r(5, 2), r(1, 2), z}
		got, e := q.ClassifyOrderedBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{Support: FilletedFrustumDistanceOptions{SqrtBits: 8}})
		want := -1
		if i == 1 {
			want = 1
		}
		if e != nil || !got.Complete || got.Comparison != want {
			t.Fatal("unequal near crossover", got, e)
		}
		// Independent sign: difference=(sqrt(1/2)-rho)/4.
		if (fdSquare(rho).Cmp(r(1, 2)) > 0) != (want < 0) {
			t.Fatal("crossover sign")
		}
		var lo, hi [2]*big.Rat
		for j, rim := range got.Rims {
			lo[j], hi[j] = fdSquareRange(fdSub(rim.AzimuthRoot[0], rim.Radius), fdSub(rim.AzimuthRoot[1], rim.Radius))
			h2 := fdSquare(fdSub(z, rim.Z))
			lo[j].Add(lo[j], h2)
			hi[j].Add(hi[j], h2)
		}
		if lo[0].Cmp(hi[1]) > 0 || lo[1].Cmp(hi[0]) > 0 {
			t.Fatal("fixture must have overlapping numerical enclosures")
		}
	}
	// Arbitrarily close equal-radius competitors have overlapping numerical
	// enclosures. Their exact height comparison must still select the upper rim.
	equal := build(.5)
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	p := [3]*big.Rat{r(5, 2), r(1, 2), fdAdd(r(9, 8), eps)}
	got, e := equal.ClassifyOrderedBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{Support: FilletedFrustumDistanceOptions{SqrtBits: 8}})
	if e != nil || !got.Complete || got.Comparison != 1 || len(got.Tied) != 2 {
		t.Fatal("sub-ULP order", got, e)
	}
	p[2] = r(9, 8)
	tied, e := equal.ClassifyOrderedBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{})
	if e != nil || !tied.Complete || tied.Comparison != 0 || len(tied.Tied) != 4 {
		t.Fatal("symbolic equality", e)
	}
	// Rational non-axis proper rotation, including a nonbinary translation.
	m := [3][4]*big.Rat{{r(3, 5), r(-4, 5), r(0, 1), r(1, 7)}, {r(4, 5), r(3, 5), r(0, 1), r(-2, 9)}, {r(0, 1), r(0, 1), r(1, 1), r(3, 11)}}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	placed, e := q.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	p[2] = r(53, 50)
	world, e := pose.TransformPoint(ctx, p, false, 4096)
	if e != nil {
		t.Fatal(e)
	}
	wr, e := placed.ClassifyOrderedBlindRimsRational(ctx, world, FilletedFrustumOpposingRimOptions{})
	if e != nil || !wr.Original.Complete || wr.Original.Comparison != 1 {
		t.Fatal("placed order", e)
	}
	for i, rim := range wr.Original.Rims {
		for k := 0; k < 3; k++ {
			want := fdAdd(fdMul(m[k][0], fdMul(rim.Radius, r(1, 2))), fdMul(m[k][1], fdMul(rim.Radius, r(1, 2))))
			if wr.WorldRims[i].RadialNumerator[k].Cmp(want) != 0 {
				t.Fatal("placed candidate coefficient")
			}
		}
	}
	obstructed, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{
		{Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1},
		{Center: geom.Vec2{X: 2}, Radius: 5. / 8, Depth: 19. / 4, FromUpper: true},
		{Center: geom.Vec2{X: 21. / 8, Y: 5. / 8}, Radius: 1. / 8},
	}, o)
	if e != nil {
		t.Fatal(e)
	}
	blocked, e := obstructed.ClassifyOrderedBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{})
	if e != nil || blocked.Complete || len(blocked.Tied) != 0 || blocked.Rims[0] != nil || len(blocked.Unresolved) != len(obstructed.solid.faces) {
		t.Fatal("unrelated closer support", e)
	}
	bad, e := q.ClassifyOrderedBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{MaxPairs: 1})
	if e == nil || bad.Lower != nil || bad.Rims[0] != nil {
		t.Fatal("atomic budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = q.ClassifyOrderedBlindRimsRational(cancelled, p, FilletedFrustumOpposingRimOptions{})
	if e == nil || bad.Lower != nil {
		t.Fatal("atomic cancellation")
	}
	p[2] = r(1, 1)
	bad, e = q.ClassifyOrderedBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{})
	if e != nil || bad.Complete || len(bad.Unresolved) != len(q.solid.faces) {
		t.Fatal("outside strict floor gap")
	}
}
