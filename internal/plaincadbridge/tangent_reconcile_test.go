package plaincadbridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestTangentDisplayProtocolsAndUnsupportedOperand(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 10, 10})
	tool := circle(t, b, vector{1, 5, 10}, vector{0, 0, -1}, 1, 3)
	hole := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	var modern displayEdges
	var old displayEdgeResult
	if e := json.Unmarshal(invoke(t, b, map[string]any{"op": "displayEdges", "handle": hole.Handle, "maxSegments": 1000, "edgeUses": 100}), &modern); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(invoke(t, b, map[string]any{"op": "displayEdges", "handle": hole.Handle, "maxEdgeSegments": 1000, "maxEdgeUses": 100}), &old); e != nil {
		t.Fatal(e)
	}
	if len(modern.Positions) == 0 || len(modern.Positions) != len(old.Segments) || modern.EdgeUses != 34 || old.EdgeUses != 34 {
		t.Fatal("protocol mismatch")
	}
	for i, x := range modern.Positions {
		if old.Segments[i] != x {
			t.Fatal("model mismatch")
		}
	}
	for _, q := range []map[string]any{
		{"op": "displayEdges", "handle": hole.Handle, "maxSegments": 1},
		{"op": "displayEdges", "handle": hole.Handle, "maxSegments": 1000, "edgeUses": 33},
		{"op": "displayEdges", "handle": hole.Handle, "maxEdgeSegments": 1, "maxEdgeUses": 100},
		{"op": "displayEdges", "handle": hole.Handle, "maxEdgeSegments": 1000, "maxEdgeUses": 33},
		{"op": "cutMany", "a": stock.Handle, "tools": []string{hole.Handle}},
		{"op": "cutMany", "a": hole.Handle, "tools": []string{tool.Handle}},
		{"op": "overlapVolume", "a": hole.Handle, "b": stock.Handle},
	} {
		before, next := len(b.shapes), b.next
		raw, _ := json.Marshal(q)
		out := b.Call(string(raw))
		code := "UNSUPPORTED"
		if q["op"] == "displayEdges" {
			code = "LIMIT"
		}
		if !strings.Contains(out, `"code":"`+code+`"`) || len(b.shapes) != before || b.next != next {
			t.Fatal(q, out)
		}
	}
}

func TestTangentRecipeTranslatedNearContact(t *testing.T) {
	b := New()
	stock := box(t, b, vector{1, 0, 0}, vector{21, 10, 10})
	// The decimal-looking center is represented, but center-1 is NOT exactly
	// the represented radius. Local rounding cannot create a contact proof.
	for _, x := range []float64{1 + .1, 1 + math.Nextafter(.1, 0), 1 + math.Nextafter(.1, 1)} {
		tool := circle(t, b, vector{x, 5, 10}, vector{0, 0, -1}, .1, 3)
		if _, ok := tangentBlindBoreRecipe(b.shapes[stock.Handle], b.shapes[tool.Handle]); ok {
			t.Fatal("rounded translated contact", x)
		}
	}
}
