package plaincadbridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestScopedJoinPublishesOnlyConnectedAddedMaterial(t *testing.T) {
	b := New()
	first := box(t, b, vector{0, 0, 0}, vector{10, 10, 10})
	last := box(t, b, vector{40, 0, 0}, vector{50, 10, 10})
	middle := box(t, b, vector{20, 0, 0}, vector{30, 10, 10})
	far := box(t, b, vector{100, 0, 0}, vector{110, 10, 10})
	tool := box(t, b, vector{5, 0, 0}, vector{45, 10, 10})
	join := func(targets ...string) map[string]any {
		return map[string]any{"op": "joinAll", "handles": targets, "b": tool.Handle}
	}
	for _, targets := range [][]string{
		{first.Handle, last.Handle},
		{last.Handle, first.Handle, middle.Handle},
	} {
		result := shape(t, b, join(targets...))
		near(t, result.GeometryAssertions.Volume, 5000)
		if result.GeometryAssertions.SolidCount != 1 || result.Bounds != (Bounds{Max: vector{50, 10, 10}}) {
			t.Fatal(result)
		}
	}

	before := len(b.shapes)
	rejected, err := json.Marshal(join(first.Handle, last.Handle, middle.Handle, far.Handle))
	if err != nil {
		t.Fatal(err)
	}
	if reply := b.Call(string(rejected)); !strings.Contains(reply, "requires connected solids") || len(b.shapes) != before {
		t.Fatal(reply)
	}
	contained := box(t, b, vector{5, 0, 0}, vector{10, 10, 10})
	before = len(b.shapes)
	tool = contained
	rejected, err = json.Marshal(join(first.Handle, first.Handle, first.Handle))
	if err != nil {
		t.Fatal(err)
	}
	if reply := b.Call(string(rejected)); !strings.Contains(reply, "added no volume") || len(b.shapes) != before {
		t.Fatal(reply)
	}
	for _, original := range []Shape{first, middle, last, far} {
		near(t, shape(t, b, map[string]any{"op": "inspect", "handle": original.Handle}).GeometryAssertions.Volume, 1000)
	}
}

func TestScopedJoinWithContainingRevolvedTool(t *testing.T) {
	b := New()
	tool := shape(t, b, map[string]any{
		"op": "revolvePolygon", "origin": vector{}, "direction": vector{0, 1, 0}, "angle": 2 * math.Pi,
		"outer": []vector{{0, 0, 0}, {25, 0, 0}, {25, 10, 0}, {0, 10, 0}},
	})
	first := box(t, b, vector{0, 0, 0}, vector{10, 10, 10})
	last := box(t, b, vector{-20, 0, 0}, vector{-10, 10, 10})
	middle := box(t, b, vector{10, 0, 0}, vector{20, 10, 10})
	result := shape(t, b, map[string]any{
		"op": "joinAll", "handles": []string{first.Handle, last.Handle, middle.Handle}, "b": tool.Handle,
	})
	// The tool holds all three boxes, so the join is the tool as a new shape.
	near(t, result.GeometryAssertions.Volume, math.Pi*25*25*10)
	if result.Handle == tool.Handle || result.GeometryAssertions.SolidCount != 1 {
		t.Fatal(result, tool)
	}
	sameShape(t, result, tool)
	invoke(t, b, map[string]any{"op": "dispose", "handle": tool.Handle})
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": result.Handle}).GeometryAssertions.Volume, result.GeometryAssertions.Volume)
}
