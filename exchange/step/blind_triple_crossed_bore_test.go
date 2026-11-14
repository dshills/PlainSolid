package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func TestBlindTripleCrossedBoreSTEP(t *testing.T) {
	for ni, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		for _, op := range []bool{false, true} {
			for _, depth := range []float64{9, 9.2} {
				t.Run(fmt.Sprintf("%d/%t/%g", ni, op, depth), func(t *testing.T) {
					f, _ := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
					p := TripleCrossedBoreBody{Name: "finite triple", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12}, Crossing: geom.Vec3{X: 10, Y: 8, Z: 6}, Radius: 2, BlindDepth: depth, OppositeCap: op}
					m := TripleCrossedBoreModel{Body: p, LengthUnit: .001}
					var out bytes.Buffer
					if e := WriteTripleCrossedBore(&out, m); e != nil {
						t.Fatal(e)
					}
					if !strings.Contains(out.String(), "ELLIPSE(") || strings.Contains(out.String(), "B_SPLINE") {
						t.Fatal("not exact conics")
					}
					got, e := ReadTripleCrossedBore(bytes.NewReader(out.Bytes()), Limits{})
					if e != nil {
						t.Fatal(e)
					}
					if math.Abs(got.Body.BlindDepth-depth) > 1e-12 {
						t.Fatal("depth", got.Body.BlindDepth, depth)
					}
					canonical := got
					canonical.Body.BlindDepth = depth
					if !reflect.DeepEqual(canonical, m) {
						t.Fatal(canonical, m)
					}
					var repeated bytes.Buffer
					if e = WriteTripleCrossedBore(&repeated, got); e != nil || !bytes.Equal(out.Bytes(), repeated.Bytes()) {
						t.Fatal("noncanonical", e)
					}
					if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
						return
					}
					path, report := filepath.Join(t.TempDir(), "finite.step"), filepath.Join(t.TempDir(), "mass.json")
					if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
						t.Fatal(e)
					}
					stepOracle(t, report, "step-mass-precise", path)
					raw, e := os.ReadFile(report)
					if e != nil {
						t.Fatal(e)
					}
					var oracle struct {
						Volume, Area float64
						Centroid     [3]float64
					}
					if e = json.Unmarshal(raw, &oracle); e != nil {
						t.Fatal(e)
					}
					native, e := trimmed.BoxBlindTripleCrossedBores(context.Background(), f, p.Dimensions, p.Crossing, 2, depth, op, 1e-9)
					if e != nil {
						t.Fatal(e)
					}
					mass, e := native.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if e != nil {
						t.Fatal(e)
					}
					wantV := 3840 - 4*math.Pi*(36+depth) + 64*math.Sqrt2
					wantA := 1504 + 4*math.Pi*(36+depth) - 96*math.Sqrt2 - 16*math.Pi
					if math.Abs(mass.Volume-wantV) > 1e-7 || math.Abs(mass.Area-wantA) > 1e-7 || math.Abs(oracle.Volume-wantV) > 1e-9*wantV || math.Abs(oracle.Area-wantA) > 1e-9*wantA || mass.Centroid.Distance(geom.Vec3{X: oracle.Centroid[0], Y: oracle.Centroid[1], Z: oracle.Centroid[2]}) > 1e-7 {
						t.Fatal(oracle, mass, wantV, wantA)
					}
				})
			}
		}
	}
}
