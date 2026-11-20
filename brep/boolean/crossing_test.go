package boolean

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// The tests of this file hold cylinders of equal radius whose axes meet
// (specs/general-boolean-crossing) to closed forms. Two cylinders of
// radius r whose axes meet at an angle of sine s have a common of volume
// 16r³/(3s), bounded by 16r²/s of their walls together.

var crossingOptions = Options{Linear: 1e-7, Angular: 1e-9, Volume: 1e-7}

// crossingWithin is the bound of A3 of the general Boolean specification.
func crossingWithin(got, want float64) bool {
	return math.Abs(got-want) <= 1e-7*math.Max(1, math.Abs(want))
}

// crossingStock is a prism on a polygon of the plane z = z0, of height h,
// moved by m.
func crossingStock(t *testing.T, m geom.Affine, outline []geom.Vec2, z0, h float64) trimmed.Solid {
	t.Helper()
	points := make([]geom.Vec3, len(outline))
	for i, p := range outline {
		points[i] = m.Apply(geom.Vec3{X: p.X, Y: p.Y, Z: z0})
	}
	p, err := brep.Prism(points, m.ApplyVector(geom.Vec3{Z: h}))
	if err != nil {
		t.Fatal(err)
	}
	s, err := trimmed.FromPlanar(p, crossingOptions.Linear)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// crossingRod is a cylinder of radius r and length l about the line
// through centre along axis, with centre at its middle, moved by m.
func crossingRod(t *testing.T, m geom.Affine, centre, axis geom.Vec3, r, l float64) trimmed.Solid {
	t.Helper()
	unit, err := axis.Unit()
	if err != nil {
		t.Fatal(err)
	}
	frame, err := geom.FrameFromNormal(m.Apply(centre.Sub(unit.Scale(l/2))), m.ApplyVector(unit))
	if err != nil {
		t.Fatal(err)
	}
	round, err := parametric.Cylinder(frame, r, l)
	if err != nil {
		t.Fatal(err)
	}
	s, err := trimmed.FromParametric(round)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func crossingRectangle(w, d float64) []geom.Vec2 {
	return []geom.Vec2{{X: -w / 2, Y: -d / 2}, {X: w / 2, Y: -d / 2}, {X: w / 2, Y: d / 2}, {X: -w / 2, Y: d / 2}}
}

// crossingTurn is a rigid motion that puts no axis of a test along a
// coordinate axis.
func crossingTurn(t *testing.T) geom.Affine {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{X: 3, Y: -2, Z: 5}, geom.Vec3{X: 2, Y: 3, Z: -6}, geom.Vec3{X: 3, Y: -6, Z: -2})
	if err != nil {
		t.Fatal(err)
	}
	x, y, z, o := frame.XAxis(), frame.YAxis(), frame.ZAxis(), frame.Origin()
	return geom.Affine{{x.X, y.X, z.X, o.X}, {x.Y, y.Y, z.Y, o.Y}, {x.Z, y.Z, z.Z, o.Z}}
}

// TestCrossingBores cuts two through bores of radius 1 from a prism of
// width 8 along x: one along z, and one in the plane y = 0 at an angle of
// sine s to the first, which leaves through the two faces across x.
func TestCrossingBores(t *testing.T) {
	const r, w, d = 1., 8., 8.
	hexagon := []geom.Vec2{{X: 4, Y: -3}, {X: 4, Y: 3}, {X: 0, Y: 6}, {X: -4, Y: 3}, {X: -4, Y: -3}, {X: 0, Y: -6}}
	for _, c := range []struct {
		name    string
		sine    float64
		outline []geom.Vec2
		area    float64 // of the outline
		around  float64 // its perimeter
		turned  bool
		refused string // part of the message when the pair is not taken
	}{
		{"right angle", 1, crossingRectangle(w, d), w * d, 2 * (w + d), false, ""},
		{"sine 0.8", 0.8, crossingRectangle(w, d), w * d, 2 * (w + d), false, ""},
		{"sine 0.6", 0.6, crossingRectangle(w, d), w * d, 2 * (w + d), false, ""},
		{"sine 5/13", 5. / 13, crossingRectangle(w, d), w * d, 2 * (w + d), false, ""},
		{"sine 1/8", 1. / 8, crossingRectangle(w, d), w * d, 2 * (w + d), false, ""},
		{"sine 1/64", 1. / 64, crossingRectangle(w, d), w * d, 2 * (w + d), false, ""},
		{"sine 1/128, below the least angle", 1. / 128, crossingRectangle(w, d), w * d, 2 * (w + d), false, "too nearly parallel"},
		{"a hexagonal prism, no axis along a coordinate axis", 0.8, hexagon, 72, 32, true, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := geom.Identity()
			if c.turned {
				m = crossingTurn(t)
			}
			cosine := math.Sqrt(1 - c.sine*c.sine)
			// The slanted bore leaves through the faces across x when the
			// prism is this tall, with 2 to spare at each end.
			h := 2 * ((w/2)*cosine/c.sine + r/c.sine + 2)
			stock := crossingStock(t, m, c.outline, -h/2, h)
			upright := crossingRod(t, m, geom.Vec3{}, geom.Vec3{Z: 1}, r, h+4)
			slanted := crossingRod(t, m, geom.Vec3{}, geom.Vec3{X: c.sine, Z: cosine}, r, w/c.sine+2*(r*cosine/c.sine+2))
			first, err := Combine(context.Background(), stock, upright, Cut, crossingOptions)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Combine(context.Background(), first.Solid, slanted, Cut, crossingOptions)
			if c.refused != "" {
				if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), c.refused) {
					t.Fatalf("%v, want unsupported with %q", err, c.refused)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			volume := c.area*h - math.Pi*r*r*h - math.Pi*r*r*w/c.sine + 16*r*r*r/(3*c.sine)
			area := 2*c.area + c.around*h - 2*math.Pi*r*r - 2*math.Pi*r*r/c.sine +
				2*math.Pi*r*h + 2*math.Pi*r*w/c.sine - 16*r*r/c.sine
			if got.Solids != 1 || !crossingWithin(got.Volume, volume) || !crossingWithin(got.Properties.Area, area) {
				t.Fatalf("%d solids, volume %.12g want %.12g, area %.12g want %.12g", got.Solids, got.Volume, volume, got.Properties.Area, area)
			}
		})
	}
}

// TestCrossingBoresThree cuts three bores of radius 1 that meet at one
// point at right angles from a cube of side 10. The three cylinders have a
// common of volume 8(2 − √2).
func TestCrossingBoresThree(t *testing.T) {
	m := geom.Identity()
	body := crossingStock(t, m, crossingRectangle(10, 10), -5, 10)
	for _, axis := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}} {
		r, err := Combine(context.Background(), body, crossingRod(t, m, geom.Vec3{}, axis, 1, 14), Cut, crossingOptions)
		if err != nil {
			t.Fatal(err)
		}
		body = r.Solid
	}
	last, err := Combine(context.Background(), body, trimmed.Solid{}, Union, crossingOptions)
	if err != nil {
		t.Fatal(err)
	}
	if want := 1000 - 30*math.Pi + 16 - 8*(2-math.Sqrt2); last.Solids != 1 || !crossingWithin(last.Volume, want) {
		t.Fatalf("%d solids, volume %.12g want %.12g", last.Solids, last.Volume, want)
	}
}

// TestCrossingCylinders takes the three operations on two solid cylinders
// of radius 1 and length 6 whose axes meet at right angles at their
// middles. The difference is two parts that meet only at two points, which
// is not a manifold.
func TestCrossingCylinders(t *testing.T) {
	m := crossingTurn(t)
	a := crossingRod(t, m, geom.Vec3{}, geom.Vec3{Z: 1}, 1, 6)
	b := crossingRod(t, m, geom.Vec3{}, geom.Vec3{X: 1}, 1, 6)
	both := 16. / 3
	for _, c := range []struct {
		op     Operation
		volume float64
		area   float64
	}{
		{Union, 12*math.Pi - both, 2 * (14*math.Pi - 8)},
		{Common, both, 16},
	} {
		got, err := Combine(context.Background(), a, b, c.op, crossingOptions)
		if err != nil {
			t.Fatalf("operation %v: %v", c.op, err)
		}
		if got.Solids != 1 || !crossingWithin(got.Volume, c.volume) || !crossingWithin(got.Properties.Area, c.area) {
			t.Fatalf("operation %v: %d solids, volume %.12g want %.12g, area %.12g want %.12g", c.op, got.Solids, got.Volume, c.volume, got.Properties.Area, c.area)
		}
	}
	if _, err := Combine(context.Background(), a, b, Cut, crossingOptions); !errors.Is(err, ErrNonManifold) {
		t.Fatalf("the difference of two crossing cylinders: %v, want a result that is not a manifold", err)
	}
}

// TestCrossingRefusals holds the pair the row does not take to its
// error: radii and axes that agree only separately, which is neither an
// equal pair nor clear enough of touching to be a quartic pair in scope.
// TestCrossingBores holds axes too nearly parallel, and TestQuarticBores
// what unequal radii and axes that pass each other now give.
func TestCrossingRefusals(t *testing.T) {
	m := geom.Identity()
	stock := crossingStock(t, m, crossingRectangle(8, 8), -4, 8)
	first, err := Combine(context.Background(), stock, crossingRod(t, m, geom.Vec3{}, geom.Vec3{Z: 1}, 1, 12), Cut, crossingOptions)
	if err != nil {
		t.Fatal(err)
	}
	const eps = 1e-7
	for _, c := range []struct {
		name    string
		centre  geom.Vec3
		axis    geom.Vec3
		radius  float64
		message string
	}{
		{"radii and axes within the tolerance separately", geom.Vec3{Y: eps}, geom.Vec3{X: 1}, 1 + eps/2, "cylinders that nearly touch"},
	} {
		_, err := Combine(context.Background(), first.Solid, crossingRod(t, m, c.centre, c.axis, c.radius, 12), Cut, crossingOptions)
		if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), c.message) {
			t.Errorf("%s: %v, want unsupported with %q", c.name, err, c.message)
		}
	}
	// Within the tolerance together, the pair is cut.
	near := crossingRod(t, m, geom.Vec3{Y: eps / 4}, geom.Vec3{X: 1}, 1+eps/2, 12)
	got, err := Combine(context.Background(), first.Solid, near, Cut, crossingOptions)
	if err != nil {
		t.Fatalf("radii and axes within the tolerance together: %v", err)
	}
	if want := 512 - 8*math.Pi - 8*math.Pi + 16./3; !crossingWithin(got.Volume, want) {
		t.Fatalf("volume %.12g, want %.12g", got.Volume, want)
	}
}

// TestClosedEdgeInAnyPose unites a cylinder in a pose off the coordinate
// axes with the empty solid. The interval of its closed rims starts off
// zero, where taking it back from its end rounds below the period.
func TestClosedEdgeInAnyPose(t *testing.T) {
	rod := crossingRod(t, crossingTurn(t), geom.Vec3{}, geom.Vec3{Z: 1}, 1, 6)
	got, err := Combine(context.Background(), rod, trimmed.Solid{}, Union, crossingOptions)
	if err != nil {
		t.Fatal(err)
	}
	if !crossingWithin(got.Volume, 6*math.Pi) {
		t.Fatalf("volume %.12g, want %.12g", got.Volume, 6*math.Pi)
	}
}

// TestCrossingFloorInTheCrossing cuts a through bore at a slant and a
// blind bore along z whose floor lies inside the part the two bores
// share, which here reaches from height 16 to height 24, in both orders.
// The result does not depend on the order. The stock and the slanted bore
// are their own image under the half turn about the line through the
// meeting point of the axes along y, which takes a blind bore from below
// with its floor at height h to one from above with its floor at 40 − h,
// so the two have one volume.
//
// The floors of this test lie outside the band from 19 to 21, or at its
// middle. In that band lies the smaller of the two ellipses along which
// the bores cross, and a floor strictly inside it meets that ellipse as
// well as the larger one. Such a floor is a known limit: it is cut in some
// orders and placements and refused in others, as unsupported by M6 ("a
// face … touches itself"), as failing V1 ("invalid face … loop … size"),
// or, with the floor at 21 where it touches the top of the ellipse, as
// inconsistent by F3. Every volume that is returned in the band agrees
// with the other order and with the image; none is wrong.
func TestCrossingFloorInTheCrossing(t *testing.T) {
	m := geom.Identity()
	stock := crossingStock(t, m, []geom.Vec2{{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 16}, {X: 0, Y: 16}}, 0, 40)
	slanted := crossingRod(t, m, geom.Vec3{X: 10, Y: 8, Z: 20}, geom.Vec3{X: 4, Z: 3}, 2, 35)
	cut := func(first, second trimmed.Solid) float64 {
		t.Helper()
		a, err := Combine(context.Background(), stock, first, Cut, crossingOptions)
		if err != nil {
			t.Fatal(err)
		}
		b, err := Combine(context.Background(), a.Solid, second, Cut, crossingOptions)
		if err != nil {
			t.Fatal(err)
		}
		return b.Volume
	}
	for _, floor := range []float64{17, 18, 20, 22, 23} {
		below := crossingRod(t, m, geom.Vec3{X: 10, Y: 8, Z: floor / 2}, geom.Vec3{Z: 1}, 2, floor)
		above := crossingRod(t, m, geom.Vec3{X: 10, Y: 8, Z: 40 - floor/2}, geom.Vec3{Z: -1}, 2, floor)
		want := cut(below, slanted)
		for _, c := range []struct {
			name string
			got  float64
		}{
			{"slanted bore first", cut(slanted, below)},
			{"the image from above, blind bore first", cut(above, slanted)},
			{"the image from above, slanted bore first", cut(slanted, above)},
		} {
			if !crossingWithin(c.got, want) {
				t.Errorf("floor at %g: %s: volume %.12g, want %.12g", floor, c.name, c.got, want)
			}
		}
	}
}
