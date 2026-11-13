package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"strings"
	"testing"
)

func TestEditedHoleRetainsDownstreamChamfer(t *testing.T) {
	for _, tc := range []struct{ r, h, w float64 }{{2, 10, 85}, {2.5, 10, 127.75}, {2.5, 3, 40.25}} {
		b := New()
		stock := box(t, b, vector{}, vector{20, 10, 10})
		for _, x := range []float64{5, 15} {
			tool := circle(t, b, vector{x, 5, 0}, vector{0, 0, 1}, tc.r, tc.h)
			stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
		}
		tool := circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 3, 1)
		stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
		out := shape(t, b, map[string]any{"op": "chamfer", "handle": stock.Handle, "edgeStart": vector{0, 0, 0}, "edgeEnd": vector{20, 0, 0}, "normal": vector{0, 0, 1}, "distance": .5})
		near(t, out.GeometryAssertions.Volume, 2000-tc.w*math.Pi-2.5)
		s := b.shapes[out.Handle].analytic.solid
		p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
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

func TestRetainedCavityEdgeTreatmentRejectsContactAtomically(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 10, 10})
	for _, x := range []float64{5, 15} {
		tool := circle(t, b, vector{x, 2, 0}, vector{0, 0, 1}, 1, 3)
		stock = shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
	}
	before := len(b.shapes)
	reply := b.Call(`{"op":"chamfer","handle":"` + stock.Handle + `","edgeStart":[0,0,0],"edgeEnd":[20,0,0],"normal":[0,0,1],"distance":1}`)
	if !strings.Contains(reply, `"code":"UNSUPPORTED"`) || len(b.shapes) != before {
		t.Fatal(reply)
	}
	if b.shapes[stock.Handle].bore == nil {
		t.Fatal("lost input owner")
	}
}
