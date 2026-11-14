package plaincadbridge

import (
	"math"
	"testing"
)

func TestRevolvedSteppedShaftSocket(t *testing.T) {
	b := New()
	blank := shape(t, b, map[string]any{
		"op": "revolvePolygon", "origin": vector{}, "direction": vector{0, 0, 1}, "angle": 2 * math.Pi,
		"outer": []vector{{0, 0, -12}, {1.5, 0, -12}, {1.5, 0, 0}, {2.75, 0, 0}, {2.75, 0, 3}, {0, 0, 3}},
	})
	want := math.Pi * (1.5*1.5*12 + 2.75*2.75*3)
	near(t, blank.GeometryAssertions.Volume, want)
	p := make([]vector, 6)
	for i := range p {
		a := float64(i) * math.Pi / 3
		p[i] = vector{1.25 * math.Cos(a) * 2 / math.Sqrt(3), 1.25 * math.Sin(a) * 2 / math.Sqrt(3), 1.5}
	}
	socket := shape(t, b, map[string]any{"op": "prism", "outer": p, "offset": vector{0, 0, 1.5}})
	part := shape(t, b, map[string]any{"op": "cut", "a": blank.Handle, "b": socket.Handle})
	near(t, part.GeometryAssertions.Volume, want-math.Sqrt(3)/2*2.5*2.5*1.5)
	s, err := b.shapes[part.Handle].tessellate(request{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	near(t, part.Bounds.Min[2], -12)
	near(t, part.Bounds.Max[2], 3)
}
