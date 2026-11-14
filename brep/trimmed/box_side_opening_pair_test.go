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
			{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 8}, Radius: 3}, Depth: depths[0]},
			{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 12}, Radius: 3}, Depth: depths[1]},
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

func TestBoxSideOpeningPairOpposedCaps(t *testing.T) {
	ctx := context.Background()
	f, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	shared := 9*math.Acos(2./3) - math.Sqrt(20)
	for _, depths := range [][2]float64{{2, 4}, {5, 6}, {3, 5}, {8, 6}} {
		cuts := [2]SideOpeningCut{
			{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 8}, Radius: 3}, Depth: depths[0]},
			{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 12}, Radius: 3}, Depth: depths[1], OppositeCap: true},
		}
		result, err := BoxSideOpeningPair(ctx, f, 20, 20, 8, cuts, 1e-9)
		if depths[0]+depths[1] == 8 {
			if err == nil {
				t.Fatal("accepted a zero-thickness contact between opposed floors")
			}
			continue
		}
		if err != nil {
			t.Fatal(depths, err)
		}
		want := 4.5*math.Pi*(depths[0]+depths[1]) - shared*max(0, depths[0]+depths[1]-8)
		props, err := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil || math.Abs(result.RemovedVolume-want) > 1e-7 || math.Abs(props.Volume-(3200-want)) > 1e-7 ||
			math.Abs(props.Area-(1440+result.SurfaceAreaDelta)) > 1e-7 {
			t.Fatal(depths, result.RemovedVolume, props, err)
		}
	}
}

func TestBoxSideOpeningCutsVolumeAndTessellation(t *testing.T) {
	ctx := context.Background()
	f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{X: -1})
	if err != nil {
		t.Fatal(err)
	}
	halfLens := func(d float64) float64 {
		return 9*math.Acos(d/6) - d*math.Sqrt(36-d*d)/4
	}
	for _, tc := range []struct {
		name                       string
		x, depths                  []float64
		opposed                    int
		shared, finalOverlapVolume float64
	}{
		{"three-way overlap", []float64{6, 8, 10}, []float64{5, 5, 5}, -1, 10 * halfLens(2), 5 * (4.5*math.Pi - halfLens(2))},
		{"four depth layers", []float64{6, 10, 14, 18}, []float64{3, 6, 4, 8}, -1, 11 * halfLens(4), 36*math.Pi - 4*halfLens(4)},
		{"opposed cap", []float64{6, 10, 14, 18}, []float64{3, 6, 4, 6}, 3, 9 * halfLens(4), 27*math.Pi - 2*halfLens(4)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cuts []SideOpeningCut
			want := -tc.shared
			for i, depth := range tc.depths {
				cuts = append(cuts, SideOpeningCut{
					ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: tc.x[i]}, Radius: 3},
					Depth:           depth,
					OppositeCap:     i == tc.opposed,
				})
				want += 4.5 * math.Pi * depth
			}
			result, err := BoxSideOpeningCuts(ctx, f, 26, 20, 8, cuts, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			props, err := result.Solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil || math.Abs(result.RemovedVolume-want) > 1e-7 ||
				math.Abs(props.Volume-(4160-want)) > 1e-7 || math.Abs(props.Area-(1776+result.SurfaceAreaDelta)) > 1e-7 {
				t.Fatal(result.RemovedVolume, want, result.SurfaceAreaDelta, props, err)
			}
			final := cuts[len(cuts)-1]
			span := sideOpeningSpan(8, final)
			overlap, err := BoxSideOpeningCutsOverlap(f, 26, 20, 8, cuts[:len(cuts)-1], final.ExtrusionCircle, span[0], span[1], 1e-9)
			if err != nil || math.Abs(overlap-tc.finalOverlapVolume) > 1e-7 {
				t.Fatal(overlap, tc.finalOverlapVolume, err)
			}
			m, err := result.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .1, Angle: .2, MaxSubdivisions: 128, MaxTriangles: 50000})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBoxSideOpeningCutsRetainIslandAndItsFloor(t *testing.T) {
	ctx := context.Background()
	f, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	cuts := []SideOpeningCut{
		{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 4.75, Y: 1}, Radius: 4.2}, Depth: 4},
		{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 4.75, Y: 9}, Radius: 4.2}, Depth: 4},
		{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 10.5, Y: 5}, Radius: 4.2}, Depth: 4},
	}
	result, err := BoxSideOpeningCuts(ctx, f, 12, 10, 8, cuts, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	topology := result.Solid.Topology()
	if len(topology.Shells) != 1 || topology.InnerWires != 1 {
		t.Fatal(topology)
	}
	mesh, err := result.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 128, MaxTriangles: 50000})
	if err != nil {
		t.Fatal(err)
	}
	if err := mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	for i := range cuts {
		cuts[i].Depth = 8
	}
	if _, err := BoxSideOpeningCuts(ctx, f, 12, 10, 8, cuts, 1e-9); err == nil {
		t.Fatal("accepted disconnected through-cut stock")
	}
}
