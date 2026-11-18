package plaincadbridge

import (
	"context"
	"fmt"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/internal/slowtest"
	"testing"
	"time"
)

func TestPlainCADM3ThreadMesh(t *testing.T) {
	slowtest.Skip(t)
	b := New()
	shaft := shape(t, b, map[string]any{"op": "revolvePolygon", "origin": vector{}, "direction": vector{0, 0, 1}, "angle": 2 * math.Pi, "outer": []vector{{0, 0, -12}, {1.5, 0, -12}, {1.5, 0, 0}, {2.75, 0, 0}, {2.75, 0, 3}, {0, 0, 3}}})
	socket := shape(t, b, map[string]any{"op": "prism", "outer": []vector{{1.443375673, 0, 1.5}, {.7216878365, 1.25, 1.5}, {-.7216878365, 1.25, 1.5}, {-1.443375673, 0, 1.5}, {-.7216878365, -1.25, 1.5}, {.7216878365, -1.25, 1.5}}, "offset": vector{0, 0, 1.5}})
	shaft = shape(t, b, map[string]any{"op": "cut", "a": shaft.Handle, "b": socket.Handle})
	tube := shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{"kind": "disk", "origin": vector{}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0}, "center": []float64{0, 0}, "height": 4, "radius": 3, "circles": []any{map[string]any{"center": []float64{0, 0}, "radius": 1.25}}}})
	for _, v := range []struct {
		side, hand string
		length     float64
	}{{"external", "left", 3}, {"external", "left", 2}, {"internal", "right", 3}} {
		t.Run(fmt.Sprintf("%s-%s-%g", v.side, v.hand, v.length), func(t *testing.T) {
			inner := 1.5 - math.Sqrt(3)*17/48*.5
			a := .5 / 12
			c := a + (1.5-inner)/math.Sqrt(3)
			source, radius := shaft, 1.5
			if v.side == "internal" {
				source, radius = tube, 1.25
				inner = 1.25
				a = .5/16 + (1.5-inner)/math.Sqrt(3)
				c = .5 / 16
			}
			result := shape(t, b, map[string]any{"op": "thread", "handle": source.Handle, "thread": map[string]any{"side": v.side, "hand": v.hand, "radius": radius, "pitch": .5, "length": v.length, "offset": 0, "innerRadius": inner, "outerRadius": 1.5, "innerHalfWidth": a, "outerHalfWidth": c}})
			start := time.Now()
			m, e := b.shapes[result.Handle].analytic.tessellate(parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
			if e != nil {
				t.Fatal(e)
			}
			t.Log("mesh", len(m.Triangles), time.Since(start))
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			// STL and the application validator store float32 coordinates.
			// Closed double-precision topology alone misses thin seam slivers
			// that cross after this required serialization.
			for i, p := range m.Vertices {
				m.Vertices[i].X, m.Vertices[i].Y, m.Vertices[i].Z = float64(float32(p.X)), float64(float32(p.Y)), float64(float32(p.Z))
			}
			report, err := collision.SelfIntersectionsBVHContext(context.Background(), m, collision.Options{MaxPairs: 10000000, MaxContacts: 1})
			if err != nil || !report.Complete || len(report.Contacts) != 0 {
				t.Fatalf("float32 mesh embedding: %v %+v", err, report)
			}
			mass, e := m.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(mass.Volume-result.GeometryAssertions.Volume) >= 2 {
				t.Fatal(mass.Volume, result.GeometryAssertions.Volume)
			}
		})
	}
}
func TestNativeLeadPairDisplay(t *testing.T) {
	for _, hand := range []int{1, -1} {
		for _, nut := range []bool{false, true} {
			t.Run(fmt.Sprintf("hand%d-nut%t", hand, nut), func(t *testing.T) {
				b := New()
				d := leadRequest{Major: 4, Minor: 3, Bore: 3.1, NutOuter: 6, Pitch: 2, Lead: 8, ScrewLength: 18, NutLength: 10, Clearance: .1, Slope: number(math.Tan(math.Pi / 12)), Starts: 4, Hand: hand}
				s, e := b.createLeadPart(d, nut)
				if e != nil {
					t.Fatal(e)
				}
				start := time.Now()
				m, e := b.shapes[s.Handle].analytic.tessellate(parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
				if e != nil {
					t.Fatal(e)
				}
				t.Log("mesh", len(m.Triangles), time.Since(start))
				if e = m.ValidateSolid(); e != nil {
					t.Fatal(e)
				}
				mass, e := m.MassProperties()
				if e != nil {
					t.Fatal(e)
				}
				if math.Abs(mass.Volume-s.GeometryAssertions.Volume) > s.GeometryAssertions.Volume*.01 {
					t.Fatal(mass.Volume, s.GeometryAssertions.Volume)
				}
			})
		}
	}
}
