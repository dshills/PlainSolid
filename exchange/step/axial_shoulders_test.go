package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func shoulderModel(t *testing.T, collar bool) AxialProfileModel {
	m := axialModel(t)
	end, mid := 2., 1.
	if collar {
		end, mid = 1, 2
	}
	m.Bodies[0].Sections = []CylinderSection{{end, 1}, {mid, 2}, {end, 1}}
	return m
}
func checkShoulders(t *testing.T, m AnalyticModel, collar bool) {
	t.Helper()
	if len(m.Bodies) != 1 {
		t.Fatal(m)
	}
	p, e := m.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	area := 26 * math.Pi
	if collar {
		area = 20 * math.Pi
	}
	if e != nil || math.Abs(p.Volume-10*math.Pi) > 1e-7 || math.Abs(p.Area-area) > 1e-7 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 5}) > 1e-8 {
		t.Fatal(p, e)
	}
}
func TestAxialShouldersRoundtrip(t *testing.T) {
	t.Parallel()
	for _, collar := range []bool{false, true} {
		var b bytes.Buffer
		if e := WriteAxialProfiles(&b, shoulderModel(t, collar)); e != nil {
			t.Fatal(e)
		}
		m, e := ReadAnalyticBodies(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		checkShoulders(t, m, collar)
	}
}
func TestOCCTAxialShoulders(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"groove", "collar", "cut-groove"} {
		collar := kind == "collar"
		path := filepath.Join(t.TempDir(), "reference.step")
		stepOracle(t, path, "write-step-"+kind)
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(kind, e)
		}
		checkShoulders(t, m, collar)
		var b bytes.Buffer
		if e = WriteAxialProfiles(&b, shoulderModel(t, collar)); e != nil {
			t.Fatal(e)
		}
		ours := filepath.Join(t.TempDir(), "ours.step")
		if e = os.WriteFile(ours, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		out := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, out, ours)
		raw, e = os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var p struct {
			Volume, Area float64
			Centroid     [3]float64
		}
		area := 26 * math.Pi
		if collar {
			area = 20 * math.Pi
		}
		if e = json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-10*math.Pi) > 1e-6 || math.Abs(p.Area-area) > 1e-6 || math.Abs(p.Centroid[2]-5) > 1e-8 {
			t.Fatal(kind, p, e)
		}
	}
}

func TestIndependentAxialShoulders(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"groove", "collar", "cut-groove"} {
		collar := kind == "collar"
		raw, e := os.ReadFile("testdata/axial-" + kind + ".step")
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadAnalyticBodies(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		checkShoulders(t, m, collar)
	}
}
func TestWideningShoulderOrientationProof(t *testing.T) {
	t.Parallel()
	var raw bytes.Buffer
	if e := WriteAxialProfiles(&raw, shoulderModel(t, true)); e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	body := int64(0)
	changed := false
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 {
			continue
		}
		r := ent.Records[0]
		if r.Type == "MANIFOLD_SOLID_BREP" {
			body = id
		}
		if r.Type == "ADVANCED_FACE" && len(r.Args[1].Items) == 2 && r.Args[3].Text == "F" {
			r.Args[3].Text = "T"
			ent.Records[0] = r
			f.Entities[id] = ent
			changed = true
		}
	}
	if !changed || body == 0 {
		t.Fatal("missing widening shoulder")
	}
	if _, e := analyticBody(f, body); e == nil {
		t.Fatal("accepted inward widening shoulder")
	}
}
func TestAxialShouldersMixedFourSections(t *testing.T) {
	t.Parallel()
	m := axialModel(t)
	m.Bodies[0].Sections = []CylinderSection{{1, 1}, {3, 1}, {2, 1}, {4, 1}}
	var b bytes.Buffer
	if e := WriteAxialProfiles(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadAnalyticBodies(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := got.Bodies[0].Solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
	if e != nil || math.Abs(p.Volume-30*math.Pi) > 1e-7 || math.Abs(p.Area-62*math.Pi) > 1e-7 {
		t.Fatal(p, e)
	}
}
func FuzzAxialShoulderSTEP(f *testing.F) {
	for _, kind := range []string{"collar", "groove", "cut-groove"} {
		raw, e := os.ReadFile("testdata/axial-" + kind + ".step")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadAnalyticBodies(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 512, MaxFaces: 256, MaxSolids: 4, MaxEdgeUses: 2048})
	})
}

// Mutate the independent OCCT cut graph, including its noncanonical upper wall.
// Parameter-origin normalization must not hide mismatched model/UV geometry.
func TestAxialShiftedOriginProof(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/axial-cut-groove.step")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		id        int64
		component int
		value     string
	}{
		{"seam-model-phase", 28, 2, "3.25"},
		{"seam-uv-phase", 39, 1, "0.25"},
		{"rim-uv-height", 61, 1, "4.25"},
		{"surface-axis-offset", 34, 0, "1.25"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := Parse(bytes.NewReader(raw), Limits{})
			if err != nil {
				t.Fatal(err)
			}
			ent := f.Entities[tc.id]
			r := ent.Records[0]
			if r.Type != "CARTESIAN_POINT" {
				t.Fatal("fixture graph changed")
			}
			r.Args[1].Items[tc.component] = Value{Kind: Number, Text: tc.value}
			ent.Records[0] = r
			f.Entities[tc.id] = ent
			body := int64(0)
			for _, id := range f.Order {
				e := f.Entities[id]
				if len(e.Records) == 1 && e.Records[0].Type == "MANIFOLD_SOLID_BREP" {
					body = id
				}
			}
			if body == 0 {
				t.Fatal("missing body")
			}
			if _, err := analyticBody(f, body); err == nil {
				t.Fatal("accepted inconsistent shifted wall")
			}
		})
	}
}
