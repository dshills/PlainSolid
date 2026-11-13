package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func unionOracleNear(t *testing.T, actual, expected float64) {
	t.Helper()
	if math.Abs(actual-expected) > 1e-7*math.Max(1, math.Abs(expected)) {
		t.Fatalf("got %.15g, expected %.15g", actual, expected)
	}
}

// Compare the retained cut boundary against both a closed-form lens and the
// independent parametric cylinder-union construction, not mesh-derived volume.
func TestBoreUnionIndependentCylinderOracle(t *testing.T) {
	base, err := brep.Box(geom.Vec3{X: -10, Y: -10}, geom.Vec3{X: 10, Y: 10, Z: 8})
	if err != nil {
		t.Fatal(err)
	}
	baseMass, err := base.MassProperties()
	if err != nil {
		t.Fatal(err)
	}
	frame, err := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	// Equal radius2 circles separated by2 have lens area 8*pi/3-2*sqrt(3).
	area, perimeter := 16*math.Pi/3+2*math.Sqrt(3), 16*math.Pi/3
	for _, depth := range []float64{0, 3} {
		height := depth
		if height == 0 {
			height = 8
		}
		cuts := []CylinderCut{
			{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: -1}, Direction: geom.Vec3{Z: 1}, Radius: 2},
			{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 1}, Direction: geom.Vec3{Z: 1}, Radius: 2},
		}
		body, history, err := BoreCylinderUnion(base, cuts, depth, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		unionOracleNear(t, history.Area, area)
		unionOracleNear(t, history.Perimeter, perimeter)
		actual, err := body.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(err)
		}
		tool, err := parametric.CombineParallelCylinders(frame,
			parametric.DiskProfile{Center: geom.Vec2{X: -1}, Radius: 2},
			parametric.DiskProfile{Center: geom.Vec2{X: 1}, Radius: 2}, height, parametric.Union, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		oracle, err := tool.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(err)
		}
		unionOracleNear(t, baseMass.Volume-actual.Volume, oracle.Volume)
		unionOracleNear(t, actual.Volume, baseMass.Volume-area*height)
		wantArea := baseMass.SurfaceArea + perimeter*height
		if depth == 0 {
			wantArea -= 2 * area
		}
		unionOracleNear(t, actual.Area, wantArea)
		unionOracleNear(t, oracle.Area, 2*area+perimeter*height)
		m, err := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .03, Angle: .1})
		if err != nil {
			t.Fatal(err)
		}
		if err = m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBoreUnionThreeCircleChainPermutationOracle(t *testing.T) {
	base, err := brep.Box(geom.Vec3{X: -10, Y: -10}, geom.Vec3{X: 10, Y: 10, Z: 8})
	if err != nil {
		t.Fatal(err)
	}
	alpha := math.Acos(.75)
	lens := 8*alpha - 1.5*math.Sqrt(7)
	area, perimeter := 12*math.Pi-2*lens, 12*math.Pi-16*alpha
	for _, order := range [][3]int{{0, 1, 2}, {2, 0, 1}, {1, 2, 0}, {2, 1, 0}} {
		var cuts []CylinderCut
		for _, index := range order {
			cuts = append(cuts, CylinderCut{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: float64(index-1) * 3}, Direction: geom.Vec3{Z: 1}, Radius: 2})
		}
		body, history, err := BoreCylinderUnion(base, cuts, 0, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		unionOracleNear(t, history.Area, area)
		unionOracleNear(t, history.Perimeter, perimeter)
		p, err := body.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(err)
		}
		unionOracleNear(t, p.Volume, 3200-area*8)
		unionOracleNear(t, p.Area, 1440-2*area+perimeter*8)
	}
}
