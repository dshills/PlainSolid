package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestFarTangentCrossedBoreSTEP(t *testing.T) {
	for _, n := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}} {
		f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		if err != nil {
			t.Fatal(err)
		}
		p := CrossedBoreBody{Name: "far tangent floor", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12},
			Crossing: geom.Vec3{X: 10, Y: 8, Z: 6}, Radius: 2, BlindDepth: 8}
		m := CrossedBoreModel{Body: p, LengthUnit: .001}
		var out bytes.Buffer
		if err := WriteCrossedBore(&out, m); err != nil {
			t.Fatal(n, err)
		}
		got, err := ReadCrossedBore(bytes.NewReader(out.Bytes()), Limits{})
		if err != nil || !reflect.DeepEqual(got, m) {
			t.Fatal(n, got, err)
		}
		p.BlindDepth = 4
		var invalid bytes.Buffer
		if err := WriteCrossedBore(&invalid, CrossedBoreModel{Body: p, LengthUnit: .001}); err == nil || invalid.Len() != 0 {
			t.Fatal("nonmanifold near-tangent stop")
		}
		if err := qualifyFarCrossedSTEP(t, &out, f); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFarTangentCrossedBoreSTEPNoDepthSnapping(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	p := CrossedBoreBody{Name: "no depth snapping", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12},
		Crossing: geom.Vec3{X: 10, Y: 8, Z: 6}, Radius: 2, BlindDepth: math.Nextafter(8, math.Inf(1))}
	var out bytes.Buffer
	if err := WriteCrossedBore(&out, CrossedBoreModel{Body: p, LengthUnit: .001}); err == nil || out.Len() != 0 {
		t.Fatal("snapped near contact")
	}
}

func qualifyFarCrossedSTEP(t *testing.T, out *bytes.Buffer, f geom.Frame) error {
	t.Helper()
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		return nil
	}
	dir := t.TempDir()
	source, dest := filepath.Join(dir, "far.step"), filepath.Join(dir, "mass.json")
	if err := os.WriteFile(source, out.Bytes(), 0600); err != nil {
		return err
	}
	stepOracle(t, dest, "step-mass-precise", source)
	data, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	var mass struct {
		Volume, Area float64
		Centroid     [3]float64
	}
	if err := json.Unmarshal(data, &mass); err != nil {
		return err
	}
	wantVolume := 3840 - 112*math.Pi + 128./3
	wantArea := 1504 + 104*math.Pi - 64
	cz := (3840*6 - (80*math.Pi*6 + 32*math.Pi*4 - 128./3*6)) / wantVolume
	wantCenter := f.ToWorld(geom.Vec3{X: 10, Y: 8, Z: cz})
	if math.Abs(mass.Volume-wantVolume) > 1e-8*wantVolume ||
		math.Abs(mass.Area-wantArea) > 1e-8*wantArea ||
		wantCenter.Distance(geom.Vec3{X: mass.Centroid[0], Y: mass.Centroid[1], Z: mass.Centroid[2]}) > 1e-7 {
		t.Fatal("far tangent STEP measures", mass, wantVolume, wantArea, wantCenter)
	}
	return nil
}
