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

func TestCrossedBoreSTEP(t *testing.T) {
	for ni, n := range []geom.Vec3{{Z: 1}, {Z: -1}, {X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
		frame, e := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, n)
		if e != nil {
			t.Fatal(e)
		}
		for ci, c := range []geom.Vec3{{X: 10, Y: 8, Z: 6}, {X: 8, Y: 7, Z: 5}} {
			t.Run(fmt.Sprintf("%d/%d", ni, ci), func(t *testing.T) {
				p := CrossedBoreBody{Name: "perpendicular bores", Frame: frame, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12}, Crossing: c, Radius: 2}
				model := CrossedBoreModel{Body: p, LengthUnit: .001}
				var out bytes.Buffer
				if e := WriteCrossedBore(&out, model); e != nil {
					t.Fatal(e)
				}
				if !strings.Contains(out.String(), "ELLIPSE(") {
					t.Fatal("missing exact intersection ellipses")
				}
				got, e := ReadCrossedBore(bytes.NewReader(out.Bytes()), Limits{})
				if e != nil || !reflect.DeepEqual(got, model) {
					t.Fatal(got, e)
				}
				var second bytes.Buffer
				if e = WriteCrossedBore(&second, got); e != nil || !bytes.Equal(out.Bytes(), second.Bytes()) {
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
				native, e := trimmed.BoxCrossedBores(context.Background(), p.Frame, p.Dimensions, p.Crossing, p.Radius, 1e-9)
				if e != nil {
					t.Fatal(e)
				}
				mass, e := native.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil {
					t.Fatal(e)
				}
				wantVolume := 3840. - 128*math.Pi + 128./3
				wantArea := 1504. + 112*math.Pi - 64
				centroid := geom.Vec3{X: oracle.Centroid[0], Y: oracle.Centroid[1], Z: oracle.Centroid[2]}
				// OCCT's numerical quadrature over elliptical trims differs from these
				// closed forms by ~2e-10 relative; native integration retains the
				// stricter absolute bound independently of the external oracle.
				if math.Abs(mass.Volume-wantVolume) > 1e-7 || math.Abs(mass.Area-wantArea) > 1e-7 || math.Abs(oracle.Volume-wantVolume) > 1e-9*wantVolume || math.Abs(oracle.Area-wantArea) > 1e-9*wantArea || math.Abs(mass.Volume-oracle.Volume) > 1e-9*wantVolume || math.Abs(mass.Area-oracle.Area) > 1e-9*wantArea || mass.Centroid.Distance(centroid) > 1e-7 {
					t.Fatal("independent OCCT", oracle, mass, wantVolume, wantArea)
				}
			})
		}
	}
}

func TestCrossedBoreSTEPRejectsMalformedEllipse(t *testing.T) {
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	m := CrossedBoreModel{Body: CrossedBoreBody{Name: "ellipse proof", Frame: frame, Dimensions: geom.Vec3{X: 20, Y: 16, Z: 12}, Crossing: geom.Vec3{X: 10, Y: 8, Z: 6}, Radius: 2}, LengthUnit: .001}
	var out bytes.Buffer
	if e := WriteCrossedBore(&out, m); e != nil {
		t.Fatal(e)
	}
	re := regexp.MustCompile(`ELLIPSE\('',#[0-9]+,[^,]+,[^)]+\)`)
	loc := re.FindStringIndex(out.String())
	if loc == nil {
		t.Fatal("no ellipse")
	}
	source := out.String()
	ellipse := source[loc[0]:loc[1]]
	comma := strings.LastIndex(ellipse, ",")
	changed := ellipse[:comma] + ",9.)"
	sense := regexp.MustCompile(`EDGE_CURVE\('[^']*',#[0-9]+,#[0-9]+,#[0-9]+,\.[TF]\.\)`).FindStringIndex(source)
	if sense == nil {
		t.Fatal("no edge orientation")
	}
	edge := source[sense[0]:sense[1]]
	if strings.Contains(edge, ".T.") {
		edge = strings.Replace(edge, ".T.", ".F.", 1)
	} else {
		edge = strings.Replace(edge, ".F.", ".T.", 1)
	}
	if _, e := ReadCrossedBore(strings.NewReader(source[:sense[0]]+edge+source[sense[1]:]), Limits{}); e == nil {
		t.Fatal("accepted wrong edge sense")
	}
	for _, replacement := range []string{changed, strings.Replace(ellipse, "ELLIPSE", "CIRCLE", 1)} {
		bad := source[:loc[0]] + replacement + source[loc[1]:]
		if _, e := ReadCrossedBore(strings.NewReader(bad), Limits{}); e == nil {
			t.Fatal("accepted malformed ellipse", replacement)
		}
	}
}
