package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestExactFilletedFrustumAxialLine(t *testing.T) {
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
	var native bytes.Buffer
	if e = WriteJSON(&native, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSONContext(ctx, &native, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	r := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	opts := geom.ExactCurvePlaneEventOptions{}
	line := func(x, z, dz *big.Rat) FilletedAxialLine {
		t.Helper()
		v, e := q.IntersectAxialRational(ctx, x, r(0, 1), z, dz, r(-1, 1), r(7, 1), opts)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, reverse := range []bool{false, true} {
		z, dz := r(0, 1), r(1, 1)
		if reverse {
			z, dz = r(6, 1), r(-1, 1)
		}
		a := line(r(8, 1), z, dz)
		if len(a.Hits) != 2 || a.Hits[0].Kind != "entry" || a.Hits[1].Kind != "exit" {
			t.Fatal(a)
		}
		index := 0
		if reverse {
			index = 1
		}
		h := a.Hits[index]
		if len(h.Faces) != 1 || h.Faces[0] != q.fillets.Blends[0] {
			t.Fatal("band owner", h)
		}
		kind, e := h.Parameter.Classify(ctx, opts)
		if e != nil || kind.Kind != geom.ExactRootKindIrrational {
			t.Fatal("irrational torus root", kind, e)
		}
		enclosure, e := h.Parameter.Enclose(ctx, r(1, 1000000), opts)
		if e != nil || !enclosure.Converged {
			t.Fatal(e)
		}
		// Independent root z=15/16-sqrt(25/32), evaluated by an exact
		// polynomial sign on the returned ORIGINAL t bracket (also reversed).
		eval := func(v *big.Rat) *big.Rat {
			height := new(big.Rat).Add(z, new(big.Rat).Mul(dz, v))
			height.Sub(height, r(15, 16))
			return new(big.Rat).Sub(new(big.Rat).Mul(height, height), r(25, 32))
		}
		aSign, bSign := eval(enclosure.Interval.Lower).Sign(), eval(enclosure.Interval.Upper).Sign()
		if aSign*bSign > 0 {
			t.Fatal("independent quadratic bracket")
		}
		if len(a.TrimmedAway) == 0 {
			t.Fatal("missing removed cone/plane contacts")
		}
	}
	b := line(r(2, 1), r(0, 1), r(1, 1))
	if len(b.Hits) != 2 || b.Hits[0].Faces[0] != q.cuts[1].floor || b.Hits[1].Faces[0] != q.cuts[2].floor || b.Hits[0].Kind != "entry" || b.Hits[1].Kind != "exit" {
		t.Fatal("opposite blind floors", b)
	}
	empty := line(r(-2, 1), r(0, 1), r(1, 1))
	if len(empty.Hits) != 0 || len(empty.TrimmedAway) < 2 {
		t.Fatal("through void", empty)
	}
	tangent := line(r(49, 4), r(0, 1), r(1, 1))
	if len(tangent.Hits) != 1 || tangent.Hits[0].Kind != "tangent" || tangent.Hits[0].Faces[0] != q.fillets.Blends[1] {
		t.Fatal("outer torus tangent", tangent)
	}
	rim := line(r(123, 16), r(0, 1), r(1, 1))
	if len(rim.Hits) != 2 || len(rim.Hits[0].Faces) != 2 {
		t.Fatal("cap/blend exact root ownership", rim)
	}
	endpoint, e := q.IntersectAxialRational(ctx, r(2, 1), r(0, 1), r(0, 1), r(1, 1), r(1, 1), r(2, 1), opts)
	if e != nil || len(endpoint.Hits) != 2 || endpoint.Hits[0].Kind != "endpoint" || endpoint.Hits[0].BeforeKnown || endpoint.Hits[1].AfterKnown {
		t.Fatal("finite endpoints", endpoint, e)
	}
	// Nonbinary original line units and exact endpoint identity.
	scaled, e := q.IntersectAxialRational(ctx, r(2, 1), r(0, 1), r(1, 3), r(2, 3), r(-1, 1), r(7, 1), opts)
	if e != nil || len(scaled.Hits) != 2 {
		t.Fatal(e)
	}
	v, e := scaled.Hits[0].Parameter.Classify(ctx, opts)
	if e != nil || v.Rational == nil || v.Rational.Cmp(r(1, 1)) != 0 {
		t.Fatal("raw parameter", v, e)
	}
	for _, xy := range [][2]*big.Rat{{r(1, 1), r(1, 1)}, {r(-3, 2), r(0, 1)}} {
		bad, e := q.IntersectAxialRational(ctx, xy[0], xy[1], r(0, 1), r(1, 1), r(-1, 1), r(7, 1), opts)
		if e == nil || bad.Hits != nil || bad.TrimmedAway != nil {
			t.Fatal("unsupported radial/coincident", e)
		}
	}
	bad, e := q.IntersectAxialRational(ctx, r(8, 1), r(0, 1), r(0, 1), r(1, 1), r(-1, 1), r(7, 1), geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || bad.Hits != nil {
		t.Fatal("atomic budget")
	}
}
