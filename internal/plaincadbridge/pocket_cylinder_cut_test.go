package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"testing"
)

func TestPocketFloorCircleCutsAndCounterbore(t *testing.T) {
	for _, kind := range []string{"floor-through", "floor-blind", "counterbore"} {
		b := New()
		stock := box(t, b, vector{}, vector{60, 40, 20})
		pocket := box(t, b, vector{12, 12, 14}, vector{24, 20, 20})
		stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": pocket.Handle})
		hole := circle(t, b, vector{45, 25, 20}, vector{0, 0, -1}, 2, 20)
		stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": hole.Handle})
		var tool Shape
		removed := 0.
		switch kind {
		case "floor-through":
			tool = circle(t, b, vector{18, 16, -1}, vector{0, 0, 1}, 1.5, 22)
			removed = math.Pi * 2.25 * 14
		case "floor-blind":
			tool = circle(t, b, vector{18, 16, 14}, vector{0, 0, -1}, 1, 1)
			removed = math.Pi
		case "counterbore":
			tool = circle(t, b, vector{45, 25, 20}, vector{0, 0, -1}, 3, 1)
			removed = 5 * math.Pi
		}
		out := shape(t, b, map[string]any{"op": "cutMany", "a": stock.Handle, "tools": []string{tool.Handle}})
		near(t, out.GeometryAssertions.Volume, 48000-576-80*math.Pi-removed)
		s := boundary(t, b, out.Handle)
		p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(kind, e)
		}
		near(t, p.Volume, out.GeometryAssertions.Volume)
		near(t, p.Area, out.GeometryAssertions.SurfaceArea)
		m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(kind, e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}
