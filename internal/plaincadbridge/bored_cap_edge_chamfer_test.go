package plaincadbridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestHoleEditLayeredSingleEdgeChamfer(t *testing.T) {
	for _, tc := range []struct {
		name          string
		r, h, removed float64
	}{{"initial", 2, 10, 85}, {"preview", 2.5, 10, 127.75}, {"edited", 2.5, 3, 40.25}} {
		t.Run(tc.name, func(t *testing.T) {
			b := New()
			stock := box(t, b, vector{}, vector{20, 10, 10})
			one := circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, tc.r, tc.h)
			two := circle(t, b, vector{15, 5, 0}, vector{0, 0, 1}, tc.r, tc.h)
			drilled := shape(t, b, map[string]any{"op": "cutMany", "a": stock.Handle, "tools": []string{one.Handle, two.Handle}})
			cb := circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 3, 1)
			counter := shape(t, b, map[string]any{"op": "cut", "a": drilled.Handle, "b": cb.Handle})
			out := shape(t, b, map[string]any{"op": "chamfer", "handle": counter.Handle, "normal": vector{0, 0, 1}, "distance": .5, "edges": []map[string]any{{"start": vector{0, 0, 0}, "end": vector{20, 0, 0}}}})
			near(t, out.GeometryAssertions.Volume, 2000-tc.removed*math.Pi-2.5)
			s := b.shapes[out.Handle]
			if s.bore == nil || len(s.bore.cylinders) != 3 || s.bore.layered == nil {
				t.Fatal("lost layered recipe")
			}
			near(t, b.shapes[counter.Handle].result.GeometryAssertions.Volume, 2000-tc.removed*math.Pi)
			m, e := s.tessellate(request{})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			// The consumed complete edge must not resolve again.
			req := map[string]any{"op": "chamfer", "handle": out.Handle, "normal": vector{0, 0, 1}, "distance": .5, "edges": []map[string]any{{"start": vector{0, 0, 0}, "end": vector{20, 0, 0}}}}
			raw, _ := json.Marshal(req)
			if response := b.Call(string(raw)); !strings.Contains(response, `"ok":false`) {
				t.Fatal(response)
			}
		})
	}
}
func TestHoleEditChamferRejectsFootprintAndNoop(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 10, 10})
	tool := circle(t, b, vector{5, 2, 0}, vector{0, 0, 1}, 1.5, 10)
	drilled := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	q := map[string]any{"op": "chamfer", "handle": drilled.Handle, "normal": vector{0, 0, 1}, "distance": .5, "edges": []map[string]any{{"start": vector{0, 0, 0}, "end": vector{20, 0, 0}}}}
	raw, _ := json.Marshal(q)
	if response := b.Call(string(raw)); !strings.Contains(response, `"ok":false`) {
		t.Fatal("touching footprint", response)
	}
	stock = box(t, b, vector{}, vector{20, 10, 10})
	wide := circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 4, 10)
	drilled = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": wide.Handle})
	cb := circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 3, 1)
	raw, _ = json.Marshal(map[string]any{"op": "cut", "a": drilled.Handle, "b": cb.Handle})
	if response := b.Call(string(raw)); !strings.Contains(response, `"ok":false`) {
		t.Fatal("downstream no-op", response)
	}
}
