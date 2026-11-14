package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestCrossedCompanionBoreSTEP(t *testing.T) {
	for i, n := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}, {Z: -1}} {
		f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		if err != nil {
			t.Fatal(err)
		}
		added := []trimmed.CardinalThroughBore{
			{Axis: 0, Center: geom.Vec3{Y: 32, Z: 32}, Radius: 2},
			{Axis: 1, Center: geom.Vec3{X: 32, Z: 8}, Radius: 2},
			{Axis: 2, Center: geom.Vec3{X: 8, Y: 8}, Radius: 2},
		}
		if i == 0 {
			added = added[:1]
		}
		if i == 1 {
			added = added[:2]
		}
		if i == 3 {
			added = added[:0]
			for j := 0; j < trimmed.MaxCrossedDisjointBores; j++ {
				added = append(added, trimmed.CardinalThroughBore{Axis: 0, Center: geom.Vec3{Y: 2 + float64(j)*.75, Z: 4}, Radius: .25})
			}
		}
		p := CrossedBoreBody{Name: "crossed Holes and separate companions", Frame: f, Dimensions: geom.Vec3{X: 40, Y: 40, Z: 40}, Crossing: geom.Vec3{X: 20, Y: 20, Z: 20}, Radius: 3, Additional: added}
		m := CrossedBoreModel{Body: p, LengthUnit: .001}
		var out bytes.Buffer
		if err := WriteCrossedBore(&out, m); err != nil {
			t.Fatal(n, err)
		}
		got, err := ReadCrossedBore(bytes.NewReader(out.Bytes()), Limits{})
		if err != nil || !reflect.DeepEqual(got, m) {
			t.Fatal(n, got, err)
		}
		var again bytes.Buffer
		if err := WriteCrossedBore(&again, got); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
			t.Fatal(n, err)
		}
		if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
			continue
		}
		dir := t.TempDir()
		source, dest := filepath.Join(dir, "multi.step"), filepath.Join(dir, "mass.json")
		if err := os.WriteFile(source, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		stepOracle(t, dest, "step-mass-precise", source)
		raw, err := os.ReadFile(dest)
		if err != nil {
			t.Fatal(err)
		}
		var mass struct{ Volume, Area float64 }
		if err := json.Unmarshal(raw, &mass); err != nil {
			t.Fatal(err)
		}
		wantVolume := 64000 - 720*math.Pi + 144
		wantArea := 9600 + 444*math.Pi - 144
		for _, site := range added {
			wantVolume -= math.Pi * site.Radius * site.Radius * 40
			wantArea += 2 * math.Pi * site.Radius * (40 - site.Radius)
		}
		if math.Abs(mass.Volume-wantVolume) > 1e-8*wantVolume || math.Abs(mass.Area-wantArea) > 1e-8*wantArea {
			t.Fatal(n, mass, wantVolume, wantArea)
		}
	}
}
