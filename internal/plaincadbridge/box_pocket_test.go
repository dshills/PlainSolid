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
