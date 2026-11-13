package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func nativeSteppedCylinder(t *testing.T, reversed bool) parametric.Solid {
	t.Helper()
	a := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Axis: geom.Vec3{Z: 2}, Radius: 5, Min: -2, Max: 2}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 1, Y: 2, Z: 8}, Axis: geom.Vec3{Z: 3}, Radius: 2, Min: -1, Max: 1}
	if reversed {
		a.Axis = a.Axis.Scale(-1)
	}
	out, err := parametric.FiniteCylinderPairUnion(context.Background(), a, b, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	return out.Solid
}

func TestNativeSteppedCylinderSTEP(t *testing.T) {
	ctx := context.Background()
	for _, reversed := range []bool{false, true} {
		s := nativeSteppedCylinder(t, reversed)
		var native, out bytes.Buffer
		if err := parametric.WriteJSON(&native, s); err != nil {
			t.Fatal(err)
		}
		s, err := parametric.ReadJSON(&native, parametric.JSONLimits{})
		if err != nil {
			t.Fatal(err)
		}
		if err = WriteNativeSteppedCylinder(&out, s, "retained step", .001); err != nil {
			t.Fatal(reversed, err)
		}
		model, err := ReadAnalyticBodies(&out, Limits{})
		if err != nil || len(model.Bodies) != 1 || model.LengthUnit != .001 || model.Bodies[0].Name != "retained step" {
			t.Fatal(model, err)
		}
		m, err := model.Bodies[0].Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if err != nil || math.Abs(m.Area-146*math.Pi) > 1e-8 || math.Abs(m.Volume-216*math.Pi) > 1e-8 || math.Abs(m.Centroid.Z-744.0/216) > 1e-9 {
			t.Fatal(m, err)
		}
	}
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	cylinder, _ := parametric.Cylinder(frame, 1, 2)
	var out bytes.Buffer
	if err := WriteNativeSteppedCylinder(&out, cylinder, "unsupported", .001); err == nil || out.Len() != 0 {
		t.Fatal("unsupported boundary published", err)
	}
}

func TestOCCTNativeSteppedCylinderSTEP(t *testing.T) {
	var data bytes.Buffer
	if err := WriteNativeSteppedCylinder(&data, nativeSteppedCylinder(t, false), "retained step", .001); err != nil {
		t.Fatal(err)
	}
	input, output := filepath.Join(t.TempDir(), "step.stp"), filepath.Join(t.TempDir(), "mass.json")
	if err := os.WriteFile(input, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	stepOracle(t, output, input)
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var p struct{ Area, Volume float64 }
	if err = json.Unmarshal(raw, &p); err != nil || math.Abs(p.Area-146*math.Pi) > 1e-7 || math.Abs(p.Volume-216*math.Pi) > 1e-7 {
		t.Fatal(p, err)
	}
}
