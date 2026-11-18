//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestExactFilletedFrustumObliqueLine(t *testing.T) {
	slowtest.Skip(t)
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
	var buf bytes.Buffer
	if e = WriteJSON(&buf, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSONContext(ctx, &buf, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	r := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	opts := geom.ExactCurvePlaneEventOptions{}
	origin := [3]*big.Rat{r(837, 104), r(0, 1), r(15, 208)}
	direction := [3]*big.Rat{r(1, 1), r(1, 1), r(1, 1)}
	hit, e := q.IntersectLineRational(ctx, origin, direction, r(-1, 1), r(8, 1), opts)
	if e != nil {
		t.Fatal(e)
	}
	t.Log("quartic", len(hit.Hits), len(hit.TrimmedAway), hit.RootOperations)
	if len(hit.Hits) != 2 || hit.Hits[0].Kind != "entry" || hit.Hits[1].Kind != "exit" || hit.Hits[0].Faces[0] != q.fillets.Blends[0] || hit.Hits[1].Faces[0] != q.fillets.Blends[1] {
		t.Fatal(hit)
	}
	first, e := hit.Hits[0].Parameter.Classify(ctx, opts)
	if e != nil || first.Rational == nil || first.Rational.Sign() != 0 {
		t.Fatal("independent Pythagorean meridian root", first, e)
	}
	// P is independently on lower circle with normal(5/13,0,-12/13).
	// Dot(normal,direction)=-7/13<0 proves its entry orientation.
	before, e := q.ClassifyRational(ctx, [3]*big.Rat{new(big.Rat).Sub(origin[0], r(1, 1000)), r(-1, 1000), new(big.Rat).Sub(origin[2], r(1, 1000))})
	if e != nil || before.Location != geom.BoreOutside {
		t.Fatal(before, e)
	}
	after, e := q.ClassifyRational(ctx, [3]*big.Rat{new(big.Rat).Add(origin[0], r(1, 1000)), r(1, 1000), new(big.Rat).Add(origin[2], r(1, 1000))})
	if e != nil || after.Location != geom.BoreInside {
		t.Fatal(after, e)
	}
	if len(hit.TrimmedAway) == 0 {
		t.Fatal("missing unretained support contacts")
	}
	// Independent full torus implicit signs bracket the second (nonrational)
	// boundary: (rho²+(z-Cz)²+M²-r²)²-4M²rho².
	enclosure, e := hit.Hits[1].Parameter.Enclose(ctx, r(1, 1000000), opts)
	if e != nil || enclosure.Interval.Lower.Cmp(enclosure.Interval.Upper) == 0 {
		t.Fatal(enclosure, e)
	}
	square := func(v *big.Rat) *big.Rat { return new(big.Rat).Mul(v, v) }
	implicit := func(t *big.Rat) *big.Rat {
		x := new(big.Rat).Add(origin[0], t)
		z := new(big.Rat).Sub(new(big.Rat).Add(origin[2], t), r(33, 8))
		rho2 := new(big.Rat).Add(square(x), square(t))
		a := new(big.Rat).Add(rho2, square(z))
		a.Add(a, new(big.Rat).Sub(square(r(83, 8)), square(r(15, 8))))
		return new(big.Rat).Sub(square(a), new(big.Rat).Mul(r(4, 1), new(big.Rat).Mul(square(r(83, 8)), rho2)))
	}
	if implicit(enclosure.Interval.Lower).Sign()*implicit(enclosure.Interval.Upper).Sign() > 0 {
		t.Fatal("independent quartic bracket")
	}
	for _, sign := range []int64{1, -1} {
		a := [3]*big.Rat{r(-4, 1), r(0, 1), r(1, 2)}
		d := [3]*big.Rat{r(sign, 1), r(0, 1), r(sign, 4)}
		lo, hi := r(-3, 1), r(20, 1)
		if sign < 0 {
			lo, hi = r(-20, 1), r(3, 1)
		}
		v, e := q.IntersectLineRational(ctx, a, d, lo, hi, opts)
		if e != nil || len(v.Hits) != 6 {
			t.Fatal("mixed zones", v, e)
		}
		floorSeen := false
		for i, h := range v.Hits {
			want := "entry"
			if i%2 == 1 {
				want = "exit"
			}
			if h.Kind != want {
				t.Fatal(h)
			}
			for _, face := range h.Faces {
				floorSeen = floorSeen || face == q.cuts[2].floor
			}
		}
		if !floorSeen {
			t.Fatal("blind floor missing")
		}
	}
	// The same full quartic path must explicitly reject the inner torus branch.
	horizontal, e := q.IntersectLineRational(ctx, [3]*big.Rat{r(0, 1), r(0, 1), r(33, 8)}, [3]*big.Rat{r(1, 1), r(0, 1), r(0, 1)}, r(-20, 1), r(20, 1), opts)
	if e != nil || len(horizontal.Hits) != 6 {
		t.Fatal("general inner-branch fixture", horizontal, e)
	}
	rejectedInner := 0
	for _, discard := range horizontal.TrimmedAway {
		if discard.Face != q.fillets.Blends[1] {
			continue
		}
		value, e := discard.Parameter.SignPolynomial(ctx, []*big.Rat{r(-289, 4), r(0, 1), r(1, 1)}, opts)
		if e != nil {
			t.Fatal(e)
		}
		if value.Sign == 0 {
			rejectedInner++
		}
	}
	if rejectedInner != 2 {
		t.Fatal("inner branch was not independently rejected", rejectedInner)
	}
	tangent, e := q.IntersectLineRational(ctx, [3]*big.Rat{r(49, 4), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(-1, 1), r(7, 1), opts)
	if e != nil || len(tangent.Hits) != 1 || tangent.Hits[0].Kind != "tangent" {
		t.Fatal("quartic multiplicity tangent", tangent, e)
	}
	rim, e := q.IntersectLineRational(ctx, [3]*big.Rat{r(123, 16), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(-1, 1), r(7, 1), opts)
	if e != nil || len(rim.Hits) != 2 || len(rim.Hits[0].Faces) != 2 {
		t.Fatal("quartic/plane exact grouping", rim, e)
	}
	// Actual work is cumulatively accounted, including final ledger comparisons.
	failed, e := q.IntersectLineRational(ctx, origin, direction, r(-1, 1), r(8, 1), geom.ExactCurvePlaneEventOptions{MaxOperations: hit.RootOperations - 1})
	if e == nil || failed.Hits != nil || failed.TrimmedAway != nil {
		t.Fatal("cumulative work limit", failed.RootOperations, e)
	}
	coincident, e := q.IntersectLineRational(ctx, [3]*big.Rat{r(-3, 2), r(0, 1), r(0, 1)}, [3]*big.Rat{r(0, 1), r(0, 1), r(1, 1)}, r(0, 1), r(6, 1), opts)
	if e == nil || coincident.Hits != nil {
		t.Fatal("coincident bore wall", e)
	}
}
