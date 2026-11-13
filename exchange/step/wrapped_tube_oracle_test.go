package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestWrappedTubeIndependentOCCT(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/wrapped-cylinder.step")
	if e != nil {
		t.Fatal(e)
	}
	model, e := ReadWrappedCylindricalBodies(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	outer := model.Bodies[0].Face.Geometry
	inner, _, e := outer.Transformed(ctx, geom.Affine{{.5, 0, 0, 0}, {0, .5, 0, 0}, {0, 0, 1, 0}}, geom.WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	tube, _, e := trimmed.NewWrappedCylinderTube(ctx, outer, inner, 1e-10, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	top := tube.Topology()
	if top.Faces != 4 || top.Edges != 18 || top.Vertices != 16 || len(top.Shells) != 1 || top.EulerCharacteristic != 0 {
		t.Fatal(top)
	}
	mass, e := tube.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	a, _, e := trimmed.NewWrappedCylinder(ctx, outer, 1e-10, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	innerSolid, _, e := trimmed.NewWrappedCylinder(ctx, inner, 1e-10, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	difference, history, e := trimmed.CombineSplineShells(ctx, a, innerSolid, parametric.Difference, trimmed.SplineShellCombineOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if history.Method != "concentric-wrapped-cylinder" {
		t.Fatal(history.Method)
	}
	differenceMass, e := difference.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(differenceMass.Volume-mass.Volume) > 1e-8 || math.Abs(differenceMass.Area-mass.Area) > 1e-8 {
		t.Fatal(differenceMass, mass)
	}
	path := t.TempDir() + "/tube.step"
	stepOracle(t, path, "write-step-wrapped-tube")
	out := t.TempDir() + "/mass.json"
	stepOracle(t, out, "step-mass-precise", path)
	raw, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var reference struct {
		Area, Volume float64
		Centroid     [3]float64
	}
	if e = json.Unmarshal(raw, &reference); e != nil {
		t.Fatal(e)
	}
	if math.Abs(mass.Volume-reference.Volume) > 1e-6 || math.Abs(mass.Area-reference.Area) > 1e-6 || mass.Centroid.Distance(geom.Vec3{X: reference.Centroid[0], Y: reference.Centroid[1], Z: reference.Centroid[2]}) > 1e-7 {
		t.Fatal(mass, reference)
	}
}
