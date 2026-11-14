package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumBoreConeOrdering(t *testing.T) {
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
	cuts := []FilletedFrustumBoreCut{{Center: geom.Vec2{X: 6}, Radius: 1}}
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
	// Independent integer-root interval oracle, not the production sign bridge.
	sqrtBounds := func(v *big.Rat) (*big.Rat, *big.Rat) {
		n := new(big.Int).Quo(new(big.Int).Lsh(new(big.Int).Set(v.Num()), 256), v.Denom())
		n.Sqrt(n)
		scale := new(big.Int).Lsh(big.NewInt(1), 128)
		return new(big.Rat).SetFrac(n, scale), new(big.Rat).SetFrac(new(big.Int).Add(n, big.NewInt(1)), scale)
	}
	distances := func(z *big.Rat) (cl, ch, bl, bh *big.Rat) {
		sl, sh := sqrtBounds(r(2113, 32))
		tl, th := sqrtBounds(r(145, 32))
		R := fdAdd(r(8, 1), fdMul(r(4, 3), z))
		return fdMul(r(9, 25), fdSquare(fdSub(R, sh))), fdMul(r(9, 25), fdSquare(fdSub(R, sl))), fdSquare(fdSub(tl, r(1, 1))), fdSquare(fdSub(th, r(1, 1)))
	}
	// Bracket the medial transition by independently bounded distances. The
	// radial query is fixed so its coarse roots do not sharpen with z's bits.
	left, right := r(3, 2), r(13, 8)
	for i := 0; i < 80; i++ {
		mid := fdMul(fdAdd(left, right), r(1, 2))
		cl, ch, bl, bh := distances(mid)
		if ch.Cmp(bl) < 0 {
			left = mid
		} else if cl.Cmp(bh) > 0 {
			right = mid
		} else {
			t.Fatal("independent oracle cannot order")
		}
	}
	for i, z := range []*big.Rat{left, right} {
		p := [3]*big.Rat{r(65, 8), r(1, 8), z}
		got, e := q.ClassifyBoreConeFacesRational(ctx, p, FilletedFrustumBoreConeOptions{Support: FilletedFrustumDistanceOptions{SqrtBits: 8}})
		want := -1
		if i == 1 {
			want = 1
		}
		if e != nil || !got.Complete || got.Comparison != want || len(got.Tied) != 1 || len(got.Excluded) != len(q.solid.faces)-1 {
			t.Fatal("ordered interior supports", got, e)
		}
		cl, ch, bl, bh := distances(z)
		if want > 0 && cl.Cmp(bh) <= 0 || want < 0 && ch.Cmp(bl) >= 0 {
			t.Fatal("independent true order")
		}
		winningFace := got.Bore.Face
		lo, hi := bl, bh
		if want < 0 {
			winningFace = got.Cone.Face
			lo, hi = cl, ch
		}
		if got.Tied[0] != winningFace || got.Lower.Cmp(lo) > 0 || got.Upper.Cmp(hi) < 0 {
			t.Fatal("owner and enclosure")
		}
		R := fdAdd(r(8, 1), fdMul(r(4, 3), z))
		cLo := fdMul(r(9, 25), fdSquare(fdSub(R, got.Cone.Root[1])))
		cHi := fdMul(r(9, 25), fdSquare(fdSub(R, got.Cone.Root[0])))
		bLo := fdSquare(fdSub(got.Bore.Root[0], r(1, 1)))
		bHi := fdSquare(fdSub(got.Bore.Root[1], r(1, 1)))
		if cLo.Cmp(bHi) > 0 || bLo.Cmp(cHi) > 0 {
			t.Fatal("fixture needs overlapping enclosures")
		}
	}
	// Rational meridional equality: cone foot(44/5,0,3/5), bore foot(7,0,3/2)
	// both attain81/64 at query(65/8,0,3/2), with two distinct original owners.
	tieP := [3]*big.Rat{r(65, 8), r(0, 1), r(3, 2)}
	tie, e := q.ClassifyBoreConeFacesRational(ctx, tieP, FilletedFrustumBoreConeOptions{})
	if e != nil || !tie.Complete || tie.Comparison != 0 || len(tie.Tied) != 2 || tie.Lower.Cmp(r(81, 64)) != 0 || tie.Upper.Cmp(tie.Lower) != 0 {
		t.Fatal("exact two-support tie", tie, e)
	}
	root := r(65, 8)
	for k, want := range [3]*big.Rat{r(44, 5), r(0, 1), r(3, 5)} {
		if fdAdd(tie.Cone.Base[k], fdMul(tie.Cone.RadicalVector[k], root)).Cmp(want) != 0 {
			t.Fatal("exact cone foot")
		}
	}
	// A closer lower cap and a bore endpoint cannot be passed as this interior
	// family even if the two untrimmed support distances are small.
	for _, p := range [][3]*big.Rat{{r(65, 8), r(1, 8), r(1, 16)}, {r(65, 8), r(1, 8), r(0, 1)}} {
		no, e := q.ClassifyBoreConeFacesRational(ctx, p, FilletedFrustumBoreConeOptions{})
		if e != nil || no.Complete || no.Cone != nil || no.Bore != nil || len(no.Unresolved) != len(q.solid.faces) {
			t.Fatal("trim or cap ambiguity", e)
		}
	}
	// A second disjoint bore is closer; the first pair must not be accepted.
	two, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, append(append([]FilletedFrustumBoreCut(nil), cuts...), FilletedFrustumBoreCut{Center: geom.Vec2{X: 29. / 4}, Radius: 1. / 8}), o)
	if e != nil {
		t.Fatal(e)
	}
	nearer, e := two.ClassifyBoreConeFacesRational(ctx, tieP, FilletedFrustumBoreConeOptions{})
	if e != nil || !nearer.Complete || len(nearer.Tied) != 1 || nearer.Tied[0] != two.cuts[1].wall || nearer.BoreChecks != 2 {
		t.Fatal("closer independent bore", nearer, e)
	}
	limited, e := two.ClassifyBoreConeFacesRational(ctx, tieP, FilletedFrustumBoreConeOptions{MaxBores: 1})
	if e == nil || limited.Lower != nil || limited.Cone != nil || limited.Bore != nil {
		t.Fatal("atomic bore scan budget")
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	no, e := q.ClassifyBoreConeFacesRational(cc, tieP, FilletedFrustumBoreConeOptions{})
	if e == nil || no.Lower != nil || no.Cone != nil {
		t.Fatal("atomic cancel")
	}
	no, e = q.ClassifyBoreConeFacesRational(ctx, tieP, FilletedFrustumBoreConeOptions{Support: FilletedFrustumDistanceOptions{MaxSupportChecks: 1}})
	if e == nil || no.Lower != nil || no.Cone != nil {
		t.Fatal("atomic support budget")
	}
}
