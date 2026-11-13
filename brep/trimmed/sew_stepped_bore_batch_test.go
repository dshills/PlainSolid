package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestSewSteppedBoreBatch(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		frame, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: -32, Z: 48}, axis)
		stock, e := ExtrudeRoundedRectangle(ctx, frame, RoundedRectangle{Min: geom.Vec2{X: -20, Y: -20}, Max: geom.Vec2{X: 20, Y: 20}, Radii: [4]float64{2, 2, 2, 2}}, nil, nil, 10)
		if e != nil {
			t.Fatal(e)
		}
		sites := []PlanarSteppedBore{{LowerCap: 0, UpperCap: 1, Center: geom.Vec2{X: -10, Y: .2}, Steps: []AxialBoreStep{{3, 0}, {4, 2}}}, {LowerCap: -1, UpperCap: 1, Center: geom.Vec2{X: 10, Y: -.2}, Steps: []AxialBoreStep{{2, 6}, {4, 2}}}}
		before := stock.Faces()
		result, e := SewPlanarSteppedBores(ctx, stock, sites)
		if e != nil {
			t.Fatal(axis, e)
		}
		if !reflect.DeepEqual(before, stock.Faces()) {
			t.Fatal("mutated source")
		}
		m, e := result.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		removed := math.Pi * (9*10 + (16-9)*2 + 4*6 + (16-4)*2)
		want := (1600-4*(4-math.Pi))*10 - removed
		if math.Abs(m.Volume-want) > 1e-6 {
			t.Fatal(m.Volume, want)
		}
		mesh, e := result.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		sites[0], sites[1] = sites[1], sites[0]
		reverse, e := SewPlanarSteppedBores(ctx, stock, sites)
		if e != nil {
			t.Fatal(e)
		}
		mp, e := reverse.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil || math.Abs(mp.Volume-m.Volume) > 1e-6 || math.Abs(mp.Area-m.Area) > 1e-6 {
			t.Fatal(mp, m, e)
		}
		sites[1].Center = sites[0].Center
		rejected, e := SewPlanarSteppedBores(ctx, stock, sites)
		if e == nil || len(rejected.Faces()) != 0 {
			t.Fatal("accepted overlapping batch", e)
		}
		if !reflect.DeepEqual(before, stock.Faces()) {
			t.Fatal("failed batch mutated source")
		}
		sites[1].Center = geom.Vec2{X: sites[0].Center.X - 8, Y: sites[0].Center.Y}
		if _, e := SewPlanarSteppedBores(ctx, stock, sites); e == nil {
			t.Fatal("accepted tangent envelopes")
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if _, e := SewPlanarSteppedBores(canceled, stock, sites); e != context.Canceled {
			t.Fatal(e)
		}

	}
}
