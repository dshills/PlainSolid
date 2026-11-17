package plaincadbridge

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestGalleryRobotGripperPalmPocketAndHoles(t *testing.T) {
	b := New()
	rounded := func(width, depth, radius, z, height float64) Shape {
		return shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
			"kind": "roundedRectangle", "origin": vector{0, 0, z}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0},
			"min": [2]float64{-width / 2, -depth / 2}, "max": [2]float64{width / 2, depth / 2},
			"radii": [4]float64{radius, radius, radius, radius}, "height": height,
		}})
	}
	blank := rounded(80, 44, 6, 0, 12)
	cutter := rounded(48, 20, 3, 12, -8)
	pocket := shape(t, b, map[string]any{"op": "cut", "a": blank.Handle, "b": cutter.Handle})
	pocketArea := 48*20 - (4-math.Pi)*9
	near(t, blank.GeometryAssertions.Volume-pocket.GeometryAssertions.Volume, pocketArea*8)
	var floor PlanarFace
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "planarFace", "handle": pocket.Handle,
		"origin": vector{0, 0, 4}, "normal": vector{0, 0, 1}}), &floor); err != nil {
		t.Fatal(err)
	}
	near(t, floor.Area, pocketArea)
	cylinders := 0
	for _, face := range b.shapes[pocket.Handle].analytic.solid.Faces() {
		if surface, ok := face.Surface.(geom.CylinderSurface); ok {
			_, radius := surface.Parameters()
			if radius == 3 && face.Reversed {
				cylinders++
			}
		}
	}
	if cylinders != 4 {
		t.Fatal("missing original concave pocket arcs", cylinders)
	}

	overwide := rounded(82, 20, 3, 12, -8)
	before := len(b.shapes)
	cutRequest := fmt.Sprintf(`{"op":"cut","a":%q,"b":%q}`, blank.Handle, overwide.Handle)
	var refused struct{ Error *failure }
	if err := json.Unmarshal([]byte(b.Call(cutRequest)), &refused); err != nil ||
		refused.Error == nil || refused.Error.Code != "INVALID" || len(b.shapes) != before {
		t.Fatal("rounded pocket crossing the stock wall must fail atomically", refused.Error, err)
	}

	cutHoles := func(stock Shape, centers []vector, radius float64) Shape {
		t.Helper()
		tools := make([]string, 0, len(centers))
		for _, center := range centers {
			tool := circle(t, b, center, vector{0, 0, 1}, radius, 12.000002)
			var overlap struct{ Volume float64 }
			if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "overlapVolume", "a": stock.Handle, "b": tool.Handle}), &overlap); err != nil {
				t.Fatal(err)
			}
			near(t, overlap.Volume, math.Pi*radius*radius*12)
			tools = append(tools, tool.Handle)
		}
		return shape(t, b, map[string]any{"op": "cutMany", "a": stock.Handle, "tools": tools})
	}
	for _, jawSpan := range []float64{56, 60} {
		pivot := cutHoles(pocket, []vector{{-jawSpan / 2, -7, 0}, {jawSpan / 2, -7, 0}}, 3.4)
		palm := cutHoles(pivot, []vector{{-15, -16, 0}, {15, -16, 0}, {15, 16, 0}, {-15, 16, 0}}, 2.2)
		want := (80*44-(4-math.Pi)*36)*12 - pocketArea*8 - math.Pi*12*(2*3.4*3.4+4*2.2*2.2)
		near(t, palm.GeometryAssertions.Volume, want)
		if !palm.GeometryAssertions.Valid || palm.GeometryAssertions.SolidCount != 1 {
			t.Fatal("expected one valid gripper palm", palm)
		}
		display, err := b.shapes[palm.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := display.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		t.Logf("jaw_span %.0f, pocket floor z=4, four radius-3 inside arcs and six full bores, volume %.12f, %d closed display triangles",
			jawSpan, palm.GeometryAssertions.Volume, len(display.Triangles))
	}
}
