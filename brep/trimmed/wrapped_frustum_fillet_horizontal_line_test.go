package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactFilletedFrustumHorizontalLine(t *testing.T) {
	ctx := context.Background()
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
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	r := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	opts := geom.ExactCurvePlaneEventOptions{}
	square := func(v *big.Rat) *big.Rat { return new(big.Rat).Mul(v, v) }
	for _, sign := range []int64{1, -1} {
		origin := [2]*big.Rat{r(0, 1), r(1, 3)}
		dir := [2]*big.Rat{r(sign, 1), r(0, 1)}
		a, e := q.IntersectHorizontalRational(ctx, origin, dir, r(15, 208), r(-20, 1), r(20, 1), opts)
		if e != nil || len(a.Hits) != 6 || len(a.TrimmedAway) != 2 {
			t.Fatal(a, e)
		}
		for i, h := range a.Hits {
			want := "entry"
			if i%2 == 1 {
				want = "exit"
			}
			if h.Kind != want || len(h.Faces) != 1 {
				t.Fatal("transition", h)
			}
			kind, e := h.Parameter.Classify(ctx, opts)
			if e != nil || kind.Kind != geom.ExactRootKindIrrational {
				t.Fatal("irrational quadratic", kind, e)
			}
			en, e := h.Parameter.Enclose(ctx, r(1, 1000000), opts)
			if e != nil {
				t.Fatal(e)
			}
			cx, R := r(0, 1), r(837, 104)
			owner := h.Faces[0]
			if owner != q.fillets.Blends[0] {
				R = r(1, 2)
				if owner == q.cuts[0].wall {
					cx = r(-2, 1)
				} else if owner == q.cuts[1].wall {
					cx = r(2, 1)
				} else {
					t.Fatal("owner", owner)
				}
			}
			eval := func(t *big.Rat) *big.Rat {
				x := new(big.Rat).Sub(new(big.Rat).Mul(r(sign, 1), t), cx)
				return new(big.Rat).Sub(new(big.Rat).Add(square(x), r(1, 9)), square(R))
			}
			if eval(en.Interval.Lower).Sign()*eval(en.Interval.Upper).Sign() > 0 {
				t.Fatal("independent circle polynomial")
			}
		}
	}
	tangent, e := q.IntersectHorizontalRational(ctx, [2]*big.Rat{r(0, 1), r(1, 2)}, [2]*big.Rat{r(1, 1), r(0, 1)}, r(15, 208), r(-20, 1), r(20, 1), opts)
	if e != nil || len(tangent.Hits) != 4 {
		t.Fatal(tangent, e)
	}
	for _, h := range tangent.Hits[1:3] {
		if h.Kind != "tangent" || h.Before != geom.BoreInside || h.After != geom.BoreInside {
			t.Fatal("bore tangent", h)
		}
	}
	joint, e := q.IntersectHorizontalRational(ctx, [2]*big.Rat{r(0, 1), r(0, 1)}, [2]*big.Rat{r(1, 1), r(0, 1)}, r(3, 16), r(-20, 1), r(20, 1), opts)
	if e != nil || len(joint.Hits) != 6 || len(joint.Hits[0].Faces) != 2 || len(joint.Hits[5].Faces) != 2 {
		t.Fatal("cone torus seam", joint, e)
	}
	gap, e := q.IntersectHorizontalRational(ctx, [2]*big.Rat{r(0, 1), r(0, 1)}, [2]*big.Rat{r(1, 1), r(0, 1)}, r(3, 2), r(-20, 1), r(20, 1), opts)
	if e != nil || len(gap.Hits) != 4 || len(gap.TrimmedAway) != 4 {
		t.Fatal("inactive opposing blind supports", gap, e)
	}
	// A genuinely non-axis horizontal direction uses the same complete quadratics.
	oblique, e := q.IntersectHorizontalRational(ctx, [2]*big.Rat{r(0, 1), r(0, 1)}, [2]*big.Rat{r(3, 5), r(4, 5)}, r(3, 2), r(-20, 1), r(20, 1), opts)
	if e != nil || len(oblique.Hits) != 2 {
		t.Fatal("rational XY oblique", oblique, e)
	}
	for _, z := range []*big.Rat{r(0, 1), r(1, 1), r(1, 10)} {
		bad, e := q.IntersectHorizontalRational(ctx, [2]*big.Rat{r(0, 1), r(0, 1)}, [2]*big.Rat{r(1, 1), r(0, 1)}, z, r(-20, 1), r(20, 1), opts)
		if e == nil || bad.Hits != nil || bad.TrimmedAway != nil {
			t.Fatal("coincident plane or nonrational radius", e)
		}
	}
	bad, e := q.IntersectHorizontalRational(ctx, [2]*big.Rat{r(0, 1), r(0, 1)}, [2]*big.Rat{r(1, 1), r(0, 1)}, r(3, 2), r(-20, 1), r(20, 1), geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || bad.Hits != nil {
		t.Fatal("atomic budget")
	}
}
