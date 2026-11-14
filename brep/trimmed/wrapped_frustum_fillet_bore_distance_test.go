package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumBoreDistance(t *testing.T) {
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

	opts := FilletedFrustumBoreDistanceOptions{Original: FilletedFrustumNestedDistanceOptions{Original: FilletedFrustumToroidDistanceOptions{Rims: FilletedFrustumRimDistanceOptions{Support: FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}}}}
	for ci, c := range q.cuts {
		z := new(big.Rat).Quo(fdAdd(c.z[0], c.z[1]), r(2, 1))
		for _, offset := range []*big.Rat{r(1, 10), r(1, 2)} {
			p := [3]*big.Rat{fdAdd(c.center[0], offset), fdAdd(c.center[1], offset), z}
			got, e := q.BoundBoundaryBoreDistanceRational(ctx, p, opts)
			if e != nil {
				t.Fatal(e)
			}
			if got.Bore == nil || got.Bore.Face != c.wall || !got.NearestProven || !got.Converged || got.BoreChecks != len(q.cuts) {
				t.Fatalf("cut%d offset%s: %+v", ci, offset, got)
			}
			w := got.Bore
			S := fdMul(r(2, 1), fdSquare(offset))
			if w.Radicand.Cmp(S) != 0 || fdSquare(w.Root[0]).Cmp(S) > 0 || fdSquare(w.Root[1]).Cmp(S) < 0 || w.Center[2].Cmp(z) != 0 || w.Numerator[2].Sign() != 0 {
				t.Fatal("exact root/height")
			}
			// Independent original cylinder equation: numerator norm²=r²*S.
			if fdAdd(fdSquare(w.Numerator[0]), fdSquare(w.Numerator[1])).Cmp(fdMul(fdSquare(c.radius), S)) != 0 {
				t.Fatal("cylinder support")
			}
			// Dist²=S+1/4-sqrt(S) for r=1/2. Exact signs bracket its smaller root.
			A := fdAdd(S, r(1, 4))
			poly := func(x *big.Rat) *big.Rat { return fdSub(fdSquare(fdSub(A, x)), S) }
			if got.Upper.Cmp(A) >= 0 || poly(got.Lower).Sign() < 0 || poly(got.Upper).Sign() > 0 {
				t.Fatal("independent distance polynomial")
			}
			state, e := q.ClassifyRational(ctx, p)
			if e != nil {
				t.Fatal(e)
			}
			want := geom.BoreOutside
			if offset.Cmp(r(1, 2)) == 0 {
				want = geom.BoreInside
			}
			if state.Location != want {
				t.Fatal("void/stock state", state)
			}
			w.Center[0].SetInt64(999)
			w.Root[0].SetInt64(999)
			fresh, e := q.BoundBoundaryBoreDistanceRational(ctx, p, opts)
			if e != nil || fresh.Bore == nil || fresh.Bore.Center[0].Cmp(r(999, 1)) == 0 {
				t.Fatal("owned witness", e)
			}
		}
		for _, height := range []*big.Rat{c.z[0], c.z[1], fdSub(c.z[0], r(1, 8)), fdAdd(c.z[1], r(1, 8))} {
			p := [3]*big.Rat{fdAdd(c.center[0], r(1, 10)), fdAdd(c.center[1], r(1, 10)), height}
			got, e := q.BoundBoundaryBoreDistanceRational(ctx, p, opts)
			if e != nil {
				t.Fatal(e)
			}
			if got.Bore != nil && got.Bore.Face == c.wall {
				t.Fatal("unretained cap/floor projection")
			}
		}
	}
	// A blind floor closer than the wall must keep its exact rational witness.
	floorQuery := [3]*big.Rat{r(21, 10), r(1, 10), r(99, 100)}
	floorResult, e := q.BoundBoundaryBoreDistanceRational(ctx, floorQuery, opts)
	if e != nil || floorResult.Bore != nil || !floorResult.NearestProven || floorResult.Lower.Cmp(r(1, 10000)) != 0 || floorResult.Upper.Cmp(r(1, 10000)) != 0 {
		t.Fatal("closer blind floor", floorResult, e)
	}
	p := [3]*big.Rat{r(-19, 10), r(1, 10), r(3, 1)}
	badOpts := opts
	badOpts.MaxBoreChecks = 1
	bad, e := q.BoundBoundaryBoreDistanceRational(ctx, p, badOpts)
	if e == nil || bad.Lower != nil || bad.Bore != nil {
		t.Fatal("atomic budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = q.BoundBoundaryBoreDistanceRational(cancelled, p, opts)
	if e == nil || bad.Lower != nil {
		t.Fatal("atomic cancellation")
	}
}
