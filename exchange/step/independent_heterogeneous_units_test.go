package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"testing"
)

func TestIndependentHeterogeneousUnits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-heterogeneous-units.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadNormalizedIndependentMaterials(ctx, bytes.NewReader(raw), IndependentMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Members) != 3 || m.Members[0].Cylinder == nil || m.Members[1].Sphere == nil || m.Members[2].Planar == nil || m.OriginalUnits[0] != IndependentMillimetre || m.OriginalUnits[1] != IndependentMetre || m.OriginalUnits[2] != IndependentMetre {
		t.Fatal("families")
	}
	var sources []*File
	for _, p := range m.Members {
		if p.LengthUnit != .001 || p.SourceRootID == 0 || p.SourceBodyID == 0 {
			t.Fatal("metadata")
		}
		if _, ok := m.OriginalSource.Entities[p.SourceBodyID]; !ok {
			t.Fatal("original body")
		}
		sources = append(sources, p.Source)
		var native bytes.Buffer
		if e = trimmed.WriteJSON(&native, p.Solid); e != nil {
			t.Fatal(e)
		}
		if _, e = trimmed.ReadJSON(&native, trimmed.JSONLimits{}); e != nil {
			t.Fatal(e)
		}
	}
	if m.Members[0].Cylinder.Radius != 2 || m.Members[1].Sphere.Radius != 2000 {
		t.Fatal("scaled")
	}
	var out bytes.Buffer
	if e = WriteIndependentSphereMaterials(ctx, &out, sources, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadNormalizedIndependentRoundMaterials(ctx, bytes.NewReader(raw), RoundMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("round widened")
	}
	if _, e = ReadNormalizedIndependentPlanarMaterials(ctx, bytes.NewReader(raw), PlanarMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("planar widened")
	}
	for _, kind := range []string{"pcurve", "round-speed", "borrow", "ellipse"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		changed := false
		if kind == "borrow" {
			part := m.Members[2]
			sdr := f.Entities[part.SourceRootID].Records[0]
			rep := f.Entities[sdr.Args[1].Ref]
			rep.Records[0].Args[1].Items[1].Ref = m.Members[0].SourceBodyID
			f.Entities[sdr.Args[1].Ref] = rep
			changed = true
		} else {
			part := m.Members[2]
			if kind == "round-speed" || kind == "ellipse" {
				part = m.Members[0]
			}
			for _, id := range part.Source.Order {
				en := f.Entities[id]
				if len(en.Records) != 1 {
					continue
				}
				r := en.Records[0]
				switch {
				case kind == "pcurve" && r.Type == "CARTESIAN_POINT" && len(r.Args[1].Items) == 2:
					en.Records[0].Args[1].Items[1].Text = "1."
					changed = true
				case kind == "round-speed" && r.Type == "VECTOR":
					en.Records[0].Args[2].Text = "0.001"
					changed = true
				case kind == "ellipse" && r.Type == "CYLINDRICAL_SURFACE":
					en.Records[0].Type = "SURFACE_OF_LINEAR_EXTRUSION"
					changed = true
				}
				f.Entities[id] = en
				if changed {
					break
				}
			}
		}
		if !changed {
			t.Fatal("mutation")
		}
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		if v, e := ReadNormalizedIndependentMaterials(ctx, bytes.NewReader(b), IndependentMillimetre, Limits{}, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	for _, l := range []Limits{{MaxBytes: 1}, {MaxSolids: 2}, {MaxEntities: 30}, {MaxFaces: 1}, {MaxVertices: 1}, {MaxEdgeUses: 1}} {
		if v, e := ReadNormalizedIndependentMaterials(ctx, bytes.NewReader(raw), IndependentMillimetre, l, 1e-9); e == nil || len(v.Members) != 0 {
			t.Fatal("limit")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNormalizedIndependentMaterials(cancelled, bytes.NewReader(raw), IndependentMillimetre, Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_HETEROGENEOUS_UNITS"); oracle != "" {
		for _, b := range [][]byte{raw, out.Bytes()} {
			p := filepath.Join(t.TempDir(), "model.step")
			if e = os.WriteFile(p, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var q struct {
				Solids       int
				Volume, Area float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Solids != 3 || math.Abs(q.Volume-(24e9+12*math.Pi+32e9*math.Pi/3)) > 1e-3 || math.Abs(q.Area-(52e6+(16e6+20)*math.Pi)) > 1e-3 {
				t.Fatal(string(data))
			}
		}
	}
}
func FuzzIndependentHeterogeneousUnits(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-heterogeneous-units.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		v, e := ReadNormalizedIndependentMaterials(context.Background(), bytes.NewReader(b), IndependentMillimetre, Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 4}, 1e-9)
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
