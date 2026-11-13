package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"testing"
)

func TestPlacedSideCrossingCircularCut(t *testing.T) {
	for _, x := range []float64{28, 30, 31} {
		b := New()
		base := box(t, b, vector{}, vector{30, 20, 8})
		tool := circle(t, b, vector{x, 7, -1}, vector{0, 0, 1}, 3, 10)
		out := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
		if out.Bounds != base.Bounds {
			t.Fatal("notch changed surviving stock extrema", out.Bounds, base.Bounds)
		}
		a := 30 - x
		area := 9*(math.Pi/2+math.Asin(a/3)) + a*math.Sqrt(9-a*a)
		near(t, out.GeometryAssertions.Volume, 4800-8*area)
		s := b.shapes[out.Handle].analytic.solid
		p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		near(t, p.Volume, out.GeometryAssertions.Volume)
		near(t, p.Area, out.GeometryAssertions.SurfaceArea)
		m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}
