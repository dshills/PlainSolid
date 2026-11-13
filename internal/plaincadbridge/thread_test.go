package plaincadbridge

import (
	"math"
	"testing"
)

func TestPlainCADModeledThreadNative(t *testing.T) {
	b := New()
	shaft := shape(t, b, map[string]any{"op": "revolvePolygon", "origin": vector{}, "direction": vector{0, 0, 1}, "angle": 2 * math.Pi,
		"outer": []vector{{0, 0, -12}, {1.5, 0, -12}, {1.5, 0, 0}, {2.75, 0, 0}, {2.75, 0, 3}, {0, 0, 3}}})
	socket := shape(t, b, map[string]any{"op": "prism", "outer": []vector{
		{1.443375673, 0, 1.5}, {.7216878365, 1.25, 1.5}, {-.7216878365, 1.25, 1.5},
		{-1.443375673, 0, 1.5}, {-.7216878365, -1.25, 1.5}, {.7216878365, -1.25, 1.5},
	}, "offset": vector{0, 0, 1.5}})
	shaft = shape(t, b, map[string]any{"op": "cut", "a": shaft.Handle, "b": socket.Handle})
	stock := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
		"kind": "disk", "origin": vector{}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0},
		"center": []float64{0, 0}, "height": 4, "radius": 3,
		"circles": []any{map[string]any{"center": []float64{0, 0}, "radius": 1.25}},
	}})
	for _, variant := range []struct {
		side, hand      string
		length, removed float64
	}{
		{"external", "left", 3, 5.002},
		{"external", "left", 2, 5.002 - 4.160/3},
		{"internal", "right", 3, 3.372},
	} {
		side := variant.side
		const pitch, outer = .5, 1.5
		inner := outer - math.Sqrt(3)*17/48*pitch
		a := pitch / 12
		c := a + (outer-inner)/math.Sqrt(3)
		source, radius := shaft, outer
		if side == "internal" {
			inner = 1.25
			a, c = pitch/16+(outer-inner)/math.Sqrt(3), pitch/16
			source, radius = stock, inner
		}
		result := shape(t, b, map[string]any{"op": "thread", "handle": source.Handle, "thread": map[string]any{
			"side": side, "hand": variant.hand, "radius": radius, "pitch": pitch, "length": variant.length, "offset": 0,
			"innerRadius": inner, "outerRadius": outer, "innerHalfWidth": a, "outerHalfWidth": c,
		}})
		got := source.GeometryAssertions.Volume - result.GeometryAssertions.Volume
		if math.Abs(got-variant.removed) >= .005 {
			t.Fatal(side, got, variant.removed)
		}
		if !b.shapes[result.Handle].analytic.helical {
			t.Fatal("thread did not retain helical ownership")
		}
	}
}
