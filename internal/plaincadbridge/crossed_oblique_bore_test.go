package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestCrossedObliqueBoreScope(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 16, 40})
	cardinal := circle(t, b, vector{10, 8, -2}, vector{0, 0, 1}, 2, 44)
	oblique := circle(t, b, vector{-4, 8, 9.5}, vector{4, 0, 3}, 2, 35)
	one := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": cardinal.Handle})
	near(t, overlapMeasure(t, b, one.Handle, oblique.Handle), 100*math.Pi-160./3)
	out := shape(t, b, map[string]any{"op": "cut", "a": one.Handle, "b": oblique.Handle})
	near(t, out.GeometryAssertions.Volume, 12800-260*math.Pi+160./3)
	near(t, out.GeometryAssertions.SurfaceArea, 3520+242*math.Pi-80)
	group, err := b.cutMany(stock.Handle, []string{oblique.Handle, cardinal.Handle})
	if err != nil || group.GeometryAssertions != out.GeometryAssertions || group.Bounds != out.Bounds {
		t.Fatal(group, out, err)
	}
	near(t, overlapMeasure(t, b, group.Handle, oblique.Handle), 0)
	body := b.shapes[group.Handle].bore
	cap, err := body.planarFace(geom.Vec3{}, geom.Vec3{X: 1})
	if err != nil {
		t.Fatal(err)
	}
	near(t, cap.Area, 640-5*math.Pi)
	file, err := b.exportSTEP([]string{group.Handle})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(file.Base64)
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.importSTEP(string(raw))
	if err != nil || got.Bounds != out.Bounds {
		t.Fatal(got, err)
	}
	near(t, got.GeometryAssertions.Volume, out.GeometryAssertions.Volume)
	near(t, got.GeometryAssertions.SurfaceArea, out.GeometryAssertions.SurfaceArea)
	near(t, overlapMeasure(t, b, got.Handle, oblique.Handle), 0)
}

func TestCrossedObliqueReciprocalScope(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		b := New()
		stock := box(t, b, vector{}, vector{20, 16, 40})
		cardinal := circle(t, b, vector{10, 8, -2}, vector{0, 0, 1}, 2, 44)
		oblique := circle(t, b, vector{-2, 8, 20 - 16*sign}, vector{3, 0, 4 * sign}, 2, 44)
		first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": cardinal.Handle})
		near(t, overlapMeasure(t, b, first.Handle, oblique.Handle), (400./3)*math.Pi-640./9)
		out := shape(t, b, map[string]any{"op": "cut", "a": first.Handle, "b": oblique.Handle})
		near(t, out.GeometryAssertions.Volume, 12800-(880./3)*math.Pi+640./9)
		near(t, out.GeometryAssertions.SurfaceArea, 3520+272*math.Pi-320./3)
		other := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": oblique.Handle})
		reversed := shape(t, b, map[string]any{"op": "cut", "a": other.Handle, "b": cardinal.Handle})
		group, err := b.cutMany(stock.Handle, []string{oblique.Handle, cardinal.Handle})
		if err != nil || group.GeometryAssertions != out.GeometryAssertions || group.GeometryAssertions != reversed.GeometryAssertions {
			t.Fatal(group, out, reversed, err)
		}
		partial := circle(t, b, vector{1, 8, 20 - 12*sign}, vector{3, 0, 4 * sign}, 1, 4)
		near(t, overlapMeasure(t, b, group.Handle, partial.Handle), 0)
		cap, err := b.shapes[group.Handle].bore.planarFace(geom.Vec3{}, geom.Vec3{X: 1})
		if err != nil {
			t.Fatal(err)
		}
		near(t, cap.Area, 640-(20./3)*math.Pi)
		file, err := b.exportSTEP([]string{group.Handle})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.StdEncoding.DecodeString(file.Base64)
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.importSTEP(string(raw))
		if err != nil || got.Bounds != stock.Bounds {
			t.Fatal(sign, got, err)
		}
		near(t, got.GeometryAssertions.Volume, out.GeometryAssertions.Volume)
		near(t, got.GeometryAssertions.SurfaceArea, out.GeometryAssertions.SurfaceArea)
		near(t, overlapMeasure(t, b, got.Handle, oblique.Handle), 0)
	}
}

func TestCrossedObliquePairWithSeparatedOppositeZOpenings(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{30, 20, 20})
	cardinal := circle(t, b, vector{25, 4, -2}, vector{0, 0, 1}, 1, 24)
	oblique := circle(t, b, vector{1.5, 4, -2}, vector{3, 0, 4}, 1, 35)
	out, err := b.cutMany(stock.Handle, []string{oblique.Handle, cardinal.Handle})
	if err != nil || b.shapes[out.Handle].bore.disjointOblique == nil {
		t.Fatal(out, err)
	}
	near(t, out.GeometryAssertions.Volume, 12000-45*math.Pi)
}

func TestCrossedObliqueBoreCoordinatePlacements(t *testing.T) {
	for _, axes := range [][2]geom.Vec3{
		{{Z: 1}, {Y: 1}}, {{X: 1}, {Y: 1}}, {{Y: 1}, {X: 1}}, {{Z: 1}, {X: 1}},
	} {
		for _, sign := range []float64{-1, 1} {
			for _, ratio := range []geom.Vec2{{X: 4, Y: 3}, {X: 3, Y: 4}, {X: 12, Y: 5}, {X: 5, Y: 12}} {
				f, err := geom.NewFrame(geom.Vec3{X: 4, Y: 8, Z: 16}, axes[0], axes[1])
				if err != nil {
					t.Fatal(err)
				}
				bounds, err := geom.Bounds([]geom.Vec3{f.Origin(), f.ToWorld(geom.Vec3{X: 20, Y: 16, Z: 70})})
				if err != nil {
					t.Fatal(err)
				}
				b := New()
				base := box(t, b, coordinates(bounds.Min), coordinates(bounds.Max))
				vec := func(x, y, z float64) vector {
					return coordinates(f.XAxis().Scale(x).Add(f.YAxis().Scale(y)).Add(f.ZAxis().Scale(z)))
				}
				point := func(x, y, z float64) vector { return coordinates(f.ToWorld(geom.Vec3{X: x, Y: y, Z: z})) }
				cardinal := circle(t, b, point(10, 8, -2), vec(0, 0, 1), 2, 74)
				steps := math.Ceil(18 / ratio.X)
				oblique := circle(t, b, point(10-steps*ratio.X, 8, 35-steps*ratio.Y*sign), vec(ratio.X, 0, ratio.Y*sign), 2, 150)
				part, err := b.cutMany(base.Handle, []string{oblique.Handle, cardinal.Handle})
				if err != nil {
					t.Fatal(axes, sign, ratio, err)
				}
				stretch := math.Hypot(ratio.X, ratio.Y) / ratio.X
				near(t, part.GeometryAssertions.Volume, 22400-4*math.Pi*(70+20*stretch)+128*stretch/3)
				opening, err := b.shapes[part.Handle].bore.planarFace(f.Origin(), f.XAxis())
				if err != nil {
					t.Fatal(axes, sign, ratio, err)
				}
				near(t, opening.Area, 1120-4*math.Pi*stretch)
				before, err := b.exportSTEP([]string{part.Handle})
				if err != nil {
					t.Fatal(axes, sign, ratio, err)
				}
				raw, err := base64.StdEncoding.DecodeString(before.Base64)
				if err != nil {
					t.Fatal(err)
				}
				got, err := b.importSTEP(string(raw))
				if err != nil || got.Bounds != base.Bounds {
					t.Fatal(axes, sign, ratio, got, err)
				}
				near(t, overlapMeasure(t, b, got.Handle, oblique.Handle), 0)
				again, err := b.exportSTEP([]string{got.Handle})
				if err != nil || again.Base64 != before.Base64 {
					t.Fatal(axes, sign, ratio, err)
				}
			}
		}
	}
}

func TestBlindCrossedObliqueBoreSTEPAndFurtherCut(t *testing.T) {
	for _, opposite := range []bool{false, true} {
		b := New()
		stock := box(t, b, vector{}, vector{20, 16, 40})
		origin, axis := vector{10, 8, 0}, vector{0, 0, 1}
		if opposite {
			origin, axis = vector{10, 8, 40}, vector{0, 0, -1}
		}
		cardinal := circle(t, b, origin, axis, 2, 28)
		through := circle(t, b, vector{-4, 8, 9.5}, vector{4, 0, 3}, 2, 35)
		one := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": cardinal.Handle})
		near(t, overlapMeasure(t, b, one.Handle, through.Handle), 100*math.Pi-160./3)
		out, err := b.cutMany(stock.Handle, []string{through.Handle, cardinal.Handle})
		if err != nil {
			t.Fatal(opposite, err)
		}
		near(t, out.GeometryAssertions.Volume, 12800-212*math.Pi+160./3)
		near(t, out.GeometryAssertions.SurfaceArea, 3520+202*math.Pi-80)
		saved, err := b.exportSTEP([]string{out.Handle})
		if err != nil {
			t.Fatal(opposite, err)
		}
		raw, err := base64.StdEncoding.DecodeString(saved.Base64)
		if err != nil {
			t.Fatal(err)
		}
		got, err := b.importSTEP(string(raw))
		if err != nil || got.GeometryAssertions != out.GeometryAssertions || got.Bounds != stock.Bounds {
			t.Fatal(opposite, got, err)
		}
		again, err := b.exportSTEP([]string{got.Handle})
		if err != nil || again.Base64 != saved.Base64 {
			t.Fatal(opposite, err)
		}
		body := b.shapes[got.Handle]
		floor, err := body.bore.planarFace(origin.vec().Add(axis.vec().Scale(28)), axis.vec().Scale(-1))
		if err != nil || floor.Face != 22 || floor.Normal != coordinates(axis.vec().Scale(-1)) {
			t.Fatal(opposite, floor, err)
		}
		near(t, floor.Area, 4*math.Pi)
		entry, err := body.bore.planarFace(origin.vec(), axis.vec())
		if err != nil {
			t.Fatal(opposite, err)
		}
		near(t, entry.Area, 320-4*math.Pi)
		exit, err := body.bore.planarFace(origin.vec().Add(axis.vec().Scale(40)), axis.vec())
		if err != nil {
			t.Fatal(opposite, err)
		}
		near(t, exit.Area, 320)
		mesh, err := body.tessellate(request{})
		if err != nil {
			t.Fatal(opposite, err)
		}
		if err := mesh.ValidateSolid(); err != nil {
			t.Fatal(opposite, err)
		}
		near(t, overlapMeasure(t, b, got.Handle, cardinal.Handle), 0)
		withoutCover := circle(t, b, coordinates(origin.vec().Add(axis.vec().Scale(26))), axis, 2, 5)
		if _, err := b.cutMany(got.Handle, []string{withoutCover.Handle}); err == nil {
			t.Fatal("in-stock cutter unexpectedly accepted as a replacement")
		}
		for _, depth := range []float64{32, 40} {
			deeper := circle(t, b, origin, axis, 2, depth)
			near(t, overlapMeasure(t, b, got.Handle, deeper.Handle), (depth-28)*4*math.Pi)
			next, err := b.cutMany(got.Handle, []string{deeper.Handle})
			if err != nil {
				t.Fatal(opposite, depth, err)
			}
			fresh, err := b.cutMany(stock.Handle, []string{through.Handle, deeper.Handle})
			if err != nil || fresh.GeometryAssertions != next.GeometryAssertions {
				t.Fatal(opposite, depth, fresh, next, err)
			}
			file, err := b.exportSTEP([]string{next.Handle})
			if err != nil {
				t.Fatal(err)
			}
			reference, err := b.exportSTEP([]string{fresh.Handle})
			if err != nil || reference.Base64 != file.Base64 {
				t.Fatal(opposite, depth, err)
			}
		}
		inside := circle(t, b, origin, axis, 2, 21)
		if _, err := b.cutMany(stock.Handle, []string{through.Handle, inside.Handle}); err == nil {
			t.Fatal("intersecting floor unexpectedly accepted")
		}
	}
}
