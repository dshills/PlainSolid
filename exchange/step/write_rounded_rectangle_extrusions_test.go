package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func roundedSTEPFixture(t *testing.T, height float64) RoundedRectangleExtrusionModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 2, Y: -3, Z: 4}, geom.Vec3{Y: 1}, geom.Vec3{X: -1})
	if e != nil {
		t.Fatal(e)
	}
	return RoundedRectangleExtrusionModel{LengthUnit: .001, Bodies: []RoundedRectangleExtrusionBody{{
		Name: "mixed profile", Frame: f, Height: height,
		Profile: trimmed.RoundedRectangle{Max: geom.Vec2{X: 8, Y: 5}, Radii: [4]float64{.5, 1, 0, 1.5}},
		Holes:   []trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 2, Y: 2}, Radius: .5}},
		Blind:   []trimmed.ExtrusionBlindHole{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 3}, Radius: .75}, Depth: 1}},
	}}}
}

func TestWriteRoundedRectangleExtrusionsGraph(t *testing.T) {
	var out bytes.Buffer
	model := roundedSTEPFixture(t, -3)
	if e := WriteRoundedRectangleExtrusions(&out, model); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(&out, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var exterior, inward int
	for id, en := range f.Entities {
		for _, r := range en.Records {
			if r.Type != "ADVANCED_FACE" {
				continue
			}
			surface, e := ref(r.Args[2])
			if e != nil {
				t.Fatal(e)
			}
			if f.Entities[surface].Records[0].Type != "CYLINDRICAL_SURFACE" {
				continue
			}
			face, uses, e := analyticFaceLoop(f, id, 4)
			if e != nil {
				t.Fatal(e)
			}
			outside, e := boolean(face.Args[3])
			if e != nil {
				t.Fatal(e)
			}
			if outside {
				exterior++
				for _, k := range []int{0, 2} {
					a, b, g, e := analyticEdge(f, uses[k].id)
					if e != nil || a == b {
						t.Fatal("quarter arc lost its distinct endpoints", e)
					}
					s, e := simple(f, g, "SURFACE_CURVE", 4)
					if e != nil {
						t.Fatal(e)
					}
					circle, e := ref(s.Args[1])
					if e != nil {
						t.Fatal(e)
					}
					if _, e := simple(f, circle, "CIRCLE", 3); e != nil {
						t.Fatal(e)
					}
				}
			} else {
				inward++
			}
		}
	}
	if exterior != 3 || inward != 2 {
		t.Fatal(exterior, inward)
	}
	model.Bodies[0].Profile.Radii = [4]float64{4, 4}
	if e := WriteRoundedRectangleExtrusions(&out, model); e == nil || out.Len() != 0 {
		t.Fatal("invalid profile produced output", e, out.Len())
	}
}

func TestWriteRoundedRectangleExtrusionsOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT")
	}
	for _, z := range []float64{-3, 3} {
		model := roundedSTEPFixture(t, z)
		b := model.Bodies[0]
		s, e := trimmed.ExtrudeRoundedRectangle(context.Background(), b.Frame, b.Profile, b.Holes, b.Blind, b.Height)
		if e != nil {
			t.Fatal(e)
		}
		mass, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteRoundedRectangleExtrusions(&out, model); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "rounded.step")
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		report := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, report, "step-mass-precise", path)
		data, e := os.ReadFile(report)
		if e != nil {
			t.Fatal(e)
		}
		var got struct {
			Area, Volume float64
			Centroid     [3]float64
		}
		if e = json.Unmarshal(data, &got); e != nil {
			t.Fatal(e)
		}
		area := 40 - (1-math.Pi/4)*3.5
		want := 3*(area-math.Pi*.25) - math.Pi*.5625
		if math.Abs(got.Volume-want) > 1e-8 || math.Abs(got.Area-mass.Area) > 1e-8 ||
			mass.Centroid.Distance(geom.Vec3{X: got.Centroid[0], Y: got.Centroid[1], Z: got.Centroid[2]}) > 1e-8 {
			t.Fatal(z, got, mass, want)
		}
	}
}
