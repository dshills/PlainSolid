package meeting

import (
	"math"
	"testing"

	"plainsolid/geom"
)

// pair is a large cylinder about the z axis and a small one whose axis
// passes it at distance d along y, at an angle of the given sine to it.
func pair(t *testing.T, d, sine float64) (large, small geom.Frame) {
	t.Helper()
	large, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	small, err = geom.FrameFromNormal(geom.Vec3{Y: d}, geom.Vec3{X: sine, Z: math.Sqrt(1 - sine*sine)})
	if err != nil {
		t.Fatal(err)
	}
	return large, small
}

// TestCurves is A10 of specs/general-boolean-quartic: every curve of a
// pair lies on both cylinders, inverts a point to its parameter, has the
// derivative its points show, and on each cylinder has a p-curve that
// lifts to it and is monotone between the turns it lists.
func TestCurves(t *testing.T) {
	for _, c := range []struct {
		name                string
		big, small, d, sine float64
		curves              int
	}{
		{"full, right angle", 5, 2, 0, 1, 2},
		{"full, skew at a slant", 5, 2, 1, 0.6, 2},
		{"partial", 5, 3, 4, 1, 1},
		{"partial, equal radii", 3, 3, 2, 1, 1},
	} {
		large, small := pair(t, c.d, c.sine)
		curves, reason := Between(large, c.big, small, c.small, 1e-9, 1e-7)
		if len(curves) != c.curves || reason != "" {
			t.Fatalf("%s: %d curves, reason %q", c.name, len(curves), reason)
		}
		if swapped, _ := Between(small, c.small, large, c.big, 1e-9, 1e-7); len(swapped) != c.curves {
			t.Fatalf("%s: %d curves with the cylinders the other way round", c.name, len(swapped))
		}
		for _, q := range append(curves, curves[0].Reversed()) {
			for k := range 64 {
				u := 2 * math.Pi * float64(k) / 64
				p, v := q.State(u)
				for _, on := range []struct {
					frame  geom.Frame
					radius float64
				}{{large, c.big}, {small, c.small}} {
					l := on.frame.ToLocal(p)
					if off := math.Abs(math.Hypot(l.X, l.Y) - on.radius); off > 1e-12 {
						t.Fatalf("%s: the point at %g is %g off a cylinder", c.name, u, off)
					}
				}
				if off := math.Abs(math.Remainder(q.Param(p)-u, 2*math.Pi)); off > 1e-9 {
					t.Fatalf("%s: the point at %g inverts %g away", c.name, u, off)
				}
				const h = 1e-6
				before, _ := q.State(u - h)
				after, _ := q.State(u + h)
				if off := after.Sub(before).Scale(1 / (2 * h)).Distance(v); off > 1e-6 {
					t.Fatalf("%s: the derivative at %g is %g from the slope of the points", c.name, u, off)
				}
			}
			for _, on := range []struct {
				frame  geom.Frame
				radius float64
			}{{large, c.big}, {small, c.small}} {
				surface, err := geom.NewCylinderSurface(on.frame, on.radius)
				if err != nil {
					t.Fatal(err)
				}
				for quarter := range 4 {
					t0 := math.Pi / 2 * float64(quarter)
					arc := q.Trim(on.frame, t0+math.Pi/2, t0, 7)
					if start, _ := arc.EvalUV(0); math.Abs(start.X-7) > math.Pi {
						t.Fatalf("%s: an arc asked near 7 starts at angle %g", c.name, start.X)
					}
					parts := append(append([]float64{0}, arc.TurnsUV()...), 1)
					for i := 0; i+1 < len(parts); i++ {
						var last geom.Vec2
						var way [2]float64
						for k := 0; k <= 8; k++ {
							u := parts[i] + (parts[i+1]-parts[i])*float64(k)/8
							uv, _ := arc.EvalUV(u)
							lifted, err := surface.Eval(uv.X, uv.Y)
							if err != nil {
								t.Fatal(err)
							}
							exact, _ := q.State(t0 + math.Pi/2*(1-u))
							if off := lifted.Distance(exact); off > 1e-12 {
								t.Fatalf("%s: the p-curve at %g lifts %g from the curve", c.name, u, off)
							}
							if k > 0 {
								for axis, step := range [2]float64{uv.X - last.X, uv.Y - last.Y} {
									if math.Abs(step) > 1e-12 && way[axis]*step < 0 {
										t.Fatalf("%s: a coordinate turns back between %g and %g, turns %v", c.name, parts[i], parts[i+1], arc.TurnsUV())
									}
									if math.Abs(step) > 1e-12 {
										way[axis] = step
									}
								}
							}
							last = uv
						}
					}
					box, _ := arc.BoundsUV()
					for k := 0; k <= 16; k++ {
						uv, _ := arc.EvalUV(float64(k) / 16)
						if uv.X < box.Min.X || uv.X > box.Max.X || uv.Y < box.Min.Y || uv.Y > box.Max.Y {
							t.Fatalf("%s: the p-curve leaves its bounds at %d/16", c.name, k)
						}
					}
					pinned := arc.Pinned(geom.Vec2{X: 1, Y: 2}, geom.Vec2{X: 3, Y: 4})
					if a, _ := pinned.EvalUV(0); a != (geom.Vec2{X: 1, Y: 2}) {
						t.Fatalf("%s: a pinned p-curve starts at %v", c.name, a)
					}
					if b, _ := pinned.EvalUV(1); b != (geom.Vec2{X: 3, Y: 4}) {
						t.Fatalf("%s: a pinned p-curve ends at %v", c.name, b)
					}
				}
			}
		}
	}
}

// TestBetweenRefuses holds the pairs that have no such curve to the
// reason given for each.
func TestBetweenRefuses(t *testing.T) {
	for _, c := range []struct {
		name                string
		big, small, d, sine float64
		reason              string
	}{
		{"axes nearly parallel", 5, 2, 0, 1. / 128, "whose axes are too nearly parallel"},
		{"nearly touching from inside", 5, 2, 2.9999, 1, "that nearly touch from inside"},
		{"nearly touching from outside", 5, 2, 6.9999, 1, "that nearly touch"},
		{"touching", 5, 2, 7, 1, "that touch"},
		{"equal radii, axes that cross", 3, 3, 0, 1, "that touch"},
		{"apart", 5, 2, 8, 1, ""},
	} {
		large, small := pair(t, c.d, c.sine)
		if curves, reason := Between(large, c.big, small, c.small, 1e-9, 1e-7); len(curves) != 0 || reason != c.reason {
			t.Errorf("%s: %d curves, reason %q", c.name, len(curves), reason)
		}
	}
}
