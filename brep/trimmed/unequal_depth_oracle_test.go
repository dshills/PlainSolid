package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// These expectations come from closed-form disk/lens integrals, independently
// of the arrangement's reported area and volume. Integrate the retained BRep,
// including first moments, to detect missing ledges or internal slab faces.
func TestUnequalDepthLensIndependentOracle(t *testing.T) {
	base, err := brep.Box(geom.Vec3{X: -10, Y: -10}, geom.Vec3{X: 10, Y: 10, Z: 8})
	if err != nil {
		t.Fatal(err)
	}
	unionArea := 16*math.Pi/3 + 2*math.Sqrt(3)
	unionPerimeter := 16 * math.Pi / 3
	for _, deep := range []float64{6, 8} {
		for _, reverse := range []bool{false, true} {
			cuts := []CylinderCut{
				{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: -1}, Direction: geom.Vec3{Z: 1}, Radius: 2},
				{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 1}, Direction: geom.Vec3{Z: 1}, Radius: 2},
			}
			depths := []float64{3, deep}
			if deep == 8 {
				depths[1] = 0
			}
			if reverse {
				cuts[0], cuts[1] = cuts[1], cuts[0]
				depths[0], depths[1] = depths[1], depths[0]
			}
			body, h, err := BoreCylinderDepthUnion(base, cuts, depths, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			removed := 3*unionArea + (deep-3)*4*math.Pi
			wall := 3*unionPerimeter + (deep-3)*4*math.Pi
			areaDelta := wall
			if deep == 8 {
				areaDelta -= 8 * math.Pi
			}
			unionOracleNear(t, h.RemovedVolume, removed)
			unionOracleNear(t, h.SurfaceAreaDelta, areaDelta)
			mass, err := body.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			volume := 3200 - removed
			unionOracleNear(t, mass.Volume, volume)
			unionOracleNear(t, mass.Area, 1440+areaDelta)
			unionOracleNear(t, mass.Centroid.X, -4*math.Pi*(deep-3)/volume)
			unionOracleNear(t, mass.Centroid.Y, 0)
			unionOracleNear(t, mass.Centroid.Z, (12800-unionArea*9/2-4*math.Pi*(deep*deep-9)/2)/volume)
			mesh, err := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .03, Angle: .1})
			if err != nil {
				t.Fatal(err)
			}
			if err = mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestUnequalDepthCounterboreIndependentOracle(t *testing.T) {
	base, err := brep.Box(geom.Vec3{X: -10, Y: -10}, geom.Vec3{X: 10, Y: 10, Z: 8})
	if err != nil {
		t.Fatal(err)
	}
	cuts := []CylinderCut{
		{Entry: 0, Exit: 1, Direction: geom.Vec3{Z: 1}, Radius: 3},
		{Entry: 0, Exit: 1, Direction: geom.Vec3{Z: 1}, Radius: 1},
	}
	body, h, err := BoreCylinderDepthUnion(base, cuts, []float64{2, 6}, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	unionOracleNear(t, h.RemovedVolume, 22*math.Pi)
	unionOracleNear(t, h.SurfaceAreaDelta, 20*math.Pi)
	floorArea := 0.
	for _, a := range h.FloorAreas {
		floorArea += a
	}
	unionOracleNear(t, floorArea, 9*math.Pi)
	mass, err := body.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	unionOracleNear(t, mass.Volume, 3200-22*math.Pi)
	unionOracleNear(t, mass.Area, 1440+20*math.Pi)
	unionOracleNear(t, mass.Centroid.Z, (12800-34*math.Pi)/(3200-22*math.Pi))
	mesh, err := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .03, Angle: .1})
	if err != nil {
		t.Fatal(err)
	}
	if err = mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}
