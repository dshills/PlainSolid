package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumTorusBoreOrdering(t *testing.T) {
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
	var native bytes.Buffer
	if e = WriteFilletedFrustumMaterialJSON(ctx, &native, q); e != nil {
		t.Fatal(e)
	}
	q, e = ReadFilletedFrustumMaterialJSON(ctx, &native, FilletedFrustumMaterialJSONLimits{})
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
	transform := func(v [3]*big.Rat, point bool) [3]*big.Rat {
		var out [3]*big.Rat
		for i := range out {
			out[i] = new(big.Rat)
			if point {
				out[i].Set(m[i][3])
			}
			for j := range v {
				out[i].Add(out[i], fdMul(m[i][j], v[j]))
			}
		}
		return out
	}
	checkWorld := func(p [3]*big.Rat, local FilletedFrustumTorusBoreFaces, opts FilletedFrustumTorusBoreOptions) {
		got, e := posed.ClassifyTorusBoreFacesRational(ctx, transform(p, true), opts)
		if e != nil || !got.Original.Complete || got.Original.Comparison != local.Comparison || got.Original.Lower.Cmp(local.Lower) != 0 || got.Original.Upper.Cmp(local.Upper) != 0 || got.Original.Baseline.SupportChecks != local.Baseline.SupportChecks {
			t.Fatal("world proof", e)
		}
		tc, tn := transform(local.Toroid.Center, true), transform(local.Toroid.Numerator, false)
		bc, bn := transform(local.Bore.Center, true), transform(local.Bore.Numerator, false)
		for k := 0; k < 3; k++ {
			if got.WorldToroid.Base[k].Cmp(tc[k]) != 0 || got.WorldToroid.Numerator[k].Cmp(tn[k]) != 0 || got.WorldBore.Center[k].Cmp(bc[k]) != 0 || got.WorldBore.RadialNumerator[k].Cmp(bn[k]) != 0 {
				t.Fatal("exact world coefficients")
			}
		}
		if got.WorldToroid.Face != local.Toroid.Face || got.WorldBore.Face != local.Bore.Face || len(got.Original.Tied) != len(local.Tied) {
			t.Fatal("world owners")
		}
		for i, f := range local.Tied {
			if got.Original.Tied[i] != f {
				t.Fatal("world tied owner")
			}
		}
	}
	sqrtBounds := func(v *big.Rat) (*big.Rat, *big.Rat) {
		n := new(big.Int).Quo(new(big.Int).Lsh(new(big.Int).Set(v.Num()), 512), v.Denom())
		n.Sqrt(n)
		scale := new(big.Int).Lsh(big.NewInt(1), 256)
		return new(big.Rat).SetFrac(n, scale), new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), scale)
	}
	x, y := r(30723, 4010), r(308, 401) // rho77/10, unit azimuth(399,40)/401.
	T := fdAdd(fdSquare(fdSub(x, r(6, 1))), fdSquare(y))
	tl, th := sqrtBounds(T)
	bLo, bHi := fdSquare(fdSub(tl, r(1, 1))), fdSquare(fdSub(th, r(1, 1)))
	distances := func(z *big.Rat) (*big.Rat, *big.Rat) {
		D := fdAdd(r(1, 6400), fdSquare(fdSub(z, r(15, 16))))
		dl, dh := sqrtBounds(D)
		return fdSquare(fdSub(r(15, 16), dh)), fdSquare(fdSub(r(15, 16), dl))
	}
	left, right := r(4, 5), r(17, 20)
	for i := 0; i < 80; i++ {
		mid := fdMul(fdAdd(left, right), r(1, 2))
		lo, hi := distances(mid)
		if hi.Cmp(bLo) < 0 {
			left = mid
		} else if lo.Cmp(bHi) > 0 {
			right = mid
		} else {
			t.Fatal("independent comparison unresolved")
		}
	}
	for i, z := range []*big.Rat{left, right} {
		got, e := q.ClassifyTorusBoreFacesRational(ctx, [3]*big.Rat{x, y, z}, FilletedFrustumTorusBoreOptions{Support: FilletedFrustumDistanceOptions{SqrtBits: 8}})
		sign := -1
		if i == 1 {
			sign = 1
		}
		if e != nil || !got.Complete || got.Comparison != sign || len(got.Tied) != 1 {
			t.Fatal("two radical torus/bore", got, e)
		}
		lo, hi := distances(z)
		face := got.Toroid.Face
		if sign > 0 {
			lo, hi = bLo, bHi
			face = got.Bore.Face
		}
		if got.Tied[0] != face || got.Lower.Cmp(lo) > 0 || got.Upper.Cmp(hi) < 0 || len(got.Excluded) != len(q.solid.faces)-1 {
			t.Fatal("exact selected owner/enclosure")
		}
		torusLo, torusHi := fdSquareRange(fdSub(got.Toroid.Root[0], r(15, 16)), fdSub(got.Toroid.Root[1], r(15, 16)))
		boreLo, boreHi := fdSquareRange(fdSub(got.Bore.Root[0], r(1, 1)), fdSub(got.Bore.Root[1], r(1, 1)))
		if torusLo.Cmp(boreHi) > 0 || boreLo.Cmp(torusHi) > 0 {
			t.Fatal("coarse intervals must overlap")
		}
		checkWorld([3]*big.Rat{x, y, z}, got, FilletedFrustumTorusBoreOptions{Support: FilletedFrustumDistanceOptions{SqrtBits: 8}})
	}
	// Pythagorean meridian v=(7,-24)/128 has length25/128. Both retained
	// feet attain(95/128)^2; torus foot(159/20,0,3/80), bore foot(7,0,3/4).
	tieP := [3]*big.Rat{r(991, 128), r(0, 1), r(3, 4)}
	tie, e := q.ClassifyTorusBoreFacesRational(ctx, tieP, FilletedFrustumTorusBoreOptions{})
	if e != nil || !tie.Complete || tie.Comparison != 0 || len(tie.Tied) != 2 || tie.Lower.Cmp(r(9025, 16384)) != 0 || tie.Upper.Cmp(tie.Lower) != 0 {
		t.Fatal("exact tie", tie, e)
	}
	checkWorld(tieP, tie, FilletedFrustumTorusBoreOptions{})
	for k, want := range [3]*big.Rat{r(159, 20), r(0, 1), r(3, 80)} {
		if fdAdd(tie.Toroid.Center[k], new(big.Rat).Quo(tie.Toroid.Numerator[k], r(25, 128))).Cmp(want) != 0 {
			t.Fatal("original torus foot")
		}
	}
	for _, p := range [][3]*big.Rat{{r(123, 16), r(0, 1), r(1, 2)}, {r(63, 8), r(0, 1), r(11, 16)}, {r(8, 1), r(1, 10), r(7, 10)}} {
		no, e := q.ClassifyTorusBoreFacesRational(ctx, p, FilletedFrustumTorusBoreOptions{})
		if e != nil || no.Complete || no.Toroid != nil || no.Bore != nil {
			t.Fatal("endpoint or unsupported azimuth", no, e)
		}
	}
	no, e := q.ClassifyTorusBoreFacesRational(ctx, tieP, FilletedFrustumTorusBoreOptions{Support: FilletedFrustumDistanceOptions{MaxSupportChecks: 1}})
	if e == nil || no.Lower != nil {
		t.Fatal("atomic budget")
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	no, e = q.ClassifyTorusBoreFacesRational(cc, tieP, FilletedFrustumTorusBoreOptions{})
	if e == nil || no.Lower != nil {
		t.Fatal("atomic cancellation")
	}
}
