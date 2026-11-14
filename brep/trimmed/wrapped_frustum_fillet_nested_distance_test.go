package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumNestedToroidDistance(t *testing.T) {
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
	q, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	opts := FilletedFrustumNestedDistanceOptions{Original: FilletedFrustumToroidDistanceOptions{Rims: FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}}}
	var first FilletedFrustumNestedDistance
	for _, tc := range []struct {
		p      [3]*big.Rat
		end    int
		square int64
		where  geom.BorePointLocation
	}{{[3]*big.Rat{r(8, 1), r(1, 1), r(3, 16)}, 0, 65, geom.BoreInside}, {[3]*big.Rat{r(8, 1), r(2, 1), r(-1, 16)}, 0, 68, geom.BoreOutside}, {[3]*big.Rat{r(11, 1), r(3, 1), r(41, 8)}, 1, 130, geom.BoreInside}} {
		got, e := q.BoundBoundaryNestedDistanceRational(ctx, tc.p, opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.Nested == nil || !got.NearestProven || !got.Converged || got.NestedChecks != 2 || got.Upper.Cmp(got.Baseline.Upper) >= 0 || got.Baseline.Toroid != nil {
			t.Fatal("genuine nested improvement", got)
		}
		n := got.Nested
		if n.Face != q.fillets.Blends[tc.end] || n.AzimuthSquared.Cmp(r(tc.square, 1)) != 0 || n.AzimuthRoot[0].Cmp(n.AzimuthRoot[1]) == 0 {
			t.Fatal(n)
		}
		if fdSquare(n.AzimuthRoot[0]).Cmp(n.AzimuthSquared) > 0 || fdSquare(n.AzimuthRoot[1]).Cmp(n.AzimuthSquared) < 0 {
			t.Fatal("root enclosure", n)
		}
		material, e := q.ClassifyRational(ctx, tc.p)
		if e != nil || material.Location != tc.where {
			t.Fatal(material, e)
		}
		if tc.square == 65 {
			first = got
		}
	}
	// Independent exact decimal brackets, checked by rational squaring.
	decimal := func(s string) *big.Rat {
		v, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal(s)
		}
		return v
	}
	rhoLo, rhoHi := decimal("8.062257748298549652366613230303"), decimal("8.062257748298549652366613230304")
	if fdSquare(rhoLo).Cmp(r(65, 1)) > 0 || fdSquare(rhoHi).Cmp(r(65, 1)) < 0 {
		t.Fatal("oracle sqrt65")
	}
	// d²=31913/256-(123/8)*sqrt65, independently expanded from the
	// original radius123/16 and query meridian height offset-3/4.
	A, B, S, rr := r(31913, 256), r(123, 8), r(65, 1), r(225, 256)
	Dlo, Dhi := fdSub(A, fdMul(B, rhoHi)), fdSub(A, fdMul(B, rhoLo))
	dLo, dHi := decimal("0.838417181306418060968522516"), decimal("0.838417181306418060968522517")
	if fdSquare(dLo).Cmp(Dlo) > 0 || fdSquare(dHi).Cmp(Dhi) < 0 {
		t.Fatal("independent nested root", Dlo, Dhi)
	}
	truthLo, truthHi := fdSquare(fdSub(r(15, 16), dHi)), fdSquare(fdSub(r(15, 16), dLo))
	if first.Lower.Cmp(truthLo) > 0 || first.Upper.Cmp(truthHi) < 0 {
		t.Fatal("independent distance", first.Lower, first.Upper, truthLo, truthHi)
	}
	// Eliminate both radicals independently. For t=(sqrt(D)-r)^2:
	// C(t)+E(t)*sqrt65=0, so C(t)^2-65E(t)^2=0 (quartic).
	polynomial := func(t *big.Rat) *big.Rat {
		sum := fdAdd(rr, t)
		C := fdAdd(fdAdd(fdSquare(A), fdMul(fdSquare(B), S)), fdSquare(fdSub(rr, t)))
		C.Sub(C, fdMul(r(2, 1), fdMul(A, sum)))
		E := fdMul(fdMul(r(2, 1), B), fdSub(sum, A))
		return fdSub(fdSquare(C), fdMul(S, fdSquare(E)))
	}
	if polynomial(first.Lower).Sign()*polynomial(first.Upper).Sign() > 0 {
		t.Fatal("independent quartic root")
	}
	p := [3]*big.Rat{r(8, 1), r(1, 1), r(3, 16)}
	coarseOpts := opts
	coarseOpts.Original.Rims.Support.SqrtBits = 8
	coarse, e := q.BoundBoundaryNestedDistanceRational(ctx, p, coarseOpts)
	if e != nil {
		t.Fatal(e)
	}
	if first.Lower.Cmp(coarse.Lower) < 0 || first.Upper.Cmp(coarse.Upper) > 0 {
		t.Fatal("nonmonotone", coarse, first)
	}
	first.Nested.Center[0].SetInt64(100)
	first.Nested.MajorRadius.SetInt64(100)
	again, e := q.BoundBoundaryNestedDistanceRational(ctx, p, opts)
	if e != nil || again.Nested.MajorRadius.Cmp(r(123, 16)) != 0 {
		t.Fatal("owned witness", e)
	}
	// Inner sheet and trim-exterior full-torus projections must not pass.
	for _, bad := range [][3]*big.Rat{{r(7, 1), r(1, 1), r(3, 16)}, {r(8, 1), r(1, 1), r(15, 16)}} {
		v, e := q.BoundBoundaryNestedDistanceRational(ctx, bad, opts)
		if e != nil {
			t.Fatal(e)
		}
		if v.Nested != nil {
			t.Fatal("false retained band", v)
		}
	}
	rational, e := q.BoundBoundaryNestedDistanceRational(ctx, [3]*big.Rat{r(77, 16), r(77, 12), r(3, 16)}, opts)
	if e != nil || rational.Nested != nil || rational.Baseline.Toroid == nil || !rational.NearestProven {
		t.Fatal("strict API preserved", rational, e)
	}
	limited := opts
	limited.MaxNestedChecks = 1
	if v, e := q.BoundBoundaryNestedDistanceRational(ctx, p, limited); e == nil || v.Lower != nil || v.Nested != nil || v.Baseline.Lower != nil {
		t.Fatal("atomic budget", v, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := q.BoundBoundaryNestedDistanceRational(cancelled, p, opts); e != context.Canceled || v.Lower != nil {
		t.Fatal(v, e)
	}
}
