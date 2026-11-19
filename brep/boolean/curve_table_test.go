package boolean

import (
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

// TestCurveTable covers the rows line with line and line with plane.
func TestCurveTable(t *testing.T) {
	tol := tolerances{eps: 1e-7, angular: 1e-9, margin: 4e-7, size: 10}
	x := line{geom.Vec3{}, geom.Vec3{X: 1}}
	skew, _ := geom.Vec3{X: 1, Y: 1}.Unit()
	frame, err := geom.NewFrame(geom.Vec3{Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	level := plane{frame}
	for _, c := range []struct {
		name  string
		c     curve
		with  carrier
		alike bool
		hits  []hit
	}{
		{"lines that cross", x, line{geom.Vec3{X: 2, Y: -1}, geom.Vec3{Y: 1}}, false, []hit{{p: geom.Vec3{X: 2}, t: 2, s: 1}}},
		{"lines that pass", x, line{geom.Vec3{X: 2, Y: -1, Z: 0.5}, geom.Vec3{Y: 1}}, false, []hit{{p: geom.Vec3{X: 2, Z: 0.25}, t: 2, s: 1, gap: 0.5}}},
		{"oblique lines", x, line{geom.Vec3{X: 3, Y: 1}, skew}, false, []hit{{p: geom.Vec3{X: 2}, t: 2, s: -math.Sqrt2}}},
		{"parallel lines", x, line{geom.Vec3{Y: 1}, geom.Vec3{X: -1}}, true, nil},
		{"line through a plane", line{geom.Vec3{X: 1, Y: 1}, geom.Vec3{Z: 1}}, level, false, []hit{{p: geom.Vec3{X: 1, Y: 1, Z: 3}, t: 3}}},
		{"oblique line through a plane", line{geom.Vec3{}, geom.Vec3{X: 0.6, Z: 0.8}}, level, false, []hit{{p: geom.Vec3{X: 2.25, Z: 3}, t: 3.75}}},
		{"line in a plane", line{geom.Vec3{Z: 3}, skew}, level, true, nil},
		{"line beside a plane", x, level, true, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, err := curveMeets(c.c, c.with, tol)
			if err != nil {
				t.Fatal(err)
			}
			if m.alike != c.alike || len(m.hits) != len(c.hits) {
				t.Fatalf("got %+v", m)
			}
			for i, h := range m.hits {
				w := c.hits[i]
				if h.p.Distance(w.p) > 1e-12 || math.Abs(h.t-w.t) > 1e-12 || math.Abs(h.s-w.s) > 1e-12 || math.Abs(h.gap-w.gap) > 1e-12 || h.touching {
					t.Fatalf("hit %d: got %+v, want %+v", i, h, w)
				}
			}
		})
	}
	if _, err := curveMeets(x, unknown{}, tol); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("a pair with no row: got %v, want ErrUnsupported", err)
	}
}

// unknown is a carrier of a family the tables have no row for.
type unknown struct{}

func (unknown) family() family { return 255 }
