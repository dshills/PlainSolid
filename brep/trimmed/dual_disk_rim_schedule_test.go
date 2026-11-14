package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"sort"
	"testing"
	"time"
)

func TestAuthoredRimScheduleQuarterArcs(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	ds := [2]ExtrusionCircle{{Center: geom.Vec2{X: 7, Y: -3}, Radius: 5}, {Center: geom.Vec2{X: 30, Y: -20}, Radius: 29}}
	cut, e := CutUnequalCenterSideOpenDualDisks(ctx, f, geom.Vec2{}, geom.Vec2{X: 60, Y: 10}, ds, [2]*big.Rat{big.NewRat(10, 1), big.NewRat(1, 1)}, 6, 6, 1e-10, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	ideal := 3600 - 6*(216.5*math.Pi-409)
	for _, schedule := range [][]int{nil, {64, 96}} {
		start := time.Now()
		r, e := cut.TessellateCertified(ctx, SideOpenDualDiskMeshOptions{Deflection: .5, AngularTurn: .025, RimSchedule: schedule})
		if e != nil {
			t.Fatal(schedule, e)
		}
		elapsed := time.Since(start)
		expected := 128
		if len(schedule) > 0 {
			expected = 96
		}
		if r.RimIntervals != expected {
			t.Fatal("wrong schedule", r.RimIntervals)
		}
		// Sum per-chord exact r²*tan(theta)^3/12. theta-sin(theta)<=theta³/6
		// and theta<=tan(theta) bound the removed circular segment area.
		bound := new(big.Rat)
		for _, ids := range cut.WallFaces {
			fi := ids[0]
			wall, e := proveDualDiskWall(ctx, cut.Solid.faces[fi])
			if e != nil {
				t.Fatal(e)
			}
			var us []*big.Rat
			seen := map[string]bool{}
			for _, uv := range r.Network.Faces[fi].ExactUV {
				if uv[1].Sign() != 0 && uv[1].Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("not axial two-row fixture")
				}
				key := uv[0].RatString()
				if !seen[key] {
					seen[key] = true
					us = append(us, uv[0])
				}
			}
			sort.Slice(us, func(i, j int) bool { return us[i].Cmp(us[j]) < 0 })
			for i := 1; i < len(us); i++ {
				a, e := wall.radial(ctx, us[i-1])
				if e != nil {
					t.Fatal(e)
				}
				b, e := wall.radial(ctx, us[i])
				if e != nil {
					t.Fatal(e)
				}
				cross := ddCross(a, b)
				tangent := new(big.Rat).Quo(new(big.Rat).Abs(cross[wall.axis]), ddDot(a, b))
				term := new(big.Rat).Mul(tangent, tangent)
				term.Mul(term, tangent)
				term.Mul(term, ddDot(a, a))
				bound.Add(bound, term)
			}
		}
		bound.Quo(bound, big.NewRat(2, 1)) // depth6 /12
		polygonVolume := new(big.Rat)
		for _, face := range r.Network.Faces {
			for _, tri := range face.Mesh.Triangles {
				polygonVolume.Add(polygonVolume, ddDot(face.ExactPoints[tri[0]], ddCross(face.ExactPoints[tri[1]], face.ExactPoints[tri[2]])))
			}
		}
		polygonVolume.Quo(polygonVolume, big.NewRat(6, 1))
		rounding := new(big.Rat).Abs(new(big.Rat).Sub(r.DisplayVolume, polygonVolume))
		bound.Add(bound, rounding)
		upper, _ := bound.Float64()
		v, _ := r.DisplayVolume.Float64()
		if v < ideal-1e-8 || v-ideal > upper+1e-8 || upper >= .3 {
			t.Fatal("independent segment-volume bound", v-ideal, upper)
		}
		if !r.DisplayCollision.Complete || len(r.DisplayCollision.Contacts) != 0 {
			t.Fatal("missing collision")
		}
		t.Log("schedule", schedule, "intervals", r.RimIntervals, "triangles", len(r.Mesh.Triangles), "volume error", v-ideal, "bound", upper, "elapsed", elapsed)
	}
	if r, e := cut.TessellateCertified(ctx, SideOpenDualDiskMeshOptions{Deflection: .5, AngularTurn: .025, RimSchedule: []int{64}}); e == nil || len(r.Mesh.Vertices) != 0 {
		t.Fatal("insufficient schedule published")
	}
}

func TestDualDiskRimScheduleLimits(t *testing.T) {
	for _, s := range [][]int{{0}, {-1}, {2, 1}, {2, 2}, {257}, {64, 128, 192, 256}, {1, 2, 3, 4, 5, 6, 7, 8, 9, 10}} {
		if _, e := dualDiskRimSchedule(s, 256); e == nil {
			t.Fatal(s)
		}
	}
	if _, e := dualDiskRimSchedule([]int{96}, 64); e == nil {
		t.Fatal("interval cap")
	}
	s := []int{64, 96}
	r, e := dualDiskRimSchedule(s, 256)
	if e != nil {
		t.Fatal(e)
	}
	s[0] = 1
	if r[0] != 64 {
		t.Fatal("caller alias")
	}
}
