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

// TestCurveTableCurved covers the rows of section 7.5 with a circle, an
// ellipse or a cylinder.
func TestCurveTableCurved(t *testing.T) {
	const eps = 1e-7
	tol := tolerances{eps: eps, angular: 1e-9, margin: 4 * eps, size: 10}
	x, y, z := geom.Vec3{X: 1}, geom.Vec3{Y: 1}, geom.Vec3{Z: 1}
	upright, err := geom.NewFrame(geom.Vec3{}, x, z)
	if err != nil {
		t.Fatal(err)
	}
	tube := cylinder{upright, 1}
	circle := conic{geom.Vec3{}, x, y, 1, 1}
	level := func(h float64, normal geom.Vec3) plane {
		f, err := geom.FrameFromNormal(geom.Vec3{X: h, Y: h, Z: h}, normal)
		if err != nil {
			t.Fatal(err)
		}
		return plane{f}
	}
	oblique, _ := geom.Vec3{X: 1, Z: 1}.Unit()
	for _, c := range []struct {
		name     string
		c        curve
		with     carrier
		alike    bool
		open     bool
		points   []geom.Vec3
		touching bool
	}{
		{"line through a cylinder", line{geom.Vec3{X: -3, Z: 1}, x}, tube, false, false, []geom.Vec3{{X: -1, Z: 1}, {X: 1, Z: 1}}, false},
		{"line touching a cylinder", line{geom.Vec3{X: -3, Y: 1, Z: 1}, x}, tube, false, false, []geom.Vec3{{Y: 1, Z: 1}}, true},
		{"line within the tolerance of touching", line{geom.Vec3{X: -3, Y: 1 + eps/2, Z: 1}, x}, tube, false, false, []geom.Vec3{{Y: 1, Z: 1}}, true},
		{"line past a cylinder", line{geom.Vec3{X: -3, Y: 2}, x}, tube, false, false, nil, false},
		{"line along a cylinder", line{geom.Vec3{X: 1}, z}, tube, true, false, nil, false},
		{"line through the plane of a circle, on it", line{geom.Vec3{X: 1, Z: -1}, z}, circle, false, false, []geom.Vec3{{X: 1}}, false},
		{"line across a circle", line{geom.Vec3{X: -3}, x}, circle, false, false, []geom.Vec3{{X: -1}, {X: 1}}, false},
		{"line touching a circle", line{geom.Vec3{X: -3, Y: 1}, x}, circle, false, false, []geom.Vec3{{Y: 1}}, true},
		{"line over a circle", line{geom.Vec3{X: -3, Z: 1}, x}, circle, false, false, nil, false},
		{"circle through a plane", circle, level(0.5, x), false, false, []geom.Vec3{{X: 0.5, Y: math.Sqrt(0.75)}, {X: 0.5, Y: -math.Sqrt(0.75)}}, false},
		{"circle touching a plane", circle, level(1, x), false, false, []geom.Vec3{{X: 1}}, true},
		{"circle beside a plane", circle, level(2, x), false, false, nil, false},
		{"circle in a parallel plane", circle, level(0, z), true, false, nil, false},
		{"circle in a plane along the axis", conic{geom.Vec3{X: 0.5}, y, z, 2, 2}, tube, false, false, []geom.Vec3{{X: 0.5, Y: math.Sqrt(0.75), Z: math.Sqrt(4 - 0.75)}, {X: 0.5, Y: math.Sqrt(0.75), Z: -math.Sqrt(4 - 0.75)}, {X: 0.5, Y: -math.Sqrt(0.75), Z: math.Sqrt(4 - 0.75)}, {X: 0.5, Y: -math.Sqrt(0.75), Z: -math.Sqrt(4 - 0.75)}}, false},
		{"circle across the axis, crossing", conic{geom.Vec3{X: 1, Z: 3}, x, y, 1, 1}, tube, false, false, []geom.Vec3{{X: 0.5, Y: math.Sqrt(0.75), Z: 3}, {X: 0.5, Y: -math.Sqrt(0.75), Z: 3}}, false},
		{"circle across the axis, touching", conic{geom.Vec3{X: 2, Z: 3}, x, y, 1, 1}, tube, false, false, []geom.Vec3{{X: 1, Z: 3}}, true},
		{"section circle on the cylinder", conic{geom.Vec3{Z: 3}, x, y, 1, 1}, tube, true, false, nil, false},
		{"oblique section on the cylinder", conic{geom.Vec3{Z: 1}, oblique, y, math.Sqrt2, 1}, tube, true, false, nil, false},
		{"ellipse across the axis", conic{geom.Vec3{}, x, y, 2, 1}, tube, false, true, nil, false},
		{"circles in different planes", circle, conic{geom.Vec3{X: 2}, x, z, 1, 1}, false, false, []geom.Vec3{{X: 1}}, false},
		{"circles of one plane, crossing", circle, conic{geom.Vec3{X: 1}, x, y, 1, 1}, false, false, []geom.Vec3{{X: 0.5, Y: math.Sqrt(0.75)}, {X: 0.5, Y: -math.Sqrt(0.75)}}, false},
		{"circles of one plane, touching", circle, conic{geom.Vec3{X: 3}, x, y, 2, 2}, false, false, []geom.Vec3{{X: 1}}, true},
		{"the same circle", circle, conic{geom.Vec3{X: eps / 2}, y, x.Scale(-1), 1, 1}, true, false, nil, false},
		{"circles in parallel planes", circle, conic{geom.Vec3{Z: 1}, x, y, 1, 1}, false, false, nil, false},
		{"conic with line, the other way round", circle, line{geom.Vec3{X: -3}, x}, false, false, []geom.Vec3{{X: -1}, {X: 1}}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, err := curveMeets(c.c, c.with, tol)
			if err != nil {
				t.Fatal(err)
			}
			var hits []hit
			for _, h := range m.hits {
				if h.gap <= 2*eps {
					hits = append(hits, h)
				}
			}
			if m.alike != c.alike || m.open != c.open || len(hits) != len(c.points) {
				t.Fatalf("got %+v", m)
			}
			for _, want := range c.points {
				found := false
				for _, h := range hits {
					if h.p.Distance(want) <= eps {
						found = h.touching == c.touching && c.c.at(h.t).Distance(want) <= eps
						if other, ok := c.with.(curve); ok {
							found = found && other.at(h.s).Distance(want) <= eps
						}
					}
				}
				if !found {
					t.Fatalf("no hit at %v with the right parameters: %+v", want, hits)
				}
			}
		})
	}
	if _, err := curveMeets(conic{geom.Vec3{}, x, y, 2, 1}, conic{geom.Vec3{X: 1}, x, y, 1, 2}, tol); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("coplanar conics that are not homothetic: got %v, want ErrUnsupported", err)
	}
}
