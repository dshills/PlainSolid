package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/geom"
)

func TestPlacedWrappedRimFilletSTEP(t *testing.T) {
	p := WrappedRimFilletBody{Name: "round rim", Center: geom.Vec3{X: 2, Y: 4, Z: 8}, Radius: 3, Height: 5,
		Fillets: [2]float64{.5, 1}, LengthUnit: .001}
	for _, axis := range []geom.Vec3{{X: 1}, {Y: 1}, {Z: 1}} {
		var file bytes.Buffer
		if err := WritePlacedWrappedRimFillet(&file, p, axis); err != nil {
			t.Fatal(axis, err)
		}
		got, direction, err := ReadPlacedWrappedRimFillet(bytes.NewReader(file.Bytes()), Limits{}, WrappedSurfaceOptions{})
		if err != nil || got.Center != p.Center || got.Fillets != p.Fillets || got.Radius != p.Radius ||
			got.Height != p.Height || got.LengthUnit != p.LengthUnit || direction != axis {
			t.Fatal(axis, got, direction, err)
		}
		if os.Getenv("PLAINSOLID_OCCT_STEP") != "1" || os.Getenv("PLAINSOLID_OCCT_ORACLE") == "" {
			continue
		}
		dir := t.TempDir()
		path, report := filepath.Join(dir, "fillet.step"), filepath.Join(dir, "measures.json")
		if err := os.WriteFile(path, file.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		stepOracle(t, report, "step-mass-precise", path)
		data, err := os.ReadFile(report)
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Volume, Area float64
			Centroid     [3]float64
		}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		volume, area := 9*3.5*math.Pi, 2*math.Pi*3*3.5
		for _, r := range p.Fillets {
			major := p.Radius - r
			volume += math.Pi * (major*major*r + major*r*r*math.Pi/2 + 2*r*r*r/3)
			area += math.Pi*major*major + 2*math.Pi*r*(major*math.Pi/2+r)
		}
		c := geom.Vec3{X: m.Centroid[0], Y: m.Centroid[1], Z: m.Centroid[2]}
		relative := c.Sub(p.Center)
		axial := relative.Dot(axis)
		if math.Abs(m.Volume-volume) > 1e-7 || math.Abs(m.Area-area) > 1e-7 ||
			relative.Sub(axis.Scale(axial)).Length() > 1e-7 || axial <= 0 || axial >= p.Height {
			t.Fatal(axis, m, volume, area)
		}
	}
}
