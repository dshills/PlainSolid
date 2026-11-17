package plaincadbridge

import (
	"context"
	"fmt"
	"math"
	"testing"

	"plainsolid/brep/parametric"
)

func TestAngleFixtureRoundedWebAndBores(t *testing.T) {
	for _, webHeight := range []float64{36, 40} {
		t.Run(fmt.Sprint(webHeight), func(t *testing.T) {
			b := New()
			foot := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
				"kind": "roundedRectangle", "origin": vector{}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0},
				"min": [2]float64{-30, -20}, "max": [2]float64{30, 20}, "radii": [4]float64{4, 4, 4, 4}, "height": 6,
			}})
			chamfer := shape(t, b, map[string]any{"op": "capChamfer", "handle": foot.Handle, "origin": vector{}, "normal": vector{0, 0, -1}, "distance": .8})
			web := shape(t, b, map[string]any{"op": "prism", "outer": []vector{{-30, 14, 5}, {30, 14, 5}, {30, 20, 5}, {-30, 20, 5}},
				"offset": vector{0, 0, webHeight}})
			body := shape(t, b, map[string]any{"op": "fuse", "a": chamfer.Handle, "b": web.Handle})
			for _, side := range []struct {
				station float64
				normal  vector
				center  func(float64) vector
			}{
				{6, vector{0, 0, 1}, func(x float64) vector { return vector{x, -10, -1} }},
				{6, vector{0, -1, 0}, func(x float64) vector { return vector{x, 25, 25} }},
			} {
				var tools []string
				for _, x := range []float64{-20, 20} {
					tool := circle(t, b, side.center(x), side.normal, 2.5, 60)
					overlap, err := b.overlapVolume(body.Handle, tool.Handle)
					if err != nil {
						t.Fatal("authored hole overlap", side.normal, x, err)
					}
					near(t, overlap.(map[string]float64)["volume"], math.Pi*2.5*2.5*side.station)
					tools = append(tools, tool.Handle)
				}
				body = shape(t, b, map[string]any{"op": "cutMany", "a": body.Handle, "tools": tools})
			}
			// Independent OCCT gallery measure for the bracket alone. The
			// 40 mm variant adds four millimeters of the same web section.
			want := 26391.95494343045 + (webHeight-36)*60*6
			near(t, body.GeometryAssertions.Volume, want)
			native := b.shapes[body.Handle].analytic.solid
			measures, err := native.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			near(t, measures.Volume, want)
			near(t, measures.Area, body.GeometryAssertions.SurfaceArea)
			if body.GeometryAssertions.SolidCount != 1 || len(native.Topology().Shells) != 1 {
				t.Fatal("expected one joined and drilled bracket")
			}
			for i, want := range [3]float64{-30, -20, 0} {
				near(t, body.Bounds.Min[i], want)
			}
			for i, want := range [3]float64{30, 20, 5 + webHeight} {
				near(t, body.Bounds.Max[i], want)
			}
			mesh, err := native.Tessellate(context.Background(), parametric.TessellationOptions{
				Deflection: .5, Angle: .12, MaxSubdivisions: 128, MaxTriangles: 100000,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
