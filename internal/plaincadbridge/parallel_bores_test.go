package plaincadbridge

import (
	"math"
	"strings"
	"testing"
)

// TestParallelBoresThatOverlap cuts bores along one direction that run
// into each other. No legacy path builds them any more (section 16 of
// specs/general-boolean-crossing); the general Boolean cuts each to its
// closed form, as one cutMany and as successive cuts.
func TestParallelBoresThatOverlap(t *testing.T) {
	type q = map[string]any
	type bore struct {
		x, radius, depth float64 // from the top face, 20 being through
	}
	// Two circles of radius 3 with centres 4 apart share a lens.
	lens := 2*9*math.Acos(4./6) - 2*math.Sqrt(36-16)
	for _, c := range []struct {
		name    string
		bores   []bore
		removed float64
	}{
		{"side by side", []bore{{15, 3, 20}, {19, 3, 20}}, (2*9*math.Pi - lens) * 20},
		{"a counterbore of three steps", []bore{{15, 5, 4}, {15, 3, 10}, {15, 2, 20}}, math.Pi * (25*4 + 9*6 + 4*10)},
		{"deepened", []bore{{15, 3, 8}, {15, 3, 14}}, math.Pi * 9 * 14},
		// The wider bore is cut second and holds the narrower one, which
		// touches its wall along a line and ends at its floor. A vertex of
		// the narrower rim then lies on a chord of the wider rim.
		{"a wider bore over one that touches it from inside", []bore{{17, 2, 12}, {15, 4, 12}}, math.Pi * 16 * 12},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			b := New()
			stock := box(t, b, vector{0, 0, 0}, vector{40, 30, 20})
			var tools []string
			successive := stock
			for _, h := range c.bores {
				tool := circle(t, b, vector{h.x, 15, 20}, vector{0, 0, -1}, h.radius, h.depth)
				tools = append(tools, tool.Handle)
				successive = shape(t, b, q{"op": "cut", "a": successive.Handle, "b": tool.Handle})
			}
			many, err := b.cutMany(stock.Handle, tools)
			if err != nil {
				t.Fatal(err)
			}
			want := 40*30*20 - c.removed
			for i, got := range []Shape{many, successive} {
				if g := got.GeometryAssertions; !g.Valid || g.SolidCount != 1 || math.Abs(g.Volume-want) > 1e-9*want {
					t.Fatalf("cut %d: %+v, want volume %.12g", i, g, want)
				}
				if s := b.shapes[got.Handle]; s.analytic == nil || !s.analytic.general {
					t.Fatalf("cut %d was not made by the general Boolean", i)
				}
			}
		})
	}
}

// TestParallelBoresThatTouch asks for two bores that touch along a line
// from outside, which would leave no manifold, and for a bore inside one
// that is there already. Each is refused with the reason of the general
// Boolean.
func TestParallelBoresThatTouch(t *testing.T) {
	b := New()
	stock := box(t, b, vector{0, 0, 0}, vector{40, 30, 20})
	first := circle(t, b, vector{15, 15, 20}, vector{0, 0, -1}, 3, 20)
	beside := circle(t, b, vector{21, 15, 20}, vector{0, 0, -1}, 3, 20)
	within := circle(t, b, vector{15.5, 15, 20}, vector{0, 0, -1}, 2, 20)
	if _, err := b.cutMany(stock.Handle, []string{first.Handle, beside.Handle}); err == nil || !strings.Contains(err.Error(), "not a manifold") {
		t.Fatalf("two bores that touch: %v", err)
	}
	bored := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": first.Handle})
	if _, err := b.cutMany(bored.Handle, []string{within.Handle}); err == nil || !strings.Contains(err.Error(), "removed no volume") {
		t.Fatalf("a bore inside a bore: %v", err)
	}
}
