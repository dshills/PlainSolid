package plaincadbridge

import (
	"math"
	"testing"
)

// TestThroughAllHole cuts a hole with the tool PlainCAD makes for a hole
// through all: a cylinder that starts on one face of the stock and is
// longer than the stock is thick by the working tolerance, or that passes
// both faces by it. Rounding puts the far cap to either side of the
// tolerance, and how far from the origin the stock stands decides which.
// The cap is seated on the far face and the hole is a through bore.
func TestThroughAllHole(t *testing.T) {
	type q = map[string]any
	const thick = 5.
	want := 20*10*thick - math.Pi*thick
	for _, c := range []struct {
		name string
		at   vector
	}{
		{"at the origin", vector{0, 0, 0}},
		{"far from the origin", vector{1000, -700, 300}},
	} {
		for _, kind := range []string{"prism", "box"} {
			for _, both := range []bool{false, true} {
				b := New()
				x, y, z := c.at[0], c.at[1], c.at[2]
				var stock answer
				if kind == "prism" {
					stock = ask(t, b, q{"op": "prism", "outer": []vector{{x, y, z}, {x + 20, y, z}, {x + 20, y + 10, z}, {x, y + 10, z}}, "offset": vector{0, 0, thick}})
				} else {
					stock = ask(t, b, q{"op": "box", "min": c.at, "max": vector{x + 20, y + 10, z + thick}})
				}
				if !stock.OK {
					t.Fatalf("%s, %s: %v", c.name, kind, stock.Error)
				}
				// As the application computes it: the length first, then
				// the start from the face the sketch is on.
				length, start := thick+1e-7, z+thick-(thick+1e-7)
				if both {
					length, start = thick+2e-7, z+thick/2-(thick+2e-7)/2
				}
				tool := shape(t, b, q{"op": "circle", "origin": vector{x + 10, y + 5, start}, "normal": vector{0, 0, 1}, "radius": 1, "height": length})
				cut := ask(t, b, q{"op": "cutMany", "a": stock.Result.Handle, "tools": []string{tool.Handle}})
				if !cut.OK {
					t.Errorf("%s, %s, both ends %v: %s: %s", c.name, kind, both, cut.Error.Code, cut.Error.Message)
					continue
				}
				if got := cut.Result.GeometryAssertions.Volume; math.Abs(got-want) > 1e-9 {
					t.Errorf("%s, %s, both ends %v: volume %.12g, want %.12g", c.name, kind, both, got, want)
				}
			}
		}
	}
}
