package plaincadbridge

import (
	"encoding/base64"
	"math"
	"testing"
)

func TestSideOpeningPairScopedAndSuccessive(t *testing.T) {
	for _, depths := range [][2]float64{{6, 6}, {2, 6}, {5, 8}, {8, 8}} {
		b := New()
		stock := box(t, b, vector{5, 8, 10}, vector{25, 28, 18})
		left := circle(t, b, vector{13, 8, 10}, vector{0, 0, 1}, 3, depths[0])
		right := circle(t, b, vector{17, 8, 10}, vector{0, 0, 1}, 3, depths[1])
		shared := 9*math.Acos(2./3) - math.Sqrt(20)
		disk := 4.5 * math.Pi
		want := 3200 - disk*(depths[0]+depths[1]) + shared*min(depths[0], depths[1])
		single := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": left.Handle})
		near(t, overlapMeasure(t, b, single.Handle, right.Handle), single.GeometryAssertions.Volume-want)
		for _, tc := range []struct {
			base  Shape
			tools []string
		}{
			{stock, []string{left.Handle, right.Handle}},
			{stock, []string{right.Handle, left.Handle}},
			{single, []string{right.Handle}},
		} {
			out, err := b.cutMany(tc.base.Handle, tc.tools)
			if err != nil {
				t.Fatal(depths, err)
			}
			near(t, out.GeometryAssertions.Volume, want)
			if out.Bounds != stock.Bounds {
				t.Fatal(out.Bounds, stock.Bounds)
			}
			near(t, overlapMeasure(t, b, out.Handle, left.Handle), 0)
			mesh, err := b.shapes[out.Handle].tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			moved := shape(t, b, map[string]any{"op": "transform", "handle": out.Handle, "matrix": [][]float64{{1, 0, 0, 0}, {0, 0, -1, 10}, {0, 1, 0, 20}}})
			export, err := b.exportSTEP([]string{moved.Handle})
			if err != nil {
				t.Fatal(err)
			}
			data, err := base64.StdEncoding.DecodeString(export.Base64)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := b.importSTEP(string(data))
			if err != nil {
				t.Fatal(err)
			}
			near(t, restored.GeometryAssertions.Volume, want)
			if restored.Bounds != moved.Bounds {
				t.Fatal(restored.Bounds, moved.Bounds)
			}
		}
	}
}

func TestSideOpeningPairOpposedScopes(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 20, 8})
	lower := circle(t, b, vector{8, 0, 0}, vector{0, 0, 1}, 3, 5)
	upper := circle(t, b, vector{12, 0, 8}, vector{0, 0, -1}, 3, 6)
	shared := 9*math.Acos(2./3) - math.Sqrt(20)
	want := 3200 - 4.5*math.Pi*11 + 3*shared
	for _, pair := range [][2]Shape{{lower, upper}, {upper, lower}} {
		single := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": pair[0].Handle})
		near(t, overlapMeasure(t, b, single.Handle, pair[1].Handle), single.GeometryAssertions.Volume-want)
		for _, tc := range []struct {
			base  Shape
			tools []string
		}{
			{stock, []string{pair[0].Handle, pair[1].Handle}},
			{single, []string{pair[1].Handle}},
		} {
			out, err := b.cutMany(tc.base.Handle, tc.tools)
			if err != nil {
				t.Fatal(err)
			}
			near(t, out.GeometryAssertions.Volume, want)
			near(t, overlapMeasure(t, b, out.Handle, pair[1].Handle), 0)
			if _, err := b.exportSTEP([]string{out.Handle}); err != nil {
				t.Fatal(err)
			}
		}
	}
}
