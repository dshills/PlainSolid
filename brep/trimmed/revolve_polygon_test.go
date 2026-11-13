package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestRevolvePolygonPartial(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{X: 2, Y: 3, Z: 4}, geom.Vec3{Z: 1}, geom.Vec3{X: 1})
	if e != nil {
		t.Fatal(e)
	}
	// Concave L meridian: a 2x2 rectangle less its upper outer unit
	// square. First radial moment is 4-1.5=2.5 with the axis side,
	// or 2.5+3=5.5 after translating the profile one unit outward.
	profile := []geom.Vec2{{}, {X: 2}, {X: 2, Y: 1}, {X: 1, Y: 1}, {X: 1, Y: 2}, {Y: 2}}
	for _, a := range []float64{math.Pi / 2, -math.Pi / 3, 3 * math.Pi / 2, 2 * math.Pi} {
		for _, off := range []bool{false, true} {
			p := append([]geom.Vec2(nil), profile...)
			moment := 2.5
			if off {
				for i := range p {
					p[i].X++
				}
				moment = 5.5
			}
			s, e := RevolvePolygon(ctx, f, p, a)
			if e != nil {
				t.Fatal(a, off, e)
			}
			chi := 2
			if off && a == 2*math.Pi {
				chi = 0
			}
			if s.Topology().EulerCharacteristic != chi {
				t.Fatal(a, off, s.Topology())
			}
			mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
			want := math.Abs(a) * moment
			if e != nil || math.Abs(mass.Volume-want) > 1e-8*want {
				t.Fatal(a, off, mass.Volume, want, e)
			}
			if a != -math.Pi/3 || off {
				continue
			}
			var b bytes.Buffer
			if e := WriteJSON(&b, s); e != nil {
				t.Fatal(e)
			}
			back, e := ReadJSONContext(ctx, &b, JSONLimits{})
			if e != nil || back.Topology().EulerCharacteristic != chi {
				t.Fatal(back.Topology(), e)
			}
			m, e := back.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, MaxTriangles: 32768})
			if e != nil || m.ValidateSolid() != nil || m.SignedVolume() <= 0 {
				t.Fatal("closed analytic revolution mesh", len(m.Triangles), e)
			}
		}
	}
	for _, a := range []float64{0, 1e-18, math.Nextafter(2*math.Pi, 0), 7, math.Inf(1)} {
		if s, e := RevolvePolygon(ctx, f, profile, a); e == nil || len(s.faces) != 0 {
			t.Fatal("invalid angle", a, e)
		}
	}
	x, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := RevolvePolygon(x, f, profile, 1); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
