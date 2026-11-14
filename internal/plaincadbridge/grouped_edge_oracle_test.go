package plaincadbridge

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// Match the original edit fixture's XZ extrusion. First moments distinguish
// physically different edge selections that remove the same total volume.
func TestGroupedEdgeEditAnalyticOracle(t *testing.T) {
	b := New()
	point := func(x, y, z float64) vector { return vector{x, -z, y} }
	stock := box(t, b, vector{0, -10, 0}, vector{20, 0, 10})
	corners := [4][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	treat := func(base Shape, op string, size float64, indices ...int) Shape {
		t.Helper()
		edges := make([]map[string]vector, 0, len(indices))
		for _, i := range indices {
			y, z := corners[i][0], corners[i][1]
			edges = append(edges, map[string]vector{"start": point(0, y, z), "end": point(20, y, z)})
		}
		return shape(t, b, map[string]any{"op": op, "handle": base.Handle, "edges": edges, "normal": vector{0, -1, 0}, "radius": size, "distance": size})
	}
	for _, tc := range []struct {
		name            string
		radius, setback float64
		rounds, bevels  []int
	}{
		{"initial", 1, .5, []int{3, 2}, []int{0}},
		{"edited", .5, 1, []int{3, 1}, []int{0}},
		{"reversed-edited", .5, 1, []int{1, 3}, []int{0}},
		{"grouped-chamfer", 1, 1.5, []int{3, 2}, []int{1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rounded := treat(stock, "fillet", tc.radius, tc.rounds...)
			beveled := treat(rounded, "chamfer", tc.setback, tc.bevels...)
			area, momentY, momentZ := 100., 500., 500.
			remove := func(i int, removed, inwardMoment float64) {
				y, z := corners[i][0], corners[i][1]
				area -= removed
				momentY -= y*removed + (1-y/5)*inwardMoment
				momentZ -= z*removed + (1-z/5)*inwardMoment
			}
			for _, i := range tc.rounds {
				r := tc.radius
				remove(i, (1-math.Pi/4)*r*r, (5./6-math.Pi/4)*r*r*r)
			}
			near(t, rounded.GeometryAssertions.Volume, 20*area)
			for _, i := range tc.bevels {
				d := tc.setback
				remove(i, d*d/2, d*d*d/6)
			}
			near(t, beveled.GeometryAssertions.Volume, 20*area)
			if beveled.Bounds != stock.Bounds {
				t.Fatal("grouped treatment changed the rectangular stock extrema", beveled.Bounds)
			}
			retained := b.shapes[beveled.Handle]
			props, err := retained.treated.chain.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			near(t, props.Volume, 20*area)
			near(t, props.Area, beveled.GeometryAssertions.SurfaceArea)
			center := retained.treated.frame.ToWorld(props.Centroid)
			want := geom.Vec3{X: 10, Y: -momentZ / area, Z: momentY / area}
			if center.Sub(want).Length() > 1e-7 {
				t.Fatalf("wrong retained centroid: %v, want %v", center, want)
			}
			mesh, err := retained.tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if math.Abs(mesh.SignedVolume()/props.Volume-1) > .001 {
				t.Fatal("mesh does not agree with analytic volume")
			}
		})
	}
	near(t, shape(t, b, map[string]any{"op": "inspect", "handle": stock.Handle}).GeometryAssertions.Volume, 2000)
}
