package plaincadbridge

import (
	"encoding/json"
	"testing"
)

func TestDrawingMeasuresCompleteRetainedBore(t *testing.T) {
	b := New()
	base := box(t, b, vector{}, vector{20, 10, 5})
	hole := shape(t, b, map[string]any{"op": "circle", "origin": vector{5, 4, 5}, "normal": vector{0, 0, -1}, "radius": 1, "height": 6})
	cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": hole.Handle})
	moved := shape(t, b, map[string]any{"op": "transform", "handle": cut.Handle,
		"matrix": [][]float64{{0, 0, 1, 30}, {0, 1, 0, 4}, {-1, 0, 0, 20}}})
	var drawing drawingGeometry
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "drawingGeometry", "handle": moved.Handle}), &drawing); err != nil {
		t.Fatal(err)
	}
	if len(drawing.Bores) != 1 || len(drawing.Warnings) != 0 {
		t.Fatal(drawing)
	}
	h := drawing.Bores[0]
	if h.Start != (vector{30, 8, 15}) || h.Direction != (vector{1, 0, 0}) {
		t.Fatal(h)
	}
	near(t, h.Diameter, 2)
	near(t, h.Length, 5)
}
