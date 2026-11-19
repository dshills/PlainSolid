package boolean

import (
	"math"
	"testing"

	"plainsolid/geom"
)

// TestClosedForms checks every closed form of CC1 against analytic
// cases, tangent and coincident ones included.
func TestClosedForms(t *testing.T) {
	const eps = 1e-7
	tol := tolerances{eps: eps, angular: 1e-9, margin: 4 * eps, size: 10}
	near := func(a, b float64) bool { return math.Abs(a-b) <= 1e-9 }
	t.Run("circle with circle", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			centre   geom.Vec2
			radius   float64
			points   []geom.Vec2
			touching bool
			same     bool
		}{
			{"two points", geom.Vec2{X: 1}, 1, []geom.Vec2{{X: 0.5, Y: math.Sqrt(0.75)}, {X: 0.5, Y: -math.Sqrt(0.75)}}, false, false},
			{"tangent from outside", geom.Vec2{X: 3}, 2, []geom.Vec2{{X: 1}}, true, false},
			{"tangent within the tolerance", geom.Vec2{X: 3 + eps/2}, 2, []geom.Vec2{{X: 1}}, true, false},
			{"tangent from inside, the smaller first", geom.Vec2{X: 1}, 2, []geom.Vec2{{X: -1}}, true, false},
			{"tangent from inside, the larger first", geom.Vec2{X: 0.5}, 0.5, []geom.Vec2{{X: 1}}, true, false},
			{"apart", geom.Vec2{X: 4}, 2, nil, false, false},
			{"one inside the other", geom.Vec2{X: 0.1}, 0.5, nil, false, false},
			{"concentric", geom.Vec2{}, 2, nil, false, false},
			{"the same circle", geom.Vec2{X: eps / 2}, 1 + eps/2, nil, false, true},
		} {
			m := circleWithCircle(geom.Vec2{}, 1, c.centre, c.radius, eps)
			if m.touching != c.touching || m.same != c.same || len(m.points) != len(c.points) {
				t.Errorf("%s: got %+v", c.name, m)
				continue
			}
			for i, p := range m.points {
				if math.Hypot(p.X-c.points[i].X, p.Y-c.points[i].Y) > eps {
					t.Errorf("%s: point %d is %v, want %v", c.name, i, p, c.points[i])
				}
			}
		}
	})
	t.Run("conic parameter", func(t *testing.T) {
		tilt, _ := geom.Vec3{X: 1, Y: 1}.Unit()
		c := conic{geom.Vec3{X: 1, Y: 2, Z: 3}, tilt, geom.Vec3{Z: 1}, 3, 1}
		for _, at := range []float64{0, 0.3, math.Pi / 2, 2, math.Pi, 4, 5.5, 6.2} {
			p := c.at(at)
			if got := c.param(p); !near(got, at) {
				t.Errorf("param of the point at %g is %g", at, got)
			}
			// A point just off the conic, along its normal, inverts to
			// the same parameter.
			out := c.tangent(at).Cross(c.normal())
			if got := c.param(p.Add(out.Scale(1e-6))); math.Abs(got-at) > 1e-6 {
				t.Errorf("param of a point off the conic at %g is %g", at, got)
			}
		}
		if f, ok := c.flipped().(conic); !ok || f.at(1).Distance(c.at(-1)) > 1e-12 || f.normal().Dot(c.normal()) > -0.999 {
			t.Error("flipped does not reverse the parameter")
		}
	})
	t.Run("homothetic reduction", func(t *testing.T) {
		x, y := geom.Vec3{X: 1}, geom.Vec3{Y: 1}
		c := conic{geom.Vec3{}, x, y, 2, 1}
		d := conic{geom.Vec3{X: 3}, y, x.Scale(-1), 0.5, 1} // radius 1 along x, 0.5 along y
		if !tol.homothetic(c, d) || tol.homothetic(c, conic{geom.Vec3{}, x, y, 2, 1.5}) || !tol.homothetic(conic{geom.Vec3{}, x, y, 2, 2}, conic{geom.Vec3{}, y, x, 1, 1}) {
			t.Fatal("homothetic gives the wrong answer")
		}
		turned, _ := geom.Vec3{X: 1, Y: 1}.Unit()
		if tol.homothetic(c, conic{geom.Vec3{}, turned, geom.Vec3{Z: 1}.Cross(turned), 2, 1}) {
			t.Fatal("ellipses with turned axes are called homothetic")
		}
		centre, radius, back := asCircles(c, d)
		if !near(centre.X, 3) || !near(centre.Y, 0) || !near(radius, 1) {
			t.Fatalf("the second conic becomes centre %v radius %g, want (3,0) and 1", centre, radius)
		}
		// The two ellipses touch at (2,0,0).
		m := circleWithCircle(geom.Vec2{}, 2, centre, radius, eps)
		if !m.touching || back(m.points[0]).Distance(geom.Vec3{X: 2}) > eps {
			t.Fatalf("got %+v", m)
		}
	})
	t.Run("projection along a cylinder axis", func(t *testing.T) {
		frame, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		s := cylinder{frame, 1}
		oblique, _ := geom.Vec3{X: 1, Z: 1}.Unit()
		for _, c := range []struct {
			name   string
			k      conic
			round  bool
			radius float64
		}{
			{"a circle across the axis", conic{geom.Vec3{X: 0.5, Z: 4}, geom.Vec3{X: 1}, geom.Vec3{Y: 1}, 2, 2}, true, 2},
			{"an oblique section", conic{geom.Vec3{Z: 1}, oblique, geom.Vec3{Y: 1}, math.Sqrt2, 1}, true, 1},
			{"an ellipse across the axis", conic{geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Y: 1}, 2, 1}, false, 0},
			{"a circle in a plane through the axis", conic{geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1}, 1, 1}, false, 0},
		} {
			centre, radius, round := tol.shadow(c.k, s)
			if round != c.round || round && (!near(radius, c.radius) || !near(centre.X, c.k.center.X) || !near(centre.Y, c.k.center.Y)) {
				t.Errorf("%s: centre %v radius %g round %v", c.name, centre, radius, round)
			}
		}
	})
	t.Run("segment distance", func(t *testing.T) {
		for _, c := range []struct {
			p0, p1, q0, q1 geom.Vec3
			want           float64
		}{
			{geom.Vec3{}, geom.Vec3{X: 2}, geom.Vec3{X: 1, Y: -1, Z: 3}, geom.Vec3{X: 1, Y: 1, Z: 3}, 3},
			{geom.Vec3{}, geom.Vec3{X: 2}, geom.Vec3{X: 5, Y: 4}, geom.Vec3{X: 9, Y: 4}, 5},
			{geom.Vec3{}, geom.Vec3{X: 2}, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 1, Y: -1}, 0},
			{geom.Vec3{}, geom.Vec3{X: 2}, geom.Vec3{Y: 1}, geom.Vec3{X: 2, Y: 1}, 1},
			{geom.Vec3{X: 1, Z: 2}, geom.Vec3{X: 1, Z: 2}, geom.Vec3{}, geom.Vec3{X: 2}, 2},
		} {
			if got := segmentDistance(c.p0, c.p1, c.q0, c.q1); !near(got, c.want) {
				t.Errorf("segments %v-%v and %v-%v: got %g, want %g", c.p0, c.p1, c.q0, c.q1, got, c.want)
			}
		}
	})
	t.Run("conic through points", func(t *testing.T) {
		tilt, _ := geom.Vec3{X: 1, Y: 2}.Unit()
		want := conic{geom.Vec3{X: 1, Y: 2, Z: 3}, tilt, geom.Vec3{Z: 1}, 3, 1}
		var five [5]geom.Vec3
		for i := range five {
			five[i] = want.at(0.4 + 1.1*float64(i))
		}
		got, ok := conicThrough(five, eps)
		if !ok || got.center.Distance(want.center) > 1e-9 {
			t.Fatalf("got %+v, %v", got, ok)
		}
		for at := 0.; at < 6; at += 0.5 {
			if d := off(got, want.at(at)); d > 1e-9 {
				t.Fatalf("the fitted conic is %g from the given one at %g", d, at)
			}
		}
		round := conic{geom.Vec3{X: 1}, geom.Vec3{X: 1}, geom.Vec3{Y: 1}, 2, 2}
		if got, ok := conicThrough([5]geom.Vec3{round.at(0), round.at(1), round.at(2), round.at(3), round.at(4)}, eps); !ok || got.a != got.b || !near(got.a, 2) || got.center.Distance(round.center) > 1e-9 {
			t.Fatalf("circle: got %+v, %v", got, ok)
		}
		if _, ok := conicThrough([5]geom.Vec3{{}, {X: 1, Y: 1}, {X: 2, Y: 4}, {X: 3, Y: 9}, {X: -1, Y: 1}}, eps); ok {
			t.Fatal("five points of a parabola gave an ellipse")
		}
	})
}
