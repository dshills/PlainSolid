package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumOpposingBlindRimTies(t *testing.T) {
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
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 19. / 4, FromUpper: true}}, o)
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

	opts := FilletedFrustumOpposingRimOptions{}
	p := [3]*big.Rat{r(5, 2), r(1, 2), r(9, 8)}
	got, e := q.ClassifyOpposingBlindRimsRational(ctx, p, opts)
	if e != nil || !got.Complete || len(got.Tied) != 4 || len(got.Unresolved) != 0 {
		t.Fatal(got, e)
	}
	// Both independent meridians give49/64-sqrt(1/2), not a numerical tie.
	poly := func(x *big.Rat) *big.Rat { return fdSub(fdSquare(fdSub(r(49, 64), x)), r(1, 2)) }
	if got.Upper.Cmp(r(49, 64)) >= 0 || poly(got.Lower).Sign() < 0 || poly(got.Upper).Sign() > 0 {
		t.Fatal("analytic common distance")
	}
	owners := map[int]bool{}
	for i, rim := range got.Rims {
		if rim == nil || rim.AzimuthSquared.Cmp(r(1, 2)) != 0 || len(rim.Faces) != 2 || len(rim.RimEdges) != 4 {
			t.Fatal("original rim")
		}
		for _, f := range rim.Faces {
			if owners[f] {
				t.Fatal("non-distinct owners")
			}
			owners[f] = true
		}
		wantZ := r(1, 1)
		if i == 1 {
			wantZ = r(5, 4)
		}
		if rim.Z.Cmp(wantZ) != 0 {
			t.Fatal("original floor")
		}
	}
	for _, id := range got.Tied {
		if !owners[id] {
			t.Fatal("owner ledger")
		}
	}
	if len(got.Excluded) != len(q.solid.faces)-4 {
		t.Fatal("omitted owner")
	}
	worldResult, e := placed.ClassifyOpposingBlindRimsRational(ctx, world(p), opts)
	if e != nil || !worldResult.Original.Complete {
		t.Fatal(e)
	}
	for i, w := range worldResult.WorldRims {
		if w == nil {
			t.Fatal("missing world rim")
		}
		rim := got.Rims[i]
		want := world([3]*big.Rat{rim.Center[0], rim.Center[1], rim.Z})
		for k := 0; k < 3; k++ {
			if w.Center[k].Cmp(want[k]) != 0 || w.RadialNumerator[k].Cmp(fdAdd(fdMul(m[k][0], r(1, 4)), fdMul(m[k][1], r(1, 4)))) != 0 {
				t.Fatal("world exact coefficients")
			}
		}
	}
	// Exact midpoint is essential: arbitrarily close is not a symbolic tie.
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	off := p
	off[2] = fdAdd(p[2], eps)
	no, e := q.ClassifyOpposingBlindRimsRational(ctx, off, opts)
	if e != nil || no.Complete || len(no.Tied) != 0 || no.Rims[0] != nil || len(no.Unresolved) != len(q.solid.faces) {
		t.Fatal("rounded midpoint tie", e)
	}
	no, e = q.ClassifyOpposingBlindRimsRational(ctx, [3]*big.Rat{r(21, 10), r(1, 10), r(9, 8)}, opts)
	if e != nil || no.Complete {
		t.Fatal("inside disk is not rim", e)
	}
	// A separate, strictly disjoint bore can be closer than both rims.
	obstructed, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 19. / 4, FromUpper: true}, {Center: geom.Vec2{X: 21. / 8, Y: 5. / 8}, Radius: 1. / 8}}, o)
	if e != nil {
		t.Fatal(e)
	}
	no, e = obstructed.ClassifyOpposingBlindRimsRational(ctx, p, opts)
	if e != nil || no.Complete || len(no.Tied) != 0 || no.Rims[0] != nil {
		t.Fatal("closer unrelated wall", e)
	}
	bad, e := q.ClassifyOpposingBlindRimsRational(ctx, p, FilletedFrustumOpposingRimOptions{MaxPairs: 1})
	if e == nil || bad.Complete || bad.Lower != nil || bad.Rims[0] != nil {
		t.Fatal("atomic pair budget")
	}
}
