package plaincadbridge

import (
	"encoding/json"
	"math"
	"math/big"
	"plainsolid/geom"
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

func TestToFaceExactDiskSeparation(t *testing.T) {
	for _, x := range []float64{math.Nextafter(2, 0), 2, math.Nextafter(2, 3)} {
		p := [2]*big.Rat{new(big.Rat).SetFloat64(x), new(big.Rat)}
		if got := toFaceDisjointDisks(p, geom.Vec2{}, 1, 1); got != (x > 2) {
			t.Fatal(x, got)
		}
	}
	// Squaring in floating point would underflow: the exact gap stays positive.
	tiny := math.SmallestNonzeroFloat64
	p := [2]*big.Rat{new(big.Rat).SetFloat64(3 * tiny), new(big.Rat)}
	if !toFaceDisjointDisks(p, geom.Vec2{}, tiny, tiny) {
		t.Fatal("underflow gap")
	}
}
func TestToFaceAnalyticCircleHoleExactGap(t *testing.T) {
	b := New()
	target := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
		"kind": "polygon", "origin": vector{0, 0, 12}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0}, "height": 2,
		"outer": [][2]float64{{-10, -10}, {10, -10}, {10, 10}, {-10, 10}}, "circles": []map[string]any{{"center": [2]float64{}, "radius": 3}}}})
	for _, x := range []float64{4, math.Nextafter(4, 5)} {
		req := map[string]any{"op": "circleToFace", "b": target.Handle, "origin": vector{0, 0, 12}, "normal": vector{0, 0, -1}, "direction": vector{0, 0, 1}, "center": vector{x, 0, 0}, "radius": 1}
		if x > 4 {
			out := shape(t, b, req)
			near(t, out.GeometryAssertions.Volume, 12*math.Pi)
		} else {
			before := len(b.shapes)
			data, _ := json.Marshal(req)
			reply := b.Call(string(data))
			if !strings.Contains(reply, "termination face does not cover") || len(b.shapes) != before {
				t.Fatal(reply)
			}
		}
	}
}
