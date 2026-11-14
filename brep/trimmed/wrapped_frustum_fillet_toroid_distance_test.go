package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumAlgebraicToroidDistance(t *testing.T) {
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
	opts := FilletedFrustumToroidDistanceOptions{Rims: FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}}
	for _, tc := range []struct {
		end                   int
		a, b, radicand, A, B2 *big.Rat
		where                 geom.BorePointLocation
	}{{0, r(1, 3), r(-3, 4), r(97, 144), r(3577, 2304), r(2425, 1024), geom.BoreInside}, {0, r(1, 2), r(-1, 1), r(5, 4), r(545, 256), r(1125, 256), geom.BoreOutside}, {1, r(1, 1), r(1, 1), r(2, 1), r(353, 64), r(225, 8), geom.BoreInside}} {
		c := q.fillets.Circles[tc.end]
		rho := fdAdd(c.Center[0], tc.a)
		// Non-axis rational azimuth(3/5,4/5) and irrational meridian normal.
		p := [3]*big.Rat{fdMul(rho, r(3, 5)), fdMul(rho, r(4, 5)), fdAdd(c.Center[1], tc.b)}
		got, e := q.BoundBoundaryToroidDistanceRational(ctx, p, opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.Toroid == nil || !got.NearestProven || !got.Converged || got.ToroidChecks != 2 || got.Upper.Cmp(got.Baseline.Upper) >= 0 {
			t.Fatalf("missing stationary improvement %+v", got)
		}
		if got.Toroid.Face != q.fillets.Blends[tc.end] || got.Toroid.Radicand.Cmp(tc.radicand) != 0 {
			t.Fatal(got.Toroid)
		}
		// Independent distance expressions:3577/2304-5sqrt(97)/32 and
		// 545/256-15sqrt(5)/16, and353/64-15sqrt(2)/4: smaller roots.
		poly := func(x *big.Rat) *big.Rat { return fdSub(fdSquare(fdSub(x, tc.A)), tc.B2) }
		if got.Upper.Cmp(tc.A) >= 0 || poly(got.Lower).Sign() < 0 || poly(got.Upper).Sign() > 0 || got.Lower.Cmp(got.Upper) >= 0 {
			t.Fatal(got.Lower, got.Upper)
		}
		classified, e := q.ClassifyRational(ctx, p)
		if e != nil || classified.Location != tc.where {
			t.Fatal(classified, e)
		}
		// The radical numerator has length r*sqrt(D), proving the point lies
		// on the meridian circle, independent of the distance interval values.
		length := new(big.Rat)
		for _, v := range got.Toroid.Numerator {
			length.Add(length, fdSquare(v))
		}
		if length.Cmp(fdMul(fdSquare(c.Radius), tc.radicand)) != 0 {
			t.Fatal("circle identity")
		}
		z0, z1 := q.z[0], c.Tangency[1]
		if tc.end == 1 {
			z0, z1 = c.Tangency[1], q.z[1]
		}
		if fdRadicalSign(got.Toroid.Numerator[2], fdSub(z0, got.Toroid.Center[2]), tc.radicand) <= 0 || fdRadicalSign(got.Toroid.Numerator[2], fdSub(z1, got.Toroid.Center[2]), tc.radicand) >= 0 {
			t.Fatal("trim retention")
		}
		// Exact cross-multiplied stationarity in meridian coordinates: the
		// projected normal and displacement from the circle center are parallel.
		if fdSub(fdMul(got.Toroid.Numerator[2], tc.a), fdMul(fdMul(c.Radius, tc.a), tc.b)).Sign() != 0 {
			t.Fatal("stationarity")
		}
		got.Toroid.Center[0].SetInt64(100)
		again, e := q.BoundBoundaryToroidDistanceRational(ctx, p, opts)
		if e != nil || again.Toroid.Center[0].Cmp(r(100, 1)) == 0 {
			t.Fatal("ownership", e)
		}
	}
	c := q.fillets.Circles[0]
	// Full torus projection on inner half, or beyond the retained arc: neither
	// is promoted to a valid source-band witness. Irrational azimuth likewise
	// remains explicitly outside this new stationary projection subset.
	for _, p := range [][3]*big.Rat{{fdSub(c.Center[0], r(1, 10)), r(0, 1), fdSub(c.Center[1], r(3, 4))}, {fdAdd(c.Center[0], r(1, 1)), r(0, 1), c.Center[1]}, {r(1, 1), r(1, 1), r(3, 1)}} {
		got, e := q.BoundBoundaryToroidDistanceRational(ctx, p, opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.Toroid != nil {
			t.Fatal("unproved trim", got.Toroid)
		}
	}
	p := [3]*big.Rat{r(77, 16), r(77, 12), r(3, 16)}
	bad := opts
	bad.MaxToroidChecks = 1
	if v, e := q.BoundBoundaryToroidDistanceRational(ctx, p, bad); e == nil || v.Lower != nil || v.Toroid != nil || v.Baseline.Lower != nil {
		t.Fatal("atomic budget", v, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := q.BoundBoundaryToroidDistanceRational(cancelled, p, opts); e != context.Canceled || v.Lower != nil {
		t.Fatal(v, e)
	}
}

func TestFilletedRadicalSign(t *testing.T) {
	r := big.NewRat
	for _, c := range []struct {
		a, b, d *big.Rat
		sign    int
	}{{r(1, 1), r(1, 1), r(2, 1), -1}, {r(-1, 1), r(-1, 1), r(2, 1), 1}, {r(1, 1), r(-1, 1), r(2, 1), 1}, {r(-1, 1), r(1, 1), r(2, 1), -1}, {r(3, 1), r(1, 1), r(9, 1), 0}, {r(-3, 1), r(-1, 1), r(9, 1), 0}, {r(0, 1), r(-1, 1), r(2, 1), 1}, {r(-1, 1), r(1, 1), r(0, 1), -1}} {
		if got := fdRadicalSign(c.a, c.b, c.d); got != c.sign {
			t.Fatal(c, got)
		}
	}
}
