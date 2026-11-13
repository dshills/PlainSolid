package plaincadbridge

import (
	"math"
	"testing"
)

func TestAnalyticArcLoopBoundsFollowPlacement(t *testing.T) {
	b := New()
	profile := map[string]any{
		"kind": "arcLoop", "origin": vector{}, "normal": vector{0, 0, 1},
		"xDirection": vector{1, 0, 0}, "height": 2,
		"segments": []map[string]any{
			{"start": [2]float64{0, 0}, "end": [2]float64{3, 0}},
			{"start": [2]float64{3, 0}, "end": [2]float64{0, 3}, "center": [2]float64{0, 0}, "radius": 3, "sweep": math.Pi / 2},
			{"start": [2]float64{0, 3}, "end": [2]float64{0, 0}},
		},
	}
	base := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": profile})
	near(t, base.GeometryAssertions.Volume, 4.5*math.Pi)
	if base.Bounds.Min != (vector{}) || base.Bounds.Max != (vector{3, 3, 2}) {
		t.Fatal(base.Bounds)
	}
	placed := shape(t, b, map[string]any{"op": "transform", "handle": base.Handle,
		"matrix": [3][4]float64{{1, 0, 0, 10}, {0, -1, 0, -4}, {0, 0, 1, 5}}})
	if placed.Bounds.Min != (vector{10, -7, 5}) || placed.Bounds.Max != (vector{13, -4, 7}) {
		t.Fatal(placed.Bounds)
	}
	near(t, placed.GeometryAssertions.Volume, base.GeometryAssertions.Volume)
}
