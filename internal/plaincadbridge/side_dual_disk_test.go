package plaincadbridge

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestPlainCADSideOpenDualDiskCuts(t *testing.T) {
	for _, depth := range []float64{3, 6} {
		bridge := New()
		stock := box(t, bridge, vector{0, 4, 0}, vector{10, 10, 6})
		left := circle(t, bridge, vector{3.5, 4, 6}, vector{0, 0, -1}, 2.5, depth)
		// Reversed drill direction, but the same clipped axial interval.
		right := circle(t, bridge, vector{6.5, 4, 6 - depth}, vector{0, 0, 1}, 2.5, depth)
		first := shape(t, bridge, map[string]any{"op": "cut", "a": stock.Handle, "b": left.Handle})
		angle := math.Pi - math.Acos(.6)
		opening := 6.25*angle + 3
		volume := 360 - depth*opening
		area := 312 + (5*angle-8)*depth
		if depth == 6 {
			area -= 2 * opening
		}
		near(t, overlapMeasure(t, bridge, first.Handle, right.Handle), depth*(opening-6.25*math.Pi/2))
		near(t, overlapMeasure(t, bridge, right.Handle, first.Handle), depth*(opening-6.25*math.Pi/2))
		for _, request := range []map[string]any{
			{"op": "cutMany", "a": stock.Handle, "tools": []string{left.Handle, right.Handle}},
			{"op": "cutMany", "a": stock.Handle, "tools": []string{right.Handle, left.Handle}},
			{"op": "cut", "a": first.Handle, "b": right.Handle},
			{"op": "cutMany", "a": first.Handle, "tools": []string{right.Handle}},
		} {
			out := shape(t, bridge, request)
			near(t, out.GeometryAssertions.Volume, volume)
			near(t, out.GeometryAssertions.SurfaceArea, area)
			if out.Bounds != stock.Bounds || out.GeometryAssertions.SolidCount != 1 {
				t.Fatal("dual cut lost stock extrema or solid ownership", out)
			}
			for _, tool := range []Shape{left, right} {
				near(t, overlapMeasure(t, bridge, out.Handle, tool.Handle), 0)
			}
			data, err := writeSTEPBody(bridge.shapes[out.Handle])
			if err != nil || bytes.Count(data, []byte("CYLINDRICAL_SURFACE(")) != 2 || bytes.Contains(data, []byte("PCURVE(")) {
				t.Fatal("retained exact STEP topology", err)
			}
		}
		out := shape(t, bridge, map[string]any{"op": "cut", "a": first.Handle, "b": right.Handle})
		body, err := bridge.shapes[out.Handle].analytic.transformed(geom.Affine{{-1, 0, 0, 12}, {0, 1, 0, 20}, {0, 0, 1, 1}})
		if err != nil || body.sideDualDisk == nil {
			t.Fatal("cardinal reflection lost dual cut recipe", err)
		}
		result, err := bridge.putAnalyticBody(body)
		if err != nil || result.Bounds != (Bounds{Min: vector{2, 24, 1}, Max: vector{12, 30, 7}}) {
			t.Fatal("reflected stock extrema", result.Bounds, err)
		}
	}
}

func TestPlainCADSideOpenDualDiskRejectsUnprovedCrossing(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 4, 0}, vector{10, 10, 6})
	a := circle(t, b, vector{3.5, 4, 0}, vector{0, 0, 1}, 2.5, 3)
	c := circle(t, b, vector{6, 4, 0}, vector{0, 0, 1}, 2.5, 3)
	before := len(b.shapes)
	out, err := b.cutMany(stock.Handle, []string{a.Handle, c.Handle})
	if err == nil || out.Handle != "" || len(b.shapes) != before || !strings.Contains(err.Error(), geom.ErrUnrepresentable.Error()) {
		t.Fatal("irrational crossing must reject without a faceted or partial publish", out, err)
	}
}
