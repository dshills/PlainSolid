package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func TestPlacedWrappedRimChamferSTEP(t *testing.T) {
	slowtest.Skip(t)
	p := WrappedRimChamferBody{Name: "circular chamfer", Center: geom.Vec3{X: 2, Y: 4, Z: 8}, Radius: 10, Height: 10,
		Setbacks: [2]float64{.5, 1}, LengthUnit: .001}
	for _, axis := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}} {
		p.Axis = axis
		var file bytes.Buffer
		if err := WritePlacedWrappedRimChamfer(&file, p); err != nil {
			t.Fatal(axis, err)
		}
		got, err := ReadPlacedWrappedRimChamfer(bytes.NewReader(file.Bytes()), Limits{}, WrappedSurfaceOptions{})
		if err != nil || got != p {
			t.Fatal(got, err)
		}
		if os.Getenv("PLAINSOLID_OCCT_STEP") != "1" || os.Getenv("PLAINSOLID_OCCT_ORACLE") == "" {
			continue
		}
		dir := t.TempDir()
		path, report := filepath.Join(dir, "chamfer.step"), filepath.Join(dir, "measures.json")
		if err := os.WriteFile(path, file.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		stepOracle(t, report, "step-mass-precise", path)
		data, err := os.ReadFile(report)
		if err != nil {
			t.Fatal(err)
		}
		var m struct{ Volume, Area float64 }
		volume, area := 850*math.Pi, 170*math.Pi
		for _, size := range p.Setbacks {
			a, r := p.Radius, p.Radius-size
			volume += math.Pi * size * (a*a + a*r + r*r) / 3
			area += math.Pi * (r*r + (a+r)*size*math.Sqrt2)
		}
		if err := json.Unmarshal(data, &m); err != nil ||
			math.Abs(m.Volume-volume) > 1e-6 || math.Abs(m.Area-area) > 1e-6 {
			t.Fatal(axis, m, volume, area, err)
		}
	}
}
