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
	"regexp"
	"strings"
	"testing"
)

func TestTripleCrossedBoreSTEP(t *testing.T) {
	for ni, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		frame, e := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		if e != nil {
			t.Fatal(e)
		}
		for ci, c := range []geom.Vec3{{X: 10, Y: 8, Z: 6}, {X: 8, Y: 7, Z: 5}} {
			t.Run(fmt.Sprintf("%d/%d", ni, ci), func(t *testing.T) {
				p := TripleCrossedBoreBody{Name: "perpendicular bores", Frame: frame, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12}, Crossing: c, Radius: 2}
				model := TripleCrossedBoreModel{Body: p, LengthUnit: .001}
				var out bytes.Buffer
				if e := WriteTripleCrossedBore(&out, model); e != nil {
					t.Fatal(e)
				}
				if !strings.Contains(out.String(), "ELLIPSE(") {
					t.Fatal("missing exact intersection ellipses")
				}
				got, e := ReadTripleCrossedBore(bytes.NewReader(out.Bytes()), Limits{})
				if e != nil || !reflect.DeepEqual(got, model) {
					t.Fatal(got, e)
				}
				var second bytes.Buffer
				if e = WriteTripleCrossedBore(&second, got); e != nil || !bytes.Equal(out.Bytes(), second.Bytes()) {
					t.Fatal("noncanonical reload", e)
				}
				if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
					return
				}
				path, report := filepath.Join(t.TempDir(), "crossed.step"), filepath.Join(t.TempDir(), "mass.json")
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
				native, e := trimmed.BoxTripleCrossedBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.Radius, 1e-9)
				if e != nil {
					t.Fatal(e)
				}
				mass, e := native.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil {
					t.Fatal(e)
				}
				wantVolume := 3840. - 192*math.Pi + 64*math.Sqrt2
				wantArea := 1504. + 168*math.Pi - 96*math.Sqrt2
				centroid := geom.Vec3{X: oracle.Centroid[0], Y: oracle.Centroid[1], Z: oracle.Centroid[2]}
				// Compare the independent OCCT evaluator with closed forms and
				// retained native integration at an explicit relative bound.
				if math.Abs(mass.Volume-wantVolume) > 1e-7 || math.Abs(mass.Area-wantArea) > 1e-7 || math.Abs(oracle.Volume-wantVolume) > 1e-9*wantVolume || math.Abs(oracle.Area-wantArea) > 1e-9*wantArea || math.Abs(mass.Volume-oracle.Volume) > 1e-9*wantVolume || math.Abs(mass.Area-oracle.Area) > 1e-9*wantArea || mass.Centroid.Distance(centroid) > 1e-7 {
					t.Fatal("independent OCCT", oracle, mass, wantVolume, wantArea)
				}
			})
		}
	}
}

func TestTripleCrossedBoreSTEPRejectsTampering(t *testing.T) {
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	m := TripleCrossedBoreModel{LengthUnit: .001, Body: TripleCrossedBoreBody{Name: "triple", Frame: f, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12}, Crossing: geom.Vec3{X: 8, Y: 7, Z: 5}, Radius: 2}}
	var out bytes.Buffer
	if e := WriteTripleCrossedBore(&out, m); e != nil {
		t.Fatal(e)
	}
	ellipse := regexp.MustCompile(`ELLIPSE\('',#\d+,[^,]+,[^)]+\)`).FindString(out.String())
	if ellipse == "" {
		t.Fatal("ellipse")
	}
	parts := strings.Split(ellipse, ",")
	parts[2] = "1."
	changed := strings.Replace(out.String(), ellipse, strings.Join(parts, ","), 1)
	if _, e := ReadTripleCrossedBore(strings.NewReader(changed), Limits{}); e == nil {
		t.Fatal("altered ellipse accepted")
	}
	face := regexp.MustCompile(`ADVANCED_FACE\([^;\n]+\)`).FindString(out.String())
	if face == "" {
		t.Fatal("face")
	}
	sense := strings.Replace(face, ",.T.)", ",.F.)", 1)
	if sense == face {
		t.Fatal("sense")
	}
	changed = strings.Replace(out.String(), face, sense, 1)
	if _, e := ReadTripleCrossedBore(strings.NewReader(changed), Limits{}); e == nil {
		t.Fatal("altered face orientation accepted")
	}
}
