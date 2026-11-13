package plaincadbridge

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToFaceAnalyticCircularHoleCoverage(t *testing.T) {
	b := New()
	target := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
		"kind": "polygon", "origin": vector{0, 0, 12}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0}, "height": 2,
		"outer":   [][2]float64{{-10, -10}, {10, -10}, {10, 10}, {-10, 10}},
		"circles": []map[string]any{{"center": [2]float64{}, "radius": 3}},
	}})
	request := map[string]any{"op": "prismToFace", "b": target.Handle, "origin": vector{0, 0, 12},
		"normal": vector{0, 0, -1}, "direction": vector{0, 0, 1},
		"outer": []vector{{-2, -2, 0}, {2, -2, 0}, {2, 2, 0}, {-2, 2, 0}}}
	before := len(b.shapes)
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	reply := b.Call(string(data))
	if !strings.Contains(reply, "termination face does not cover") || len(b.shapes) != before {
		t.Fatal(reply)
	}
	request["outer"] = []vector{{5, -2, 0}, {9, -2, 0}, {9, 2, 0}, {5, 2, 0}}
	output := shape(t, b, request)
	near(t, output.GeometryAssertions.Volume, 192)
}
