package plaincadbridge

import (
	"math"
	"plainsolid/geom"
	"testing"
)

func TestSignedProfileLineBothAxialEdges(t *testing.T) {
	for _, axes := range [][2]geom.Vec3{{{X: 1}, {Z: 1}}, {{X: 1}, {Y: -1}}, {{Y: 1}, {X: 1}}} {
		for _, symmetric := range []bool{false, true} {
			for _, depth := range []float64{10, 20} {
				f, e := geom.NewFrame(geom.Vec3{}, axes[0], axes[1])
				if e != nil {
					t.Fatal(e)
				}
				height := -depth
				origin := geom.Vec3{}
				if symmetric {
					height = depth
					origin = f.ZAxis().Scale(-depth / 2)
				}
				f, e = geom.NewFrame(origin, axes[0], axes[1])
				if e != nil {
					t.Fatal(e)
				}
				var points []geom.Vec3
				for _, x := range []float64{0, 20} {
					for _, y := range []float64{0, 10} {
						for _, z := range []float64{0, height} {
							points = append(points, f.ToWorld(geom.Vec3{X: x, Y: y, Z: z}))
						}
					}
				}
				bounds, e := geom.Bounds(points)
				if e != nil {
					t.Fatal(e)
				}
				b := New()
				stock := box(t, b, coordinates(bounds.Min), coordinates(bounds.Max))
				var edges []map[string]any
				for _, x := range []float64{0, 20} {
					edges = append(edges, map[string]any{"start": coordinates(f.ToWorld(geom.Vec3{X: x})), "end": coordinates(f.ToWorld(geom.Vec3{X: x, Z: height}))})
				}
				out := shape(t, b, map[string]any{"op": "fillet", "handle": stock.Handle, "normal": coordinates(f.YAxis().Scale(-20)), "radius": 2, "edges": edges})
				near(t, out.GeometryAssertions.Volume, 200*depth-8*depth*(1-math.Pi/4))
				if out.Bounds != stock.Bounds {
					t.Fatal(out.Bounds, stock.Bounds)
				}
			}
		}
	}
}
