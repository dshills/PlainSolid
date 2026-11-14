package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestPartialSkewBoreSTEP(t *testing.T) {
	for i, tc := range []struct {
		normal geom.Vec3
		x, z   float64
		offset float64
	}{
		{geom.Vec3{Z: 1}, 5, 4, 5},
		{geom.Vec3{Y: -1}, 4, 5, -5},
		{geom.Vec3{X: -1}, 5, 5, 5},
		{geom.Vec3{Z: -1}, 5, 4, -5.3},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			frame, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: -32, Z: 48}, tc.normal)
			if err != nil {
				t.Fatal(err)
			}
			p := UnequalBoreBody{Name: "partial skew", Frame: frame, Dimensions: geom.Vec3{X: 40, Y: 40, Z: 40},
				Crossing: geom.Vec3{X: 20, Y: 16, Z: 20}, XRadius: tc.x, ZRadius: tc.z, Offset: tc.offset}
			var data bytes.Buffer
			if err := WriteUnequalBore(&data, UnequalBoreModel{Body: p, LengthUnit: .001}); err != nil {
				t.Fatal(err)
			}
			if data.Len() >= 512*1024 || strings.Count(data.String(), "B_SPLINE_CURVE_WITH_KNOTS") != 24 {
				t.Fatal("export must fit the browser and retain three splines for every trim", data.Len())
			}
			m, err := ReadUnequalBore(bytes.NewReader(data.Bytes()), Limits{})
			if err != nil || m.Body.Offset <= 0 {
				t.Fatal(m, err)
			}
			var again bytes.Buffer
			if err := WriteUnequalBore(&again, m); err != nil || !bytes.Equal(again.Bytes(), data.Bytes()) {
				t.Fatal("non-canonical readback", err)
			}
			if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
				return
			}
			dir := t.TempDir()
			source, report := filepath.Join(dir, "partial.step"), filepath.Join(dir, "occt.json")
			if err := os.WriteFile(source, data.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			stepOracle(t, report, "step-mass-precise", source)
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
			native, err := trimmed.BoxPartialSkewBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.XRadius, p.ZRadius, p.Offset, 1e-9)
			if err != nil {
				t.Fatal(err)
			}
			mass, err := native.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10, MaxSubdivisions: 128})
			if err != nil {
				t.Fatal(err)
			}
			center := geom.Vec3{X: oracle.Centroid[0], Y: oracle.Centroid[1], Z: oracle.Centroid[2]}
			if math.Abs(oracle.Volume-mass.Volume) > 2e-6 || math.Abs(oracle.Area-mass.Area) > 2e-6 ||
				center.Distance(mass.Centroid) > 1e-7 {
				t.Fatalf("OCCT readback %+v differs from native %+v", oracle, mass)
			}
		})
	}
}
