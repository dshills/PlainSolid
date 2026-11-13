package plaincadbridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestThreadLeadInRejectsNoRetainedWall(t *testing.T) {
	b := New()
	stock := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{"kind": "disk", "origin": vector{}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0}, "center": []float64{0, 0}, "height": 4, "radius": 1.505, "circles": []any{map[string]any{"center": []float64{0, 0}, "radius": 1.25}}}})
	before := len(b.shapes)
	q := map[string]any{"op": "thread", "handle": stock.Handle, "thread": map[string]any{"side": "internal", "hand": "right", "radius": 1.25, "pitch": .5, "length": 3, "offset": 0, "innerRadius": 1.25, "outerRadius": 1.5, "innerHalfWidth": .5/16 + .25/math.Sqrt(3), "outerHalfWidth": .5 / 16}}
	data, _ := json.Marshal(q)
	result := b.Call(string(data))
	if !strings.Contains(result, "surrounding wall") || len(b.shapes) != before {
		t.Fatal(result)
	}
	near(t, b.shapes[stock.Handle].result.GeometryAssertions.Volume, stock.GeometryAssertions.Volume)
}
