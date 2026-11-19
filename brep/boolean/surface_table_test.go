package boolean

import (
	"errors"
	"math"
	"testing"

	"plainsolid/geom"
)

// TestSurfaceTable covers the plane rows of section 7.4 and the
// node-by-node test of SP4.
func TestSurfaceTable(t *testing.T) {
	const eps = 1e-7
	tol := tolerances{eps: eps, angular: 1e-9, margin: 4 * eps, size: 10}
	level := func(z float64, normal geom.Vec3) plane {
		f, err := geom.FrameFromNormal(geom.Vec3{Z: z}, normal)
		if err != nil {
			t.Fatal(err)
		}
		return plane{f}
	}
	floor := level(0, geom.Vec3{Z: 1})
	t.Run("planes that are not parallel meet in one line", func(t *testing.T) {
		wall := level(0, geom.Vec3{X: 1})
		p, err := surfacePair(floor, wall, tol)
		if err != nil {
			t.Fatal(err)
		}
		if p.alike || len(p.curves) != 1 || p.touching[0] {
			t.Fatalf("got %+v", p)
		}
		l := p.curves[0].(line)
		if !tol.parallel(l.dir, geom.Vec3{Y: 1}) || floor.offset(l.origin) != 0 || wall.offset(l.origin) != 0 {
			t.Fatalf("line %+v", l)
		}
	})
	t.Run("parallel planes are alike, whatever their separation", func(t *testing.T) {
		for _, z := range []float64{0, eps / 2, 5} {
			for _, n := range []geom.Vec3{{Z: 1}, {Z: -1}} {
				if p, err := surfacePair(floor, level(z, n), tol); err != nil || !p.alike || p.curves != nil {
					t.Fatalf("z %g: got %+v, %v", z, p, err)
				}
			}
		}
	})
	t.Run("a pair with no row is unsupported", func(t *testing.T) {
		if _, err := surfacePair(floor, strange{floor}, tol); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("got %v, want ErrUnsupported", err)
		}
	})
	t.Run("SP4", func(t *testing.T) {
		at := func(z, radius float64) point { return point{geom.Vec3{X: 1, Y: 1, Z: z}, radius} }
		for _, c := range []struct {
			name  string
			lower []point // nodes of a face on z = 0
			upper []point // nodes of a face on z = h
			h     float64
			same  bool
			err   error
		}{
			{"coincident", []point{at(0, eps), at(0, eps)}, []point{at(0, eps)}, 0, true, nil},
			{"within every radius", []point{at(eps/4, 1.25*eps)}, []point{at(eps/4, 1.25*eps)}, eps / 2, true, nil},
			{"separated", []point{at(0, eps)}, []point{at(1, eps)}, 1, false, nil},
			{"some nodes on, some off", []point{at(0, eps)}, []point{at(0.5*eps, eps), at(5*eps, eps)}, 0.5 * eps, false, ErrTolerance},
		} {
			same, err := sameSurface([2]surface{floor, level(c.h, geom.Vec3{Z: -1})}, [2][]point{c.lower, c.upper})
			if same != c.same || !errors.Is(err, c.err) {
				t.Errorf("%s: got %v, %v", c.name, same, err)
			}
		}
	})
}

// strange is a surface carrier of a family the tables have no row for.
type strange struct{ plane }

func (strange) family() family { return 255 }

// TestSurfaceTableCurved covers the rows of section 7.4 with a cylinder.
func TestSurfaceTableCurved(t *testing.T) {
	const eps = 1e-7
	tol := tolerances{eps: eps, angular: 1e-9, margin: 4 * eps, size: 10}
	stand := func(x float64, axis geom.Vec3, radius float64) cylinder {
		f, err := geom.FrameFromNormal(geom.Vec3{X: x}, axis)
		if err != nil {
			t.Fatal(err)
		}
		return cylinder{f, radius}
	}
	wall := func(x float64, normal geom.Vec3) plane {
		f, err := geom.FrameFromNormal(geom.Vec3{X: x, Z: 1}, normal)
		if err != nil {
			t.Fatal(err)
		}
		return plane{f}
	}
	up := geom.Vec3{Z: 1}
	tilted, _ := geom.Vec3{X: 1, Z: 1}.Unit()
	tube := stand(0, up, 1)
	for _, c := range []struct {
		name     string
		a, b     surface
		lines    int
		conics   int
		touching bool
		alike    bool
		open     bool
	}{
		{"plane across the axis", wall(0, up), tube, 0, 1, false, false, false},
		{"oblique plane", wall(0, tilted), tube, 0, 1, false, false, false},
		{"plane along the axis, cutting", wall(0.5, geom.Vec3{X: 1}), tube, 2, 0, false, false, false},
		{"plane along the axis, touching", wall(1, geom.Vec3{X: 1}), tube, 1, 0, true, false, false},
		{"plane along the axis, touching within the tolerance", wall(1+eps/2, geom.Vec3{X: 1}), tube, 1, 0, true, false, false},
		{"plane along the axis, clear", wall(2, geom.Vec3{X: 1}), tube, 0, 0, false, false, false},
		{"cylinder with plane, the other way round", tube, wall(0.5, geom.Vec3{X: 1}), 2, 0, false, false, false},
		{"parallel cylinders, cutting", tube, stand(1, up, 1), 2, 0, false, false, false},
		{"parallel cylinders, touching from outside", tube, stand(3, up, 2), 1, 0, true, false, false},
		{"parallel cylinders, touching from inside", tube, stand(0.5, up, 0.5), 1, 0, true, false, false},
		{"parallel cylinders, apart", tube, stand(4, up, 1), 0, 0, false, false, false},
		{"parallel cylinders, one inside the other", tube, stand(0.1, up, 0.5), 0, 0, false, false, false},
		{"coaxial cylinders", tube, stand(0, up, 0.5), 0, 0, false, false, false},
		{"the same cylinder", tube, stand(eps, up.Scale(-1), 1+eps), 0, 0, false, true, false},
		{"cylinders with axes that are not parallel", tube, stand(0, geom.Vec3{X: 1}, 1), 0, 0, false, false, true},
	} {
		p, err := surfacePair(c.a, c.b, tol)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		lines, conics := 0, 0
		for i, k := range p.curves {
			if k.family() == lineFamily {
				lines++
			} else {
				conics++
			}
			if p.touching[i] != c.touching {
				t.Errorf("%s: curve %d touching %v", c.name, i, p.touching[i])
			}
			// Every curve lies on both carriers.
			for _, at := range []float64{-1, 0, 0.7, 2} {
				if q := k.at(at); math.Abs(c.a.offset(q)) > eps || math.Abs(c.b.offset(q)) > eps {
					t.Errorf("%s: curve %d leaves a carrier at %g", c.name, i, at)
				}
			}
		}
		if lines != c.lines || conics != c.conics || p.alike != c.alike || p.open != c.open {
			t.Errorf("%s: got %+v", c.name, p)
		}
	}
}
