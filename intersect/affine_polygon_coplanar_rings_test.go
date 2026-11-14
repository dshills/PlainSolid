package intersect

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func affineRingBox(lo, hi *big.Rat, hole bool) [][2]*big.Rat {
	r := [][2]*big.Rat{{lo, lo}, {hi, lo}, {hi, hi}, {lo, hi}}
	if hole {
		r[1], r[3] = r[3], r[1]
	}
	return r
}
func TestNoncontactingAffineRingsNested(t *testing.T) {
	ctx := context.Background()
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	embed := func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v, Z: u + 2*v} }
	a := affineContactPatch(t, embed, d, true)
	c := affineContactPatch(t, func(u, v float64) geom.Vec3 { return embed(1-u, v) }, d, false)
	trims := [2]AffinePolygonTrim{{Outer: affineTrimSquare(), Holes: [][][2]*big.Rat{affineRingBox(big.NewRat(2, 5), big.NewRat(3, 5), true)}}, {Outer: affineRingBox(big.NewRat(1, 5), big.NewRat(4, 5), false)}}
	for _, tc := range []struct {
		op                  AffineRingOperation
		owners, rings, dirs []int
	}{
		{AffineRingIntersection, []int{0, 1}, []int{1, 0}, []int{1, -1}},
		{AffineRingUnion, []int{0}, []int{0}, []int{1}},
		{AffineRingDifference, []int{0, 1}, []int{0, 0}, []int{1, 1}},
	} {
		q, e := SelectNoncontactingAffinePolygonRings(ctx, a, c, trims, tc.op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		rings := q.Rings()
		if !q.Complete() || len(rings) != len(tc.owners) {
			t.Fatal(rings)
		}
		for i, r := range rings {
			if r.Owner != tc.owners[i] || r.Ring != tc.rings[i] || r.Direction != tc.dirs[i] {
				t.Fatal(r)
			}
			for j, uv := range r.UV {
				xyz := r.Model[j]
				x := new(big.Rat).Set(uv[0])
				if r.Owner == 1 {
					x.Sub(big.NewRat(1, 1), x)
				}
				z := new(big.Rat).Add(x, new(big.Rat).Mul(big.NewRat(2, 1), uv[1]))
				if xyz[0].Cmp(x) != 0 || xyz[1].Cmp(uv[1]) != 0 || xyz[2].Cmp(z) != 0 {
					t.Fatal("original evaluator", r)
				}
			}
		}
		if tc.op == AffineRingIntersection {
			for _, cap := range []int{q.ExactOperations(), q.ExactOperations() - 1} {
				out, e := SelectNoncontactingAffinePolygonRings(ctx, a, c, trims, tc.op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{MaxOperations: cap})
				if cap == q.ExactOperations() {
					if e != nil || out == nil {
						t.Fatal(e)
					}
				} else if e == nil || out != nil {
					t.Fatal("partial budget")
				}
			}
			rings[0].UV[0][0].SetInt64(99)
			rings[0].Model[0][0].SetInt64(99)
			if q.Rings()[0].UV[0][0].Cmp(big.NewRat(99, 1)) == 0 || q.Rings()[0].Model[0][0].Cmp(big.NewRat(99, 1)) == 0 {
				t.Fatal("mutable publication")
			}
		}
	}
	// An island strictly inside A's hole is outside A material, not contained in it.
	trims[1].Outer = affineRingBox(big.NewRat(9, 20), big.NewRat(11, 20), false)
	q, e := SelectNoncontactingAffinePolygonRings(ctx, a, c, trims, AffineRingUnion, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil || len(q.Rings()) != 3 {
		t.Fatal(q, e)
	}
	q, e = SelectNoncontactingAffinePolygonRings(ctx, a, c, trims, AffineRingIntersection, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
	if e != nil || len(q.Rings()) != 0 {
		t.Fatal(q, e)
	}
}
func TestNoncontactingAffineRingsRejectContacts(t *testing.T) {
	d := geom.ParameterRectangle{UMax: 1, VMax: 1}
	a := affineContactPatch(t, func(u, v float64) geom.Vec3 { return geom.Vec3{X: u, Y: v} }, d, false)
	trims := [2]AffinePolygonTrim{{Outer: affineRingBox(big.NewRat(0, 1), big.NewRat(1, 2), false)}, {Outer: affineRingBox(big.NewRat(3, 4), big.NewRat(1, 1), false)}}
	for _, op := range []AffineRingOperation{AffineRingUnion, AffineRingIntersection, AffineRingDifference} {
		q, e := SelectNoncontactingAffinePolygonRings(context.Background(), a, a, trims, op, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := 2
		if op == AffineRingIntersection {
			want = 0
		} else if op == AffineRingDifference {
			want = 1
		}
		if len(q.Rings()) != want {
			t.Fatal(q.Rings())
		}
	}
	for _, lo := range []*big.Rat{big.NewRat(1, 2), big.NewRat(1, 4), big.NewRat(0, 1)} {
		trims[1].Outer = affineRingBox(lo, big.NewRat(3, 4), false)
		q, e := SelectNoncontactingAffinePolygonRings(context.Background(), a, a, trims, AffineRingUnion, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{})
		if e == nil || q != nil {
			t.Fatal("boundary contact accepted", lo)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if q, e := SelectNoncontactingAffinePolygonRings(ctx, a, a, trims, AffineRingUnion, geom.SurfacePatchOptions{}, AffinePolygonContactOptions{}); e != context.Canceled || q != nil {
		t.Fatal(q, e)
	}
}
