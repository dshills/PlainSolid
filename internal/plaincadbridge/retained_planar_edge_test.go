package plaincadbridge

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"testing"
)

func TestRetainedJoinedXZFilletAndXYChamfer(t *testing.T) {
	for _, h := range []float64{10, 12} {
		for _, fillet := range []bool{true, false} {
			b := New()
			lo, hi := vector{-10, -h, -6}, vector{10, 0, 6}
			tl, th := vector{5, -h, -2}, vector{15, 0, 2}
			start, end, normal := vector{-10, -h, 6}, vector{10, -h, 6}, vector{0, -1, 0}
			op := "fillet"
			key := "radius"
			length := 20.
			section := 1 - math.Pi/4
			if !fillet {
				lo, hi = vector{-10, -6, 0}, vector{10, 6, h}
				tl, th = vector{5, -2, 0}, vector{15, 2, h}
				start, end, normal = vector{-10, -6, 0}, vector{-10, 6, 0}, vector{0, 0, 1}
				op = "chamfer"
				key = "distance"
				length = 12
				section = .5
			}
			base := box(t, b, lo, hi)
			tool := box(t, b, tl, th)
			joined := shape(t, b, map[string]any{"op": "fuse", "a": base.Handle, "b": tool.Handle})
			if !b.shapes[joined.Handle].edgeAvailable(start.vec(), end.vec()) {
				t.Fatal("authored edge unavailable")
			}
			out := shape(t, b, map[string]any{"op": op, "handle": joined.Handle, "edgeStart": start, "edgeEnd": end, "normal": normal, key: 1})
			near(t, out.GeometryAssertions.Volume, 260*h-length*section)
			if out.Bounds != joined.Bounds {
				t.Fatal(out.Bounds, joined.Bounds)
			}
			stored := b.shapes[out.Handle]
			if !fillet {
				if !stored.planar() {
					t.Fatal("exact planar chamfer path was replaced")
				}
				continue
			}
			s := stored.analytic.solid
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
}
