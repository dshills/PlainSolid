package plaincadbridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestNativeBoxPocketBoreAndNestedCut(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{60, 40, 20})
	pocket := box(t, b, vector{12, 12, 14}, vector{24, 20, 20})
	first := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": pocket.Handle})
	near(t, first.GeometryAssertions.Volume, 48000-96*6)
	bit := circle(t, b, vector{45, 25, -1}, vector{0, 0, 1}, 2, 21)
	drilled := shape(t, b, map[string]any{"op": "cutMany", "a": first.Handle, "tools": []string{bit.Handle}})
	want := 48000 - 96*6 - 80*math.Pi
	near(t, drilled.GeometryAssertions.Volume, want)
	floor := map[string]any{"op": "pocketFloor", "handle": drilled.Handle, "origin": vector{0, 0, 14}, "normal": vector{0, 0, 1},
		"outer": []vector{{12, 12, 14}, {24, 12, 14}, {24, 20, 14}, {12, 20, 14}}}
	invoke(t, b, floor)
	var rim measuredRim
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "holeRim", "handle": drilled.Handle,
		"center": vector{45, 25, 20}, "normal": vector{0, 0, 1}, "radius": 2}), &rim); err != nil {
		t.Fatal(err)
	}
	if rim.Center != (vector{45, 25, 20}) || rim.Radius != 2 {
		t.Fatal(rim)
	}
	nested := box(t, b, vector{17, 15, 13}, vector{20, 18, 14})
	cut := shape(t, b, map[string]any{"op": "cut", "a": drilled.Handle, "b": nested.Handle})
	near(t, cut.GeometryAssertions.Volume, want-9)
	floor["handle"] = cut.Handle
	data, err := json.Marshal(floor)
	if err != nil {
		t.Fatal(err)
	}
	if response := b.Call(string(data)); !strings.Contains(response, `"code":"MISMATCH"`) {
		t.Fatal(response)
	}
	m, err := b.shapes[cut.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}

func TestScopedRotatedPocketCopiesOnPocketedStock(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, -40, -25}, vector{20, 40, 25})
	tool := box(t, b, vector{0, 10, -2}, vector{5, 14, 2})
	seed := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	copies := make([]string, 0, 3)
	for i := 1; i <= 3; i++ {
		cosine, sine := math.Cos(float64(i)*math.Pi/2), math.Sin(float64(i)*math.Pi/2)
		copy := shape(t, b, map[string]any{
			"op": "prism", "offset": vector{5, 0, 0},
			"outer": []vector{
				{0, 10*cosine + 2*sine, 10*sine - 2*cosine},
				{0, 14*cosine + 2*sine, 14*sine - 2*cosine},
				{0, 14*cosine - 2*sine, 14*sine + 2*cosine},
				{0, 10*cosine - 2*sine, 10*sine + 2*cosine},
			},
		})
		var hit struct{ Volume float64 }
		err := json.Unmarshal(invoke(t, b, map[string]any{"op": "overlapVolume", "a": seed.Handle, "b": copy.Handle}), &hit)
		if err != nil {
			t.Fatal(err)
		}
		near(t, hit.Volume, 80)
		cut := shape(t, b, map[string]any{"op": "cut", "a": seed.Handle, "b": copy.Handle})
		near(t, cut.GeometryAssertions.Volume, 80000-160)
		copies = append(copies, copy.Handle)
	}
	result := shape(t, b, map[string]any{"op": "cutMany", "a": seed.Handle, "tools": copies})
	near(t, result.GeometryAssertions.Volume, 80000-320)
	if result.GeometryAssertions.SolidCount != 1 {
		t.Fatal(result)
	}
}

func TestPatternOverlapUsesFiniteCylinderLens(t *testing.T) {
	b := New()
	first := circle(t, b, vector{}, vector{0, 0, 1}, 2, 20)
	second := circle(t, b, vector{1, 0, 15}, vector{0, 0, -1}, 2, 10)
	want := (8*math.Acos(.25) - .5*math.Sqrt(15)) * 10
	for _, operands := range [][2]string{{first.Handle, second.Handle}, {second.Handle, first.Handle}} {
		var hit struct{ Volume float64 }
		err := json.Unmarshal(invoke(t, b, map[string]any{"op": "overlapVolume", "a": operands[0], "b": operands[1]}), &hit)
		if err != nil {
			t.Fatal(err)
		}
		near(t, hit.Volume, want)
	}
}
