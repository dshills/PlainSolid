package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plainsolid/geom"
)

func TestQuarterAnnularExtrusionSTEP(t *testing.T) {
	for _, normal := range []geom.Vec3{{Z: 1}, {Y: 1}, {Y: -1}} {
		for _, unit := range []float64{.001, .01} {
			f, err := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, normal)
			if err != nil {
				t.Fatal(err)
			}
			p := QuarterAnnularExtrusionBody{Name: "annular quadrant", Frame: f, Inner: 2, Radius: 5, Height: 10}
			m := QuarterAnnularExtrusionModel{Body: p, LengthUnit: unit}
			var file bytes.Buffer
			if err := WriteQuarterAnnularExtrusion(&file, m); err != nil {
				t.Fatal(err)
			}
			raw := file.Bytes()
			got, err := ReadQuarterAnnularExtrusion(bytes.NewReader(raw), Limits{})
			if err != nil || got != m {
				t.Fatal(got, err)
			}
			changed := strings.Replace(string(raw), "CYLINDRICAL_SURFACE", "CONICAL_SURFACE", 1)
			if _, err := ReadQuarterAnnularExtrusion(strings.NewReader(changed), Limits{}); err == nil {
				t.Fatal("accepted a substituted wall")
			}
			if os.Getenv("PLAINSOLID_OCCT_STEP") == "1" && os.Getenv("PLAINSOLID_OCCT_ORACLE") != "" {
				dir := t.TempDir()
				path, report := filepath.Join(dir, "sector.step"), filepath.Join(dir, "measures.json")
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				stepOracle(t, report, "step-mass-precise", path)
				got, err := os.ReadFile(report)
				if err != nil {
					t.Fatal(err)
				}
				var measure struct{ Volume, Area float64 }
				if err := json.Unmarshal(got, &measure); err != nil {
					t.Fatal(err)
				}
				scale := unit / .001
				if math.Abs(measure.Volume/(scale*scale*scale)-52.5*math.Pi) > 1e-7 ||
					math.Abs(measure.Area/(scale*scale)-(45.5*math.Pi+60)) > 1e-7 {
					t.Fatal(measure)
				}
			}
		}
	}
}
