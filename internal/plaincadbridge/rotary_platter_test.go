package plaincadbridge

import (
	"math"
	"plainsolid/brep/parametric"
	"testing"
)

func TestPlainCADRotaryPlatterSlotsAndBores(t *testing.T) {
	for _, height := range []float64{14, 13} {
		t.Run(map[float64]string{14: "original", 13: "edited"}[height], func(t *testing.T) {
			b := New()
			profile := func(kind string) map[string]any {
				return map[string]any{"kind": kind, "origin": vector{0, 0, 34}, "normal": vector{0, 0, 1},
					"xDirection": vector{1, 0, 0}, "height": height}
			}
			annulus := profile("disk")
			annulus["center"], annulus["radius"] = [2]float64{}, 78
			annulus["circles"] = []map[string]any{{"center": [2]float64{}, "radius": 26}}
			body := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": annulus})
			body = shape(t, b, map[string]any{"op": "capChamfer", "handle": body.Handle, "origin": vector{0, 0, 34}, "normal": vector{0, 0, -1}, "distance": .6})
			blank := body.GeometryAssertions.Volume
			for _, xy := range [][4]float64{{-71, -4, -45, 4}, {-4, -71, 4, -45}, {45, -4, 71, 4}, {-4, 45, 4, 71}} {
				slot := profile("roundedRectangle")
				slot["min"], slot["max"] = xy[:2], xy[2:]
				slot["radii"] = [4]float64{3, 3, 3, 3}
				tool := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": slot})
				body = shape(t, b, map[string]any{"op": "cut", "a": body.Handle, "b": tool.Handle})
			}
			near(t, blank-body.GeometryAssertions.Volume, 4*height*(26*8-(4-math.Pi)*9))
			var tools []string
			for _, major := range []int{0, 1} {
				for _, u := range []float64{-63.747688, 63.747688} {
					for _, v := range []float64{-26.405157, 26.405157} {
						origin := vector{u, v, 34}
						if major == 1 {
							origin = vector{v, u, 34}
						}
						hole := shape(t, b, map[string]any{"op": "circle", "origin": origin, "normal": vector{0, 0, 1}, "radius": 3.2, "height": height + 1})
						tools = append(tools, hole.Handle)
					}
				}
			}
			finished := shape(t, b, map[string]any{"op": "cutMany", "a": body.Handle, "tools": tools})
			near(t, body.GeometryAssertions.Volume-finished.GeometryAssertions.Volume, 8*math.Pi*3.2*3.2*height)
			m, e := b.shapes[finished.Handle].analytic.tessellate(parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
			if e != nil {
				t.Fatal("finished platter mesh", e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			mass, e := m.MassProperties()
			if e != nil || math.Abs(mass.Volume-finished.GeometryAssertions.Volume) > finished.GeometryAssertions.Volume*.01 {
				t.Fatal(mass, e)
			}
			t.Log("platter triangles", len(m.Triangles))
		})
	}
}
