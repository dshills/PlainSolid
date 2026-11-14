package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumNearestTies(t *testing.T) {
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

	opts := FilletedFrustumTieOptions{}
	cases := []struct {
		p     [3]*big.Rat
		faces []int
		d     *big.Rat
	}{
		{[3]*big.Rat{r(2, 1), r(0, 1), r(1, 2)}, []int{q.cuts[1].wall, q.cuts[1].floor}, r(1, 4)},
		{[3]*big.Rat{r(-2, 1), r(0, 1), r(-1, 8)}, []int{q.cuts[0].wall, q.fillets.Caps[0]}, r(17, 64)},
		{[3]*big.Rat{r(2, 1), r(0, 1), r(3, 2)}, []int{q.cuts[1].floor, q.cuts[2].floor}, r(1, 4)},
	}
	c := q.fillets.Circles[0]
	cases = append(cases, struct {
		p     [3]*big.Rat
		faces []int
		d     *big.Rat
	}{[3]*big.Rat{fdMul(c.Center[0], r(3, 5)), fdMul(c.Center[0], r(4, 5)), c.Center[1]}, []int{q.fillets.Blends[0], q.fillets.Caps[0], q.fillets.Lateral}, fdSquare(c.Radius)})
	for _, tc := range cases {
		got, e := q.ClassifyNearestFacesRational(ctx, tc.p, opts)
		if e != nil || !got.Exact || !got.Complete || len(got.Tied) != len(tc.faces) || got.Baseline.Upper.Cmp(tc.d) != 0 {
			t.Fatal(got, e)
		}
		want := map[int]bool{}
		for _, f := range tc.faces {
			want[f] = true
		}
		seen := map[int]bool{}
		for _, tie := range got.Tied {
			if !want[tie.Face] || seen[tie.Face] {
				t.Fatal("wrong tied owner")
			}
			seen[tie.Face] = true
			state, e := q.ClassifyRational(ctx, tie.Witness)
			if e != nil || state.Location != geom.BoreBoundary {
				t.Fatal(e)
			}
			owns := false
			for _, f := range state.Faces {
				owns = owns || f == tie.Face
			}
			if !owns {
				t.Fatal("unretained witness")
			}
			d := new(big.Rat)
			for i := range tc.p {
				d.Add(d, fdSquare(fdSub(tc.p[i], tie.Witness[i])))
			}
			if d.Cmp(tc.d) != 0 {
				t.Fatal("not attained")
			}
		}
		for _, f := range got.Excluded {
			if seen[f] {
				t.Fatal("duplicate face")
			}
			seen[f] = true
		}
		if len(seen) != len(q.solid.faces) {
			t.Fatal("incomplete original ledger")
		}
		direct, e := q.BoundBoundaryDistanceRational(ctx, tc.p, opts.Support)
		if e != nil || direct.Lower.Cmp(got.Baseline.Lower) != 0 || direct.Upper.Cmp(got.Baseline.Upper) != 0 || direct.SupportChecks != got.Baseline.SupportChecks {
			t.Fatal("legacy query changed")
		}
		if got.RetainedCandidates > 1 {
			low := opts
			low.MaxCandidates = got.RetainedCandidates - 1
			bad, e := q.ClassifyNearestFacesRational(ctx, tc.p, low)
			if e == nil || len(bad.Tied) != 0 || bad.Baseline.Lower != nil {
				t.Fatal("atomic candidate limit")
			}
		}
	}
	// An irrational nearest distance cannot produce a complete rational tie ledger.
	unknown, e := q.ClassifyNearestFacesRational(ctx, [3]*big.Rat{r(8, 1), r(1, 1), r(3, 16)}, opts)
	if e != nil || unknown.Exact || unknown.Complete || len(unknown.Tied) != 0 || len(unknown.Unresolved) != len(q.solid.faces) {
		t.Fatal("unproved tie promotion", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := q.ClassifyNearestFacesRational(cancelled, cases[0].p, opts)
	if e == nil || bad.Baseline.Lower != nil || len(bad.Tied) != 0 {
		t.Fatal("atomic cancellation")
	}
}
