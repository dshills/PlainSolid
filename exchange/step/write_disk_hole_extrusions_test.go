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

func diskExtrusionFixture(t *testing.T, height float64) DiskHoleExtrusionModel {
	t.Helper()
	f, e := geom.NewFrame(geom.Vec3{X: 2, Y: 5, Z: -3}, geom.Vec3{Z: 1}, geom.Vec3{X: -1})
	if e != nil {
		t.Fatal(e)
	}
	return DiskHoleExtrusionModel{LengthUnit: .001, Bodies: []DiskHoleExtrusionBody{{
		Name: "eccentric pockets", Frame: f, Height: height,
		Outer: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 1, Y: 2}, Radius: 4},
		Holes: []trimmed.ExtrusionCircle{{Center: geom.Vec2{X: -1, Y: 2}, Radius: .75}},
		Blind: []trimmed.ExtrusionBlindHole{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 3, Y: 2}, Radius: 1}, Depth: 1.25}},
	}}}
}

func TestWriteDiskHoleExtrusionsGraph(t *testing.T) {
	for _, z := range []float64{3, -3} {
		model := diskExtrusionFixture(t, z)
		var out bytes.Buffer
		if e := WriteDiskHoleExtrusions(&out, model); e != nil {
			t.Fatal(e)
		}
		f, e := Parse(&out, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		counts := map[string]int{}
		var outer, inward int
		for id, en := range f.Entities {
			for _, r := range en.Records {
				counts[r.Type]++
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
				sense, e := boolean(face.Args[3])
				if e != nil {
					t.Fatal(e)
				}
				if sense {
					outer++
					if !uses[0].sense || !uses[1].sense || uses[2].sense || uses[3].sense || uses[1].id != uses[3].id {
						t.Fatal("bad exterior cylinder orientation or seam", uses)
					}
				} else {
					inward++
				}
			}
		}
		if outer != 1 || inward != 2 || counts["MANIFOLD_SOLID_BREP"] != 1 || counts["ADVANCED_FACE"] != 6 ||
			counts["PLANE"] != 3 || counts["CIRCLE"] != 12 || counts["SEAM_CURVE"] != 3 || counts["PCURVE"] != 18 {
			t.Fatal(outer, inward, counts)
		}
	}
	invalid := diskExtrusionFixture(t, 3)
	invalid.Bodies[0].Blind[0].Radius = 2 // tangent to the outer wall
	var out bytes.Buffer
	if e := WriteDiskHoleExtrusions(&out, invalid); e == nil || out.Len() != 0 {
		t.Fatal("tangent pocket emitted", e, out.Len())
	}
}

func TestWriteDiskHoleExtrusionsOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_STEP") == "" {
		t.Skip("optional OCCT")
	}
	for _, z := range []float64{3, -3} {
		for _, voids := range []bool{true, false} {
			m := diskExtrusionFixture(t, z)
			if !voids {
				m.Bodies[0].Blind = nil
				m.Bodies[0].Holes = nil
			}
			b := m.Bodies[0]
			s, e := trimmed.ExtrudeDiskWithHoles(context.Background(), b.Frame, b.Outer, b.Holes, b.Blind, b.Height)
			if e != nil {
				t.Fatal(e)
			}
			p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil {
				t.Fatal(e)
			}
			var out bytes.Buffer
			if e = WriteDiskHoleExtrusions(&out, m); e != nil {
				t.Fatal(e)
			}
			path := filepath.Join(t.TempDir(), "disk.step")
			if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			report := filepath.Join(t.TempDir(), "mass.json")
			stepOracle(t, report, "step-mass-precise", path)
			raw, e := os.ReadFile(report)
			if e != nil {
				t.Fatal(e)
			}
			var got struct {
				Area, Volume float64
				Centroid     [3]float64
			}
			if e = json.Unmarshal(raw, &got); e != nil {
				t.Fatal(e)
			}
			want := 48 * math.Pi
			if voids {
				want -= (3*.75*.75 + 1.25) * math.Pi
			}
			if math.Abs(got.Volume-want) > 1e-8 || math.Abs(got.Volume-p.Volume) > 1e-8 || math.Abs(got.Area-p.Area) > 1e-8 ||
				p.Centroid.Distance(geom.Vec3{X: got.Centroid[0], Y: got.Centroid[1], Z: got.Centroid[2]}) > 1e-8 {
				t.Fatal(z, voids, got, p, want)
			}
		}
	}
}
