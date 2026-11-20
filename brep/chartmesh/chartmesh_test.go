package chartmesh

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"plainsolid/brep"
	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

var options = boolean.Options{Linear: 1e-7, Angular: 1e-9, Volume: 1e-7}

// stock is a prism 16 wide and deep and 40 tall about the origin, moved
// by m.
func stock(t *testing.T, m geom.Affine) trimmed.Solid {
	t.Helper()
	var points []geom.Vec3
	for _, p := range []geom.Vec2{{X: -8, Y: -8}, {X: 8, Y: -8}, {X: 8, Y: 8}, {X: -8, Y: 8}} {
		points = append(points, m.Apply(geom.Vec3{X: p.X, Y: p.Y, Z: -20}))
	}
	p, err := brep.Prism(points, m.ApplyVector(geom.Vec3{Z: 40}))
	if err != nil {
		t.Fatal(err)
	}
	s, err := trimmed.FromPlanar(p, options.Linear)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// rod is a cylinder of radius r and length 60 about the line through
// centre along axis, moved by m.
func rod(t *testing.T, m geom.Affine, centre, axis geom.Vec3, r float64) trimmed.Solid {
	t.Helper()
	unit, err := axis.Unit()
	if err != nil {
		t.Fatal(err)
	}
	frame, err := geom.FrameFromNormal(m.Apply(centre.Sub(unit.Scale(30))), m.ApplyVector(unit))
	if err != nil {
		t.Fatal(err)
	}
	round, err := parametric.Cylinder(frame, r, 60)
	if err != nil {
		t.Fatal(err)
	}
	s, err := trimmed.FromParametric(round)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// turned is a rigid motion that puts no axis of a test along a
// coordinate axis.
func turned(t *testing.T) geom.Affine {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{X: 3, Y: -2, Z: 5}, geom.Vec3{X: 2, Y: 3, Z: -6}, geom.Vec3{X: 3, Y: -6, Z: -2})
	if err != nil {
		t.Fatal(err)
	}
	x, y, z, o := frame.XAxis(), frame.YAxis(), frame.ZAxis(), frame.Origin()
	return geom.Affine{{x.X, y.X, z.X, o.X}, {x.Y, y.Y, z.Y, o.Y}, {x.Z, y.Z, z.Z, o.Z}}
}

// TestTessellate meshes prisms with two bores across each other, of
// which the pairs of unequal radius and with skew axes meet along curves
// that are not conics. Each mesh is a closed solid, which Tessellate
// itself checks, and its volume is as near the solid's as its facets
// allow at the finer of two facet angles, and nearer than at the coarser.
func TestTessellate(t *testing.T) {
	for _, c := range []struct {
		name                string
		big, small, d, sine float64
	}{
		{"one bore", 5, 0, 0, 1},
		{"equal radii, axes that meet", 3, 3, 0, 1},
		{"unequal radii, right angle", 5, 2, 0, 1},
		{"unequal radii, skew at a slant", 5, 2, 1, 0.6},
		{"the small bore half out of the large one", 5, 3, 4, 1},
		{"equal radii, axes that pass", 3, 3, 2, 1},
	} {
		for _, m := range []geom.Affine{geom.Identity(), turned(t)} {
			body := stock(t, m)
			volume := 0.
			tools := []trimmed.Solid{rod(t, m, geom.Vec3{}, geom.Vec3{Z: 1}, c.big)}
			if c.small > 0 {
				tools = append(tools, rod(t, m, geom.Vec3{Y: c.d}, geom.Vec3{X: c.sine, Z: math.Sqrt(1 - c.sine*c.sine)}, c.small))
			}
			for _, tool := range tools {
				r, err := boolean.Combine(context.Background(), body, tool, boolean.Cut, options)
				if err != nil {
					t.Fatalf("%s: %v", c.name, err)
				}
				body, volume = r.Solid, r.Volume
			}
			var off [2]float64
			for i, angle := range [2]float64{0.2, 0.045} {
				start := time.Now()
				got, err := Tessellate(context.Background(), body, parametric.TessellationOptions{Deflection: 0.05, Angle: angle})
				if err != nil {
					t.Fatalf("%s at %g: %v", c.name, angle, err)
				}
				off[i] = math.Abs(got.SignedVolume()-volume) / volume
				t.Logf("%s at %g: %d triangles, volume off by %.2g of it, %v", c.name, angle, len(got.Triangles), off[i], time.Since(start))
			}
			// A bore meshed in facets that each span the angle a about
			// its axis loses a²/6 of its volume, and a is at most 1.8
			// times the facet angle.
			if off[1] > 1.8*0.045*1.8*0.045/6 || off[1] > off[0] {
				t.Fatalf("%s: volume off by %g of it at the finer angle and %g at the coarser", c.name, off[1], off[0])
			}
		}
	}
}

// TestTessellateRefuses gives Tessellate a solid outside its class: a rod
// as parametric.Cylinder builds it, whose caps are planes in a polar
// chart with a pole.
func TestTessellateRefuses(t *testing.T) {
	_, err := Tessellate(context.Background(), rod(t, geom.Identity(), geom.Vec3{}, geom.Vec3{Z: 1}, 2), parametric.TessellationOptions{Deflection: 0.05, Angle: 0.2})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("%v, want ErrUnsupported", err)
	}
}
