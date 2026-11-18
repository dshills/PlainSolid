package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestIndependentToroidalMaterials(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-model-torus.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentToroidalMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Members) != 1 || m.Members[0].Torus == nil {
		t.Fatal("family")
	}
	p := m.Members[0]
	q := p.Torus
	if q.MajorRadius != 3 || q.MinorRadius != 1 || q.Frame.Origin().X != 1 || q.SourceEdgeIDs[0] == q.SourceEdgeIDs[1] || p.SourceProductID == 0 {
		t.Fatal("roles")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, p.Solid); e != nil {
		t.Fatal(e)
	}
	solid, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	mass, e := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7, MaxSamples: 100000})
	if e != nil || math.Abs(mass.Volume-6*math.Pi*math.Pi) > 1e-5 || math.Abs(mass.Area-12*math.Pi*math.Pi) > 1e-5 || mass.Centroid.Distance(q.Frame.Origin()) > 1e-6 {
		t.Fatal("native measures", mass, e)
	}
	var out bytes.Buffer
	if e = WriteIndependentToroidalMaterials(ctx, &out, []*File{p.Source}, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentToroidalMaterials(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentSphereMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old widened")
	}
	vendor, e := os.ReadFile("testdata/occt-independent-unsupported-torus.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentToroidalMaterials(ctx, bytes.NewReader(vendor), Limits{}, 1e-9); e == nil {
		t.Fatal("rounded source")
	}
	for _, kind := range []string{"horn", "spindle", "face", "use", "edge", "vertex", "circle", "pcurve", "minor-frame"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		id := q.SourceSurfaceID
		switch kind {
		case "face":
			id = q.SourceFaceID
		case "use":
			id = q.SourceUseIDs[0]
		case "edge":
			id = q.SourceEdgeIDs[0]
		case "vertex":
			id = q.SourcePointID
		case "circle", "pcurve", "minor-frame":
			id = q.SourceCircleIDs[1]
		}
		if kind == "minor-frame" {
			circle := f.Entities[id].Records[0]
			placement := f.Entities[circle.Args[1].Ref].Records[0]
			id = placement.Args[2].Ref
		}
		en := f.Entities[id]
		r := &en.Records[0]
		switch kind {
		case "horn":
			r.Args[2].Text = "1."
		case "spindle":
			r.Args[2].Text = "0.5"
		case "face":
			r.Args[3].Text = "F"
		case "use":
			r.Args[4].Text = "T"
		case "edge":
			r.Args[4].Text = "F"
		case "vertex":
			r.Args[1].Items[0].Text = "5.000000000000001"
		case "circle":
			r.Args[2].Text = "2."
		case "pcurve":
			r.Type = "SEAM_CURVE"
		case "minor-frame":
			r.Args[1].Items[0].Text = "-2.449293598295E-16"
		}
		f.Entities[id] = en
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		if v, e := ReadIndependentToroidalMaterials(ctx, bytes.NewReader(b), Limits{}, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	base, e := os.ReadFile("testdata/independent-sphere-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	prior, e := ReadIndependentSphereMaterials(ctx, bytes.NewReader(base), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	sources := []*File{}
	for _, member := range prior.Members {
		sources = append(sources, member.Source)
	}
	var rejected bytes.Buffer
	if e = WriteIndependentToroidalMaterials(ctx, &rejected, sources, Limits{}, 1e-9); e == nil || rejected.Len() != 0 {
		t.Fatal("missing torus atomic")
	}
	sources = append(sources, p.Source)
	var mixed bytes.Buffer
	if e = WriteIndependentToroidalMaterials(ctx, &mixed, sources, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if v, e := ReadIndependentToroidalMaterials(ctx, bytes.NewReader(mixed.Bytes()), Limits{}, 1e-9); e != nil || len(v.Members) != 8 {
		t.Fatal("mixed", e)
	}
	if _, e = ReadIndependentToroidalMaterials(ctx, bytes.NewReader(mixed.Bytes()), Limits{MaxSolids: 7}, 1e-9); e == nil {
		t.Fatal("budget")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadIndependentToroidalMaterials(cancelled, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TOROIDAL_MATERIALS"); oracle != "" {
		for i, b := range [][]byte{raw, out.Bytes(), mixed.Bytes()} {
			path := filepath.Join(t.TempDir(), "model.step")
			if e = os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", path).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var v struct {
				Solids, Faces, EdgeUses int
				Volume, Area            float64
			}
			if at < 0 || json.Unmarshal(data[at:], &v) != nil {
				t.Fatal(string(data))
			}
			wantV, wantA := 6*math.Pi*math.Pi, 12*math.Pi*math.Pi
			count := 1
			if i == 2 {
				count = 8
				wantV += 24 + (135.25+32.0/3)*math.Pi
				wantA += 522.168698195557 + 56*math.Pi
			}
			if v.Solids != count || math.Abs(v.Volume-wantV) > 1e-6 || math.Abs(v.Area-wantA) > 1e-6 {
				t.Fatal(string(data))
			}
			if i < 2 && (v.Faces != 1 || v.EdgeUses != 4) {
				t.Fatal("topology")
			}
		}
	}
}
func FuzzIndependentToroidalMaterials(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-model-torus.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		v, e := ReadIndependentToroidalMaterials(context.Background(), bytes.NewReader(b), Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 4}, 1e-9)
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
