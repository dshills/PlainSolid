package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
	"testing"
)

func TestFilletedFrustumWorldNearestTies(t *testing.T) {
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
	var encoded bytes.Buffer
	if e = WriteFilletedFrustumMaterialJSON(ctx, &encoded, q); e != nil {
		t.Fatal(e)
	}
	q, e = ReadFilletedFrustumMaterialJSON(ctx, &encoded, FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	m := [3][4]*big.Rat{{r(3, 13), r(-12, 13), r(4, 13), r(1, 7)}, {r(36, 65), r(5, 13), r(48, 65), r(-2, 9)}, {r(-4, 5), r(0, 1), r(3, 5), r(3, 11)}}
	pose, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		t.Fatal(e)
	}
	placed, e := q.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	world := func(p [3]*big.Rat) [3]*big.Rat {
		var v [3]*big.Rat
		for i := range v {
			v[i] = new(big.Rat).Set(m[i][3])
			for j := range p {
				v[i].Add(v[i], fdMul(m[i][j], p[j]))
			}
		}
		return v
	}

	opts := FilletedFrustumTieOptions{}
	c := q.fillets.Circles[0]
	for _, local := range [][3]*big.Rat{{r(2, 1), r(0, 1), r(1, 2)}, {r(2, 1), r(0, 1), r(3, 2)}, {fdMul(c.Center[0], r(3, 5)), fdMul(c.Center[0], r(4, 5)), c.Center[1]}, {r(8, 1), r(1, 1), r(3, 16)}} {
		original, e := q.ClassifyNearestFacesRational(ctx, local, opts)
		if e != nil {
			t.Fatal(e)
		}
		p := world(local)
		got, e := placed.ClassifyNearestFacesRational(ctx, p, opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.Exact != original.Exact || got.Complete != original.Complete || len(got.Tied) != len(original.Tied) || len(got.Excluded) != len(original.Excluded) || len(got.Unresolved) != len(original.Unresolved) || got.Lower.Cmp(original.Baseline.Lower) != 0 || got.Upper.Cmp(original.Baseline.Upper) != 0 || got.Original.RetainedCandidates != original.RetainedCandidates || got.Original.Baseline.SupportChecks != original.Baseline.SupportChecks {
			t.Fatal("changed source proof")
		}
		for i, f := range got.Excluded {
			if f != original.Excluded[i] {
				t.Fatal("excluded owner")
			}
		}
		for i, f := range got.Unresolved {
			if f != original.Unresolved[i] {
				t.Fatal("unresolved owner")
			}
		}
		for i, tie := range got.Tied {
			if tie.Face != original.Tied[i].Face {
				t.Fatal("native owner")
			}
			want := world(original.Tied[i].Witness)
			d := new(big.Rat)
			for j := range want {
				if tie.Witness[j].Cmp(want[j]) != 0 {
					t.Fatal("exact placed witness")
				}
				d.Add(d, fdSquare(fdSub(p[j], tie.Witness[j])))
			}
			if d.Cmp(got.Upper) != 0 {
				t.Fatal("world attained distance")
			}
			state, e := placed.ClassifyRational(ctx, tie.Witness)
			if e != nil || state.Location != geom.BoreBoundary {
				t.Fatal("world retention", e)
			}
			owns := false
			for _, f := range state.Faces {
				owns = owns || f == tie.Face
			}
			if !owns {
				t.Fatal("world false owner")
			}
		}
		if len(got.Tied) > 0 {
			got.Tied[0].Witness[0].SetInt64(999)
			if got.Original.Tied[0].Witness[0].Cmp(r(999, 1)) == 0 {
				t.Fatal("source/world alias")
			}
		}
		got.Lower.SetInt64(-1)
		if got.Original.Baseline.Lower.Sign() < 0 {
			t.Fatal("bound alias")
		}
	}
	// A different placement cannot reuse this pose's apparent floor ties.
	shiftedMatrix := m
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			shiftedMatrix[i][j] = new(big.Rat).Set(m[i][j])
		}
	}
	shiftedMatrix[0][3].Add(shiftedMatrix[0][3], r(100, 1))
	shiftedPose, e := placement.NewRationalRigid(shiftedMatrix, 4096)
	if e != nil {
		t.Fatal(e)
	}
	shifted, e := q.PlaceRigid(ctx, shiftedPose)
	if e != nil {
		t.Fatal(e)
	}
	p := world([3]*big.Rat{r(2, 1), r(0, 1), r(3, 2)})
	other, e := shifted.ClassifyNearestFacesRational(ctx, p, opts)
	if e != nil {
		t.Fatal(e)
	}
	if other.Upper.Cmp(r(1, 4)) == 0 || other.Lower.Cmp(r(1, 4)) <= 0 {
		t.Fatal("mixed occurrence distance")
	}
	// A distinct immutable source with a shallower lower cut loses the lower
	// floor tie even though the original integer IDs can coincide.
	different, e := PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	differentPlaced, e := different.PlaceRigid(ctx, pose)
	if e != nil {
		t.Fatal(e)
	}
	changed, e := differentPlaced.ClassifyNearestFacesRational(ctx, p, opts)
	if e != nil || !changed.Complete || len(changed.Tied) != 1 || changed.Tied[0].Face != different.cuts[2].floor {
		t.Fatal("mixed source false tie", changed, e)
	}
	// One-candidate exhaustion must not leak any world ties.
	bad, e := placed.ClassifyNearestFacesRational(ctx, p, FilletedFrustumTieOptions{MaxCandidates: 1})
	if e == nil || len(bad.Tied) != 0 || bad.Lower != nil {
		t.Fatal("atomic candidate limit")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e = placed.ClassifyNearestFacesRational(cancelled, p, opts)
	if e == nil || bad.Lower != nil || len(bad.Tied) != 0 {
		t.Fatal("atomic cancellation")
	}
}
