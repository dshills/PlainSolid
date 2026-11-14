package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"testing"
)

func TestIndependentCylinderMaterials(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-cylinder-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentCylinderMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Members) != 6 || m.Members[4].Cylinder == nil || m.Members[5].Cylinder == nil || m.Members[4].Cylinder.Cells != 2 || m.Members[5].Cylinder.Cells != 4 {
		t.Fatal("families")
	}
	var sources []*File
	for i, p := range m.Members {
		sources = append(sources, p.Source)
		if i < 4 {
			continue
		}
		c := p.Cylinder
		if c.SourceBodyID != p.SourceBodyID || len(c.SourceFaces) != c.Cells+2 || len(c.SourceEdges) != 3*c.Cells || p.SourceProductID == 0 {
			t.Fatal("ancestry")
		}
		if c.NativeFaceSourceIDs[0] != c.SourceFaces[c.Cells] || c.NativeFaceSourceIDs[1] != c.SourceFaces[c.Cells+1] {
			t.Fatal("caps")
		}
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, p.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := trimmed.ReadJSON(&native, trimmed.JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		proof, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if e != nil || !proof.Complete {
			t.Fatal(e)
		}
		for _, use := range proof.Uses {
			if use.Report.SquaredUpperBound == nil || use.Report.SquaredUpperBound.Sign() != 0 {
				t.Fatal("nonexact native curve")
			}
		}
	}
	var out bytes.Buffer
	if e = WriteIndependentCylinderMaterials(ctx, &out, sources, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	again, e := ReadIndependentCylinderMaterials(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil || len(again.Members) != 6 {
		t.Fatal(e)
	}
	if _, e = ReadIndependentExactMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old widened")
	}
	var envelope bytes.Buffer
	if e = WriteIndependentExactMaterialsJSON(ctx, &envelope, m); e == nil || envelope.Len() != 0 {
		t.Fatal("legacy native envelope widened or partially written")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if v, err := ReadIndependentCylinderMaterials(cancelled, bytes.NewReader(raw), Limits{}, 1e-9); err == nil || len(v.Members) != 0 {
		t.Fatal("cancelled read")
	}
	old, e := os.ReadFile("testdata/occt-independent-exact-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentCylinderMaterials(ctx, bytes.NewReader(old), Limits{}, 1e-9); e == nil {
		t.Fatal("missing cylinder")
	}
	for _, kind := range []string{"radius", "sense", "duplicate-face", "circle", "unit"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		p := m.Members[4]
		changed := false
		for _, id := range p.Source.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 {
				if kind == "unit" {
					for j, r := range en.Records {
						if r.Type == "SI_UNIT" && len(r.Args) == 2 && r.Args[1].Text == "METRE" {
							en.Records[j].Args[0] = Value{Kind: Omitted}
							f.Entities[id] = en
							changed = true
							break
						}
					}
				}
				if changed {
					break
				}
				continue
			}
			r := en.Records[0]
			switch {
			case kind == "radius" && r.Type == "CYLINDRICAL_SURFACE":
				en.Records[0].Args[2].Text = "3"
				changed = true
			case kind == "sense" && r.Type == "ADVANCED_FACE":
				en.Records[0].Args[3].Text = "F"
				changed = true
			case kind == "duplicate-face" && r.Type == "CLOSED_SHELL":
				en.Records[0].Args[1].Items[1] = en.Records[0].Args[1].Items[0]
				changed = true
			case kind == "circle" && r.Type == "CIRCLE":
				en.Records[0].Type = "ELLIPSE"
				changed = true
			}
			if changed {
				f.Entities[id] = en
				break
			}
		}
		if !changed {
			t.Fatal("mutation")
		}
		bad, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		v, e := ReadIndependentCylinderMaterials(ctx, bytes.NewReader(bad), Limits{}, 1e-9)
		if e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	for _, l := range []Limits{{MaxSolids: 5}, {MaxFaces: 1}, {MaxEntities: 600}, {MaxEdgeUses: 1}} {
		v, e := ReadIndependentCylinderMaterials(ctx, bytes.NewReader(raw), l, 1e-9)
		if e == nil || len(v.Members) != 0 {
			t.Fatal("limits")
		}
	}
	var reject bytes.Buffer
	if e = WriteIndependentCylinderMaterials(ctx, &reject, append(sources, nil), Limits{}, 1e-9); e == nil || reject.Len() != 0 {
		t.Fatal("partial write")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_CYLINDER_MATERIALS"); oracle != "" {
		for _, b := range [][]byte{raw, out.Bytes()} {
			path := filepath.Join(t.TempDir(), "mixed.step")
			os.WriteFile(path, b, 0600)
			data, e := exec.Command(oracle, "read", path).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			var q struct {
				Solids, Faces, EdgeUses int
				Volume, Area            float64
			}
			at := bytes.LastIndex(data, []byte("{"))
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Solids != 6 || q.Faces != 26 || q.EdgeUses != 84 || math.Abs(q.Volume-(24+135.25*math.Pi)) > 1e-6 || math.Abs(q.Area-(522.168698195557+40*math.Pi)) > 1e-6 {
				t.Fatal(string(data))
			}
		}
	}
}
func FuzzIndependentCylinderMaterials(f *testing.F) {
	raw, e := os.ReadFile("testdata/independent-cylinder-materials.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		v, e := ReadIndependentCylinderMaterials(context.Background(), bytes.NewReader(b), Limits{MaxSolids: 6, MaxEntities: 1024, MaxValues: 16384}, 1e-9)
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
