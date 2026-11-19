package boolean

import (
	"testing"

	"plainsolid/geom"
)

// TestRegion covers the tolerant point-in-loops test of X9 in the plane
// chart, on a square with a square hole.
func TestRegion(t *testing.T) {
	const eps = 1e-7
	frame, err := geom.NewFrame(geom.Vec3{Z: 5}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	loop := func(points ...geom.Vec3) []leg {
		var out []leg
		for i, p := range points {
			q := cyclic(points, i+1)
			d, _ := q.Sub(p).Unit()
			out = append(out, leg{line{p, d}, 0, q.Distance(p)})
		}
		return out
	}
	loops := [][]leg{
		loop(geom.Vec3{Z: 5}, geom.Vec3{X: 4, Z: 5}, geom.Vec3{X: 4, Y: 4, Z: 5}, geom.Vec3{Y: 4, Z: 5}),
		loop(geom.Vec3{X: 1, Y: 1, Z: 5}, geom.Vec3{X: 1, Y: 3, Z: 5}, geom.Vec3{X: 3, Y: 3, Z: 5}, geom.Vec3{X: 3, Y: 1, Z: 5}),
	}
	ch := plane{frame}.chart()
	if got := chartArea(ch, loops); got != 12 {
		t.Fatalf("area %g, want 12", got)
	}
	for _, c := range []struct {
		name string
		p    geom.Vec3
		r    float64
		want where
	}{
		{"inside", geom.Vec3{X: 0.5, Y: 2, Z: 5}, eps, within},
		{"inside, level with a corner", geom.Vec3{X: 0.5, Y: 1, Z: 5}, eps, within},
		{"in the hole", geom.Vec3{X: 2, Y: 2, Z: 5}, eps, without},
		{"outside", geom.Vec3{X: 5, Y: 2, Z: 5}, eps, without},
		{"outside, level with an edge", geom.Vec3{X: -1, Y: 4, Z: 5}, eps, without},
		{"on the outer loop", geom.Vec3{X: 4, Y: 2, Z: 5}, eps, onRim},
		{"on the hole", geom.Vec3{X: 2, Y: 3, Z: 5}, eps, onRim},
		{"at a corner", geom.Vec3{X: 4, Y: 4, Z: 5}, eps, onRim},
		{"within its radius of the loop", geom.Vec3{X: 2, Y: 0.9 * eps, Z: 5}, eps, onRim},
		{"beyond its radius of the loop", geom.Vec3{X: 2, Y: 1.1 * eps, Z: 5}, eps, within},
		{"a wide node near the loop", geom.Vec3{X: 2, Y: 3 * eps, Z: 5}, 4 * eps, onRim},
		{"just outside", geom.Vec3{X: 2, Y: -1.1 * eps, Z: 5}, eps, without},
	} {
		if got := locate(ch, loops, c.p, c.r); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}
