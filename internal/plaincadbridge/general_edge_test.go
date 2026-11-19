package plaincadbridge

import (
	"context"
	"encoding/json"
	"testing"
)

// general publishes, on a default bridge, the result of the general
// Boolean for two stored shapes, without asking a legacy path. PlainCAD
// reaches such a body where a legacy path declines a request.
func general(t *testing.T, b *Bridge, op string, a, c Shape) Shape {
	t.Helper()
	s, err := b.generalBoolean(context.Background(), b.shapes[a.Handle], b.shapes[c.Handle], op)
	if err != nil {
		t.Fatal(err)
	}
	if body := b.shapes[s.Handle].analytic; body == nil || !b.generalResult(body) {
		t.Fatalf("the %s did not publish a general result", op)
	}
	return s
}

// slottedBox is the body of Q1: the box (0,0,0)–(20,10,10) less the box
// (12,−1,6)–(16,11,11), as a general result.
func slottedBox(t *testing.T, b *Bridge) Shape {
	t.Helper()
	s := general(t, b, "cut", box(t, b, vector{}, vector{20, 10, 10}), box(t, b, vector{12, -1, 6}, vector{16, 11, 11}))
	near(t, s.GeometryAssertions.Volume, 1840)
	near(t, s.GeometryAssertions.SurfaceArea, 1048)
	return s
}

func available(t *testing.T, b *Bridge, handle string, start, end vector) bool {
	t.Helper()
	var reply struct{ Available bool }
	if err := json.Unmarshal(invoke(t, b, map[string]any{"op": "edgeAvailable", "handle": handle, "edgeStart": start, "edgeEnd": end}), &reply); err != nil {
		t.Fatal(err)
	}
	return reply.Available
}

// TestGeneralEdgeAvailable asks a general result which references resolve
// (R11).
func TestGeneralEdgeAvailable(t *testing.T) {
	b := New()
	s := slottedBox(t, b)
	for _, c := range []struct {
		name       string
		start, end vector
		want       bool
	}{
		{"a bottom edge", vector{0, 0, 0}, vector{20, 0, 0}, true},
		{"a top edge up to the slot", vector{0, 0, 10}, vector{12, 0, 10}, true},
		{"a top edge across the slot", vector{0, 0, 10}, vector{20, 0, 10}, false},
		{"a line on a face", vector{0, 2, 0}, vector{20, 2, 0}, false},
		{"one point", vector{0, 0, 0}, vector{0, 0, 0}, false},
	} {
		if got := available(t, b, s.Handle, c.start, c.end); got != c.want {
			t.Errorf("%s: available %v, want %v", c.name, got, c.want)
		}
	}
	// A bore that leaves through the front face meets it along straight
	// edges with a cylinder on one side, which take no treatment.
	bored := general(t, b, "cut", box(t, b, vector{}, vector{20, 10, 10}), circle(t, b, vector{5, 0, -1}, vector{0, 0, 1}, 2, 12))
	if available(t, b, bored.Handle, vector{3, 0, 0}, vector{3, 0, 10}) {
		t.Error("an edge between a plane and a cylinder is available")
	}
	if !available(t, b, bored.Handle, vector{0, 10, 0}, vector{20, 10, 0}) {
		t.Error("an untouched edge of the bored box is not available")
	}
}
