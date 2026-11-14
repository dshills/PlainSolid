package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestBlindCrossedBoreSTEP(t *testing.T) {
	for i, normal := range []geom.Vec3{{Z: 1}, {X: -1}, {Y: 1}} {
		frame, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, normal)
		if err != nil {
			t.Fatal(err)
		}
		for _, opposite := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", i, opposite), func(t *testing.T) {
				p := CrossedBoreBody{Name: "intersecting blind Hole", Frame: frame, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12},
					Crossing: geom.Vec3{X: 8, Y: 7, Z: 6}, Radius: 2, BlindDepth: 9.25, OppositeCap: opposite}
				unit := .001
				if i == 2 {
					unit = .0254
				}
				model := CrossedBoreModel{Body: p, LengthUnit: unit}
				var out bytes.Buffer
				if err := WriteCrossedBore(&out, model); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(out.String(), "ELLIPSE(") || strings.Contains(out.String(), "B_SPLINE_CURVE_WITH_KNOTS(") {
					t.Fatal("expected analytic saddle edges")
				}
				got, err := ReadCrossedBore(bytes.NewReader(out.Bytes()), Limits{})
				if err != nil || !reflect.DeepEqual(got, model) {
					t.Fatal(got, err)
				}
				var again bytes.Buffer
				if err := WriteCrossedBore(&again, got); err != nil || !bytes.Equal(out.Bytes(), again.Bytes()) {
					t.Fatal("distinct boundary after STEP readback", err)
				}
				if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
					return
				}
				dir := t.TempDir()
				path, report := filepath.Join(dir, "blind-crossed.step"), filepath.Join(dir, "mass.json")
				if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
					t.Fatal(err)
				}
				stepOracle(t, report, "step-mass-precise", path)
				raw, err := os.ReadFile(report)
				if err != nil {
					t.Fatal(err)
				}
				var oracle struct {
					Volume, Area float64
					Centroid     [3]float64
				}
				if err := json.Unmarshal(raw, &oracle); err != nil {
					t.Fatal(err)
				}
				native, err := trimmed.BoxBlindCrossedBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.Radius, p.BlindDepth, p.OppositeCap, 1e-9)
				if err != nil {
					t.Fatal(err)
				}
				mass, err := native.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if err != nil {
					t.Fatal(err)
				}
				scaled := unit / .001
				centroid := geom.Vec3{X: oracle.Centroid[0], Y: oracle.Centroid[1], Z: oracle.Centroid[2]}
				if math.Abs(mass.Volume-oracle.Volume/math.Pow(scaled, 3)) > 1e-8*mass.Volume ||
					math.Abs(mass.Area-oracle.Area/math.Pow(scaled, 2)) > 1e-8*mass.Area ||
					mass.Centroid.Distance(centroid.Scale(1/scaled)) > 1e-6 {
					t.Fatal("independent OCCT STEP import", oracle, mass)
				}
			})
		}
	}
}
