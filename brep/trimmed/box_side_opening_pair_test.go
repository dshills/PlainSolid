package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBoxSideOpeningPairDepths(t *testing.T) {
	ctx := context.Background()
	f, err := geom.NewFrame(geom.Vec3{X: 24, Y: 32, Z: 48}, geom.Vec3{Y: 1}, geom.Vec3{X: -1})
	if err != nil {
		t.Fatal(err)
	}
	shared := 9*math.Acos(2./3) - math.Sqrt(20)
	disk := 4.5 * math.Pi
	for _, depths := range [][2]float64{{6, 6}, {2, 6}, {8, 5}, {8, 8}} {
		cuts := [2]SideOpeningCut{
			{ExtrusionCircle{Center: geom.Vec2{X: 8}, Radius: 3}, depths[0]},
			{ExtrusionCircle{Center: geom.Vec2{X: 12}, Radius: 3}, depths[1]},
		}
		result, err := BoxSideOpeningPair(ctx, f, 20, 20, 8, cuts, 1e-9)
		if err != nil {
			t.Fatal(depths, err)
		}
		want := disk*(depths[0]+depths[1]) - shared*min(depths[0], depths[1])
		near := func(got, want float64) {
			t.Helper()
			if math.Abs(got-want) > 1e-7 {
				t.Fatalf("depths %v: got %.12g want %.12g", depths, got, want)
			}
		}
		near(result.RemovedVolume, want)
		props, err := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(depths, err)
		}
		near(props.Volume, 3200-want)
		near(props.Area, 1440+result.SurfaceAreaDelta)
		m, err := result.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, Angle: .2, MaxSubdivisions: 128, MaxTriangles: 50000})
		if err != nil {
			t.Fatal(depths, err)
		}
		if err := m.ValidateSolid(); err != nil {
			t.Fatal(depths, err)
		}
	}
}
