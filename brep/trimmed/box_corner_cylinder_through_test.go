package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestBoxCornerCylinderThroughSignedFrames(t *testing.T) {
	ctx := context.Background()
	for _, n := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 32, Y: 16, Z: -8}, n)
		s, e := BoxCornerCylinderThrough(ctx, f, 10, 8, 12, 2, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		v := 960 - 12*math.Pi
		a := 2.*(80+120+96) + (math.Pi/2-2)*24 - 2*math.Pi
		if math.Abs(p.Volume-v) > 1e-7 || math.Abs(p.Area-a) > 1e-7 {
			t.Fatal(p, v, a)
		}
		m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}
