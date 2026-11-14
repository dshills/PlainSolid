package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIndependentCylinderMaterialsNative(t *testing.T) {
	ctx := context.Background()
	source, e := os.ReadFile("testdata/independent-cylinder-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentCylinderMaterials(ctx, bytes.NewReader(source), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentCylinderMaterialsJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	n, e := ReadIndependentCylinderMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteIndependentCylinderMaterialsJSON(ctx, &again, n); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("unstable", e)
	}
	if _, e = ReadIndependentExactMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old accepted")
	}
	for _, kind := range []string{"tag", "version", "product", "shape", "ancestry", "source", "native", "unit"} {
		var q independentMaterialEnvelope
		if json.Unmarshal(raw, &q) != nil {
			t.Fatal("json")
		}
		switch kind {
		case "tag":
			q.Format = "plainsolid-step-independent-exact-materials"
		case "version":
			q.Version++
		case "source":
			q.Source = q.Source[:len(q.Source)/2]
		case "native":
			q.Members[4].Native = json.RawMessage(`{}`)
		case "unit":
			q.LengthUnit *= 1000
		default:
			var claim map[string]any
			if json.Unmarshal(q.Members[4].Claims, &claim) != nil {
				t.Fatal("claim")
			}
			if kind == "product" {
				claim["SourceProductID"] = float64(1)
			}
			if kind == "shape" {
				claim["Cylinder"].(map[string]any)["Radius"] = float64(9)
			}
			if kind == "ancestry" {
				claim["Cylinder"].(map[string]any)["SourceFaces"] = []int{1, 2, 3, 4}
			}
			q.Members[4].Claims, _ = json.Marshal(claim)
		}
		bad, _ := json.Marshal(q)
		v, e := ReadIndependentCylinderMaterialsJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{})
		if e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 1}, {Source: Limits{MaxSolids: 5}}, {Source: Limits{MaxEntities: 100}}} {
		if v, e := ReadIndependentCylinderMaterialsJSON(ctx, bytes.NewReader(raw), l); e == nil || len(v.Members) != 0 {
			t.Fatal("limits")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadIndependentCylinderMaterialsJSON(cancelled, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_CYLINDER_MATERIALS"); oracle != "" {
		var sources []*File
		for _, member := range n.Members {
			sources = append(sources, member.Source)
		}
		var step bytes.Buffer
		if e = WriteIndependentCylinderMaterials(ctx, &step, sources, Limits{}, 1e-9); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(t.TempDir(), "reloaded.step")
		if e = os.WriteFile(path, step.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		data, e := exec.Command(oracle, "read", path).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		at := bytes.LastIndex(data, []byte("{"))
		var q struct {
			Solids, Faces int
			Volume        float64
		}
		if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Solids != 6 || q.Faces != 26 || math.Abs(q.Volume-(24+135.25*math.Pi)) > 1e-6 {
			t.Fatal(string(data))
		}
	}
	n.Members[4].Cylinder.SourceFaces[0]++
	var rejected bytes.Buffer
	if e = WriteIndependentCylinderMaterialsJSON(ctx, &rejected, n); e == nil || rejected.Len() != 0 {
		t.Fatal("stale writer")
	}
}

func FuzzIndependentCylinderMaterialsNative(f *testing.F) {
	raw, e := os.ReadFile("testdata/independent-cylinder-materials.step")
	if e != nil {
		f.Fatal(e)
	}
	m, e := ReadIndependentCylinderMaterials(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteIndependentCylinderMaterialsJSON(context.Background(), &b, m); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 2<<20 {
			return
		}
		v, e := ReadIndependentCylinderMaterialsJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 2 << 20, Source: Limits{MaxEntities: 1024, MaxSolids: 6}})
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
