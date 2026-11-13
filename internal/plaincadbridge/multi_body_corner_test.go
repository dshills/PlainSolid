package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"strings"
	"testing"
)

func TestMultiBodyFiniteCornerRevolveCuts(t *testing.T) {
	for _, r := range []float64{2, 3} {
		b := New()
		tool := circle(t, b, vector{}, vector{0, 1, 0}, r, 40)
		for _, y := range []float64{0, 20} {
			base := box(t, b, vector{0, y, 0}, vector{10, y + 10, 10})
			cut := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
			near(t, cut.GeometryAssertions.Volume, 1000-2.5*math.Pi*r*r)
			s := b.shapes[cut.Handle].analytic.solid
			p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			near(t, p.Volume, cut.GeometryAssertions.Volume)
			near(t, p.Area, cut.GeometryAssertions.SurfaceArea)
			m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
		}
		far := box(t, b, vector{100, 0, 0}, vector{110, 10, 10})
		before := len(b.shapes)
		reply := b.Call(`{"op":"cut","a":"` + far.Handle + `","b":"` + tool.Handle + `"}`)
		if !strings.Contains(reply, "removed no volume") || len(b.shapes) != before {
			t.Fatal(reply)
		}
		if _, e := b.get(far.Handle); e != nil {
			t.Fatal(e)
		}
	}
}
func TestMultiBodyExtrudeDisjointNoop(t *testing.T) {
	b := New()
	base := box(t, b, vector{100, 0, 0}, vector{120, 10, 10})
	tool := circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 2, 40)
	before := len(b.shapes)
	reply := b.Call(`{"op":"cut","a":"` + base.Handle + `","b":"` + tool.Handle + `"}`)
	if !strings.Contains(reply, "removed no volume") || len(b.shapes) != before {
		t.Fatal(reply)
	}
}
func TestCornerThroughRejectsFiniteShortTool(t *testing.T) {
	b := New()
	base := box(t, b, vector{0, 20, 0}, vector{10, 30, 10})
	tool := circle(t, b, vector{}, vector{0, 1, 0}, 2, math.Nextafter(30, 0))
	before := len(b.shapes)
	reply := b.Call(`{"op":"cut","a":"` + base.Handle + `","b":"` + tool.Handle + `"}`)
	if !strings.Contains(reply, `"code":"UNSUPPORTED"`) || len(b.shapes) != before {
		t.Fatal(reply)
	}
}
func TestCornerThroughReversedFiniteAxis(t *testing.T) {
	b := New()
	base := box(t, b, vector{0, 20, 0}, vector{10, 30, 10})
	tool := circle(t, b, vector{0, 40, 0}, vector{0, -1, 0}, 2, 40)
	out := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
	near(t, out.GeometryAssertions.Volume, 1000-10*math.Pi)
	if out.Bounds != base.Bounds {
		t.Fatal(out.Bounds)
	}
}
