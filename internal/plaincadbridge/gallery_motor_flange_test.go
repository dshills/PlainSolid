package plaincadbridge

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"plainsolid/geom"
)

func TestGalleryMotorFlangeCircularBossAndReliefs(t *testing.T) {
	for _, thickness := range []float64{8, 10} {
		t.Run(fmt.Sprintf("%.0fmm", thickness), func(t *testing.T) {
			b := New()
			rounded := func(x, width, depth, radius, height float64) Shape {
				return shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
					"kind": "roundedRectangle", "origin": vector{}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0},
					"min": [2]float64{x - width/2, -depth / 2}, "max": [2]float64{x + width/2, depth / 2},
					"radii": [4]float64{radius, radius, radius, radius}, "height": height,
				}})
			}
			stock := rounded(0, 100, 84, 8, thickness)
			normal := vector{0, 0, 1}
			filleted := shape(t, b, map[string]any{"op": "capFillet", "handle": stock.Handle, "origin": vector{0, 0, thickness}, "normal": normal, "radius": 1})
			boss := circle(t, b, vector{0, 0, thickness - 1}, normal, 22, 11)
			result := shape(t, b, map[string]any{"op": "fuse", "a": filleted.Handle, "b": boss.Handle})
			near(t, result.GeometryAssertions.Volume-filleted.GeometryAssertions.Volume, math.Pi*22*22*10)

			if thickness == 8 {
				bad := circle(t, b, vector{0, 0, thickness - 1}, normal, 48, 11)
				before := len(b.shapes)
				fuseRequest := fmt.Sprintf(`{"op":"fuse","a":%q,"b":%q}`, filleted.Handle, bad.Handle)
				var refused struct{ Error *failure }
				if err := json.Unmarshal([]byte(b.Call(fuseRequest)), &refused); err != nil ||
					refused.Error == nil || refused.Error.Code != "INVALID" || len(b.shapes) != before {
					t.Fatal("boss crossing the retained inset perimeter must fail atomically", refused.Error, err)
				}
			}

			cutBores := func(body Shape, centers []vector, radius, span float64) Shape {
				t.Helper()
				tools := make([]string, 0, len(centers))
				for _, center := range centers {
					tool := circle(t, b, center, normal, radius, thickness+10.000002)
					var overlap struct{ Volume float64 }
					if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "overlapVolume", "a": body.Handle, "b": tool.Handle}), &overlap); err != nil {
						t.Fatal(err)
					}
					near(t, overlap.Volume, math.Pi*radius*radius*span)
					tools = append(tools, tool.Handle)
				}
				return shape(t, b, map[string]any{"op": "cutMany", "a": body.Handle, "tools": tools})
			}
			result = cutBores(result, []vector{{}}, 12, thickness+10)
			result = cutBores(result, []vector{{-40, -32, 0}, {40, -32, 0}, {40, 32, 0}, {-40, 32, 0}}, 3, thickness)
			for _, x := range []float64{-32, 32} {
				relief := rounded(x, 8, 24, 3, thickness+10.000002)
				result = shape(t, b, map[string]any{"op": "cut", "a": result.Handle, "b": relief.Handle})
			}

			area := 100*84 - (4-math.Pi)*64
			perimeter := 2*(100+84) - (8-2*math.Pi)*8
			fillet := perimeter*(1-math.Pi/4) - math.Pi*(5.0/3-math.Pi/2)
			reliefArea := 8*24 - (4-math.Pi)*9
			want := area*thickness - fillet + math.Pi*22*22*10 - math.Pi*12*12*(thickness+10) -
				4*math.Pi*3*3*thickness - 2*reliefArea*thickness
			near(t, result.GeometryAssertions.Volume, want)
			if !result.GeometryAssertions.Valid || result.GeometryAssertions.SolidCount != 1 {
				t.Fatal("expected one closed motor flange", result)
			}

			var torus, reliefArcs int
			var bossWall, bearingWall bool
			for _, face := range b.shapes[result.Handle].analytic.solid.Faces() {
				switch s := face.Surface.(type) {
				case geom.TorusSurface:
					torus++
				case geom.CylinderSurface:
					frame, radius := s.Parameters()
					if x := math.Abs(frame.Origin().X); radius == 3 && face.Reversed && (x == 31 || x == 33) {
						reliefArcs++
					}
					bossWall = bossWall || radius == 22 && !face.Reversed
					bearingWall = bearingWall || radius == 12 && face.Reversed
				}
			}
			if torus != 4 || reliefArcs != 8 || !bossWall || !bearingWall {
				t.Fatal("missing analytic perimeter blends, reliefs, boss or bearing bore", torus, reliefArcs, bossWall, bearingWall)
			}
			display, err := b.shapes[result.Handle].tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := display.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			t.Logf("thickness %.0f, boss and 5 full bores, two curved reliefs, volume %.12f, %d closed display triangles",
				thickness, result.GeometryAssertions.Volume, len(display.Triangles))
		})
	}
}
