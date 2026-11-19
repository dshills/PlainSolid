//go:build research

package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestSphereCutBox(t *testing.T) {
	ctx := context.Background()
	box := geom.Vec3{X: 8, Y: 6, Z: 4}
	for _, c := range []geom.Vec3{{X: 4, Y: 3, Z: 4}, {X: 4, Y: 3, Z: 4.5}, {X: 4, Y: 3, Z: 3.5}, {X: 4, Y: 3, Z: 2}, {X: 4, Y: 3, Z: 7}} {
		s, h, e := CutSphereFromBox(geom.Vec3{}, box, parametric.Ball{Center: c, Radius: 1}, 1e-9)
		if e != nil {
			t.Fatal(c, e)
		}
		v, a := 192., 208.
		if c.Z >= 3.5 && c.Z <= 4.5 {
			if h.Opening != 1 || len(s.Faces()[1].Holes) != 1 {
				t.Fatal(h)
			}
			cap := 5 - c.Z
			v -= math.Pi * cap * cap * (1 - cap/3)
			a += 2*math.Pi*cap - math.Pi*(2*cap-cap*cap)
		} else if c.Z == 2 {
			v -= 4 * math.Pi / 3
			a += 4 * math.Pi
			h := s.Topology()
			if len(h.Shells) != 2 || h.EulerCharacteristic != 4 || h.CollapsedEdges != 2 {
				t.Fatal(h)
			}
			inner, e := s.Shell(1)
			if e != nil || inner.Topology().EulerCharacteristic != 2 {
				t.Fatal(inner.Topology(), e)
			}
		}
		var buf bytes.Buffer
		if e = WriteJSON(&buf, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&buf, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-v) > 1e-8 || math.Abs(p.Area-a) > 1e-8 {
			t.Fatal(c, p, v, a, e)
		}
		m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05})
		if e != nil || m.ValidateSolid() != nil {
			t.Fatal(c, e)
		}
	}
	if _, _, e := CutSphereFromBox(geom.Vec3{}, box, parametric.Ball{Center: geom.Vec3{X: .25, Y: 3, Z: 4}, Radius: 1}, 1e-9); e == nil {
		t.Fatal("accepted a sphere crossing two faces")
	}
	s, _, e := CutSphereFromBox(geom.Vec3{}, box, parametric.Ball{Center: geom.Vec3{X: 4, Y: 3, Z: 4}, Radius: 1}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []struct {
		z    float64
		want parametric.Location
	}{{2, parametric.Inside}, {3, parametric.Boundary}, {3.5, parametric.Outside}, {4, parametric.Outside}} {
		loc, e := s.Classify(geom.Vec3{X: 4, Y: 3, Z: x.z}, 1e-9)
		if e != nil || loc != x.want {
			t.Fatal(x, loc, e)
		}
	}
	r, e := s.Ray(geom.Vec3{X: 4, Y: 3, Z: 6}, geom.Vec3{Z: -1}, 10, 1e-9)
	if e != nil || len(r) != 2 || math.Abs(r[0].Distance-3) > 1e-9 || math.Abs(r[1].Distance-6) > 1e-9 {
		t.Fatal(r, e)
	}
}
