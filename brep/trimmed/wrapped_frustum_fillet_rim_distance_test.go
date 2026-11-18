package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestFilletedFrustumAlgebraicRimDistance(t *testing.T) {
	slowtest.Skip(t)
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
	opts := FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}
	var first FilletedFrustumRimDistance
	for _, tc := range []struct {
		p         [3]*big.Rat
		cap, wall int
	}{{[3]*big.Rat{r(-19, 10), r(1, 10), r(-1, 8)}, q.fillets.Caps[0], q.cuts[0].wall}, {[3]*big.Rat{r(21, 10), r(1, 10), r(49, 8)}, q.fillets.Caps[1], q.cuts[2].wall}} {
		got, e := q.BoundBoundaryRimDistanceRational(ctx, tc.p, opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.Rim == nil || !got.NearestProven || !got.Converged || got.RimChecks != 10 || got.Upper.Cmp(got.Baseline.Upper) >= 0 {
			t.Fatalf("missing irrational refinement %+v", got)
		}
		if !reflect.DeepEqual(got.Rim.Faces, []int{tc.cap, tc.wall}) || len(got.Rim.RimEdges) == 0 {
			t.Fatal(got.Rim)
		}
		// Independent analytic truth: D=457/1600-sqrt(1/50). The interval
		// lies on the smaller root of (D-457/1600)^2-1/50, strictly irrational.
		polynomial := func(x *big.Rat) *big.Rat { return fdSub(fdSquare(fdSub(x, r(457, 1600))), r(1, 50)) }
		if got.Lower.Cmp(r(457, 1600)) >= 0 || got.Upper.Cmp(r(457, 1600)) >= 0 || polynomial(got.Lower).Sign() < 0 || polynomial(got.Upper).Sign() > 0 || got.Lower.Cmp(got.Upper) >= 0 {
			t.Fatal(got.Lower, got.Upper)
		}
		root := got.Rim.AzimuthRoot
		if fdSquare(root[0]).Cmp(r(1, 50)) > 0 || fdSquare(root[1]).Cmp(r(1, 50)) < 0 || root[0].Sign() <= 0 {
			t.Fatal(root)
		}
		// Original edge IDs actually have both reported owners, not an inferred
		// geometric near-contact. Each rim edge has exactly these two coedges.
		for _, edge := range got.Rim.RimEdges {
			var owners []int
			for fi, f := range q.solid.faces {
				loops := append([][]Use{f.Outer}, f.Holes...)
				for _, loop := range loops {
					for _, u := range loop {
						if u.Edge == edge {
							owners = append(owners, fi)
						}
					}
				}
			}
			if !reflect.DeepEqual(owners, got.Rim.Faces) {
				t.Fatal(edge, owners, got.Rim.Faces)
			}
		}
		first = got
	}
	// Returned algebraic/rational fields do not alias the prepared proof.
	first.Rim.Radius.SetInt64(99)
	first.Rim.Center[0].SetInt64(99)
	p := [3]*big.Rat{r(-19, 10), r(1, 10), r(-1, 8)}
	again, e := q.BoundBoundaryRimDistanceRational(ctx, p, opts)
	if e != nil || again.Rim.Radius.Cmp(r(1, 2)) != 0 {
		t.Fatal(again, e)
	}
	coarse, e := q.BoundBoundaryRimDistanceRational(ctx, p, FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SqrtBits: 8}})
	if e != nil {
		t.Fatal(e)
	}
	if again.Lower.Cmp(coarse.Lower) < 0 || again.Upper.Cmp(coarse.Upper) > 0 {
		t.Fatal("nonmonotone enclosures", again, coarse)
	}
	if v, e := q.BoundBoundaryRimDistanceRational(ctx, p, FilletedFrustumRimDistanceOptions{MaxRimChecks: 9}); e == nil || v.Rim != nil || v.Lower != nil || v.Baseline.Lower != nil {
		t.Fatal("atomic rim budget", v, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if v, e := q.BoundBoundaryRimDistanceRational(cancelled, p, opts); e != context.Canceled || v.Lower != nil {
		t.Fatal(v, e)
	}
	// The same azimuth inside the through bore, midway up the stock, is
	// closest to its wall rather than an entry rim. Do not assert rim nearest.
	p[2] = r(3, 1)
	v, e := q.BoundBoundaryRimDistanceRational(ctx, p, opts)
	if e != nil {
		t.Fatal(e)
	}
	if v.Rim != nil && v.NearestProven {
		t.Fatal("interior falsely classified as entry", v)
	}
}
