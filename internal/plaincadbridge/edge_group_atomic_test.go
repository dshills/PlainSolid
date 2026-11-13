package plaincadbridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func edgeRef(a, b vector) map[string]any { return map[string]any{"start": a, "end": b} }
func rejectGroup(t *testing.T, b *Bridge, q map[string]any) {
	t.Helper()
	count, next := len(b.shapes), b.next
	raw, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	reply := b.Call(string(raw))
	if !strings.Contains(reply, `"ok":false`) {
		t.Fatal("invalid group accepted", reply)
	}
	if len(b.shapes) != count || b.next != next {
		t.Fatal("invalid group published an intermediate handle")
	}
}
func TestGroupedEdgesRejectAtomicallyAgainstOriginalInput(t *testing.T) {
	b := New()
	base := box(t, b, vector{}, vector{20, 10, 10})
	front := edgeRef(vector{0, 0, 10}, vector{20, 0, 10})
	back := edgeRef(vector{0, 10, 10}, vector{20, 10, 10})
	bottom := edgeRef(vector{0, 0, 0}, vector{20, 0, 0})
	for _, edges := range [][]map[string]any{
		{front, edgeRef(vector{0, 0, 10}, vector{0, 10, 10})},
		{front, edgeRef(vector{0, 0, 10}, vector{19, 0, 10})},
		{front, {"start": vector{0, 10, 10}}},
		{edgeRef(vector{0, 0, 0}, vector{0, 0, 10}), front},
	} {
		rejectGroup(t, b, map[string]any{"op": "fillet", "handle": base.Handle, "edges": edges, "normal": vector{0, 0, 1}, "radius": 1})
	}
	rejectGroup(t, b, map[string]any{"op": "fillet", "handle": base.Handle, "edges": []any{front, back}, "normal": vector{0, 0, 1}, "radius": 6})
	many := make([]any, 33)
	for i := range many {
		many[i] = front
	}
	rejectGroup(t, b, map[string]any{"op": "fillet", "handle": base.Handle, "edges": many, "normal": vector{0, 0, 1}, "radius": 1})
	rejectGroup(t, b, map[string]any{"op": "fillet", "handle": base.Handle, "edges": []any{front}, "edgeStart": vector{0, 0, 10}, "edgeEnd": vector{20, 0, 10}, "normal": vector{0, 0, 1}, "radius": 1})
	rounded := shape(t, b, map[string]any{"op": "fillet", "handle": base.Handle, "edges": []any{front}, "normal": vector{0, 0, 1}, "radius": 1})
	original := b.shapes[rounded.Handle].treated.corners
	rejectGroup(t, b, map[string]any{"op": "chamfer", "handle": rounded.Handle, "edges": []any{bottom, front}, "normal": vector{0, 0, 1}, "distance": 1})
	rejectGroup(t, b, map[string]any{"op": "chamfer", "handle": rounded.Handle, "edges": []any{bottom}, "normal": vector{0, 1, 0}, "distance": 1})
	if b.shapes[rounded.Handle].treated.corners != original {
		t.Fatal("failed group mutated the source recipe")
	}
	var available struct {
		Available bool `json:"available"`
	}
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "edgeAvailable", "handle": rounded.Handle, "edgeStart": vector{0, 0, 0}, "edgeEnd": vector{20, 0, 0}}), &available); err != nil {
		t.Fatal(err)
	}
	if !available.Available {
		t.Fatal("failed group consumed its first valid edge")
	}
	near(t, b.shapes[base.Handle].result.GeometryAssertions.Volume, 2000)
}
func TestGroupedChamferOverlapIsAtomic(t *testing.T) {
	b := New()
	base := box(t, b, vector{}, vector{20, 10, 10})
	front := edgeRef(vector{0, 0, 0}, vector{20, 0, 0})
	back := edgeRef(vector{0, 10, 0}, vector{20, 10, 0})
	rejectGroup(t, b, map[string]any{"op": "chamfer", "handle": base.Handle, "edges": []any{front, back}, "normal": vector{0, 0, 1}, "distance": 6})
	// Duplicate/reversed authored references resolve once, as in OCCT.
	cut := shape(t, b, map[string]any{"op": "chamfer", "handle": base.Handle, "edges": []any{front, back, edgeRef(vector{20, 0, 0}, vector{0, 0, 0})}, "normal": vector{0, 0, 1}, "distance": 1})
	near(t, cut.GeometryAssertions.Volume, 1980)
}

func TestMixedCapGroupOrderPreservesAuthoredUpperBoss(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		b := New()
		base := box(t, b, vector{}, vector{20, 10, 10})
		top := edgeRef(vector{0, 0, 10}, vector{20, 0, 10})
		bottom := edgeRef(vector{0, 10, 0}, vector{20, 10, 0})
		edges := []any{top, bottom}
		if reverse {
			edges[0], edges[1] = edges[1], edges[0]
		}
		rounded := shape(t, b, map[string]any{"op": "fillet", "handle": base.Handle, "edges": edges, "normal": vector{0, 0, 1}, "radius": 1})
		boss := box(t, b, vector{15, 5, 10}, vector{17, 7, 13})
		joined := shape(t, b, map[string]any{"op": "fuse", "a": rounded.Handle, "b": boss.Handle})
		near(t, joined.GeometryAssertions.Volume, 2012-40*(1-math.Pi/4))
		near(t, joined.Bounds.Min[2], 0)
		near(t, joined.Bounds.Max[2], 13)
		face, err := b.shapes[joined.Handle].treated.planarFace(vector{0, 0, 10}.vec(), vector{0, 0, 1}.vec())
		if err != nil {
			t.Fatal(err)
		}
		near(t, face.Area, 176)
		near(t, face.Normal[2], 1)
	}
}
