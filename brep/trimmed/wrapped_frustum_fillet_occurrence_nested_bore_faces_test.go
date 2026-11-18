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

func TestFilletedFrustumOccurrenceNestedBoreOrdering(t *testing.T) {
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
	for i, z := range []*big.Rat{r(69, 100), r(71, 100)} {
		p := [3]*big.Rat{r(77, 10), r(1, 100), z}
		opts := FilletedFrustumNestedBoreOptions{}
		local, e := q.ClassifyNestedTorusBoreFacesRational(ctx, p, opts)
		if e != nil {
			t.Fatal(e)
		}
		got, e := posed.ClassifyNestedTorusBoreFacesRational(ctx, transform(p, true), opts)
		sign := -1
		if i == 1 {
			sign = 1
		}
		if e != nil || !got.Original.Complete || got.Original.Comparison != sign || got.WorldNested == nil || got.WorldBore == nil {
			t.Fatal("world strict order", got, e)
		}
		if got.Original.Lower.Cmp(local.Lower) != 0 || got.Original.Upper.Cmp(local.Upper) != 0 || got.Original.RefinementChecks != local.RefinementChecks || got.Original.PairChecks != local.PairChecks || got.Original.Baseline.SupportChecks != local.Baseline.SupportChecks {
			t.Fatal("source work/bounds changed")
		}
		nb := transform(local.Nested.Center, true)
		nv := transform([3]*big.Rat{local.Nested.Azimuth[0], local.Nested.Azimuth[1], r(0, 1)}, false)
		bc, bv := transform(local.Bore.Center, true), transform(local.Bore.Numerator, false)
		for k := 0; k < 3; k++ {
			if got.WorldNested.Base[k].Cmp(nb[k]) != 0 || got.WorldNested.RadialVector[k].Cmp(nv[k]) != 0 || got.WorldNested.Axis[k].Cmp(m[k][2]) != 0 || got.WorldBore.Center[k].Cmp(bc[k]) != 0 || got.WorldBore.RadialNumerator[k].Cmp(bv[k]) != 0 {
				t.Fatal("independent world coefficients")
			}
		}
		if got.WorldNested.AzimuthSquared.Cmp(r(592901, 10000)) != 0 || got.WorldBore.Radicand.Cmp(r(28901, 10000)) != 0 || got.WorldNested.Face != local.Nested.Face || got.WorldBore.Face != local.Bore.Face || got.Original.Tied[0] != local.Tied[0] {
			t.Fatal("original radical/owner")
		}
		got.Original.Nested.AzimuthRoot[0].SetInt64(-1)
		if got.WorldNested.AzimuthRoot[0].Sign() < 0 {
			t.Fatal("world enclosure alias")
		}
	}
	tie := [3]*big.Rat{r(991, 128), r(0, 1), r(3, 4)}
	worldTie := transform(tie, true)
	no, e := posed.ClassifyNestedTorusBoreFacesRational(ctx, worldTie, FilletedFrustumNestedBoreOptions{})
	if e != nil || no.Original.Complete || len(no.Original.Tied) != 0 || no.WorldNested != nil || no.WorldBore != nil || len(no.Original.Unresolved) != len(q.solid.faces) {
		t.Fatal("world true tie must remain unresolved", e)
	}
	no, e = posed.ClassifyNestedTorusBoreFacesRational(ctx, worldTie, FilletedFrustumNestedBoreOptions{MaxPairs: 1})
	if e == nil || no.Original.Lower != nil || no.WorldNested != nil {
		t.Fatal("atomic world pair limit")
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	no, e = posed.ClassifyNestedTorusBoreFacesRational(cc, worldTie, FilletedFrustumNestedBoreOptions{})
	if e == nil || no.Original.Lower != nil {
		t.Fatal("atomic world cancellation")
	}
}
