package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestIndependentToroidalMaterialsNative(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source, e := os.ReadFile("testdata/independent-model-torus.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentToroidalMaterials(ctx, bytes.NewReader(source), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentToroidalMaterialsJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	n, e := ReadIndependentToroidalMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteIndependentToroidalMaterialsJSON(ctx, &again, n); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("unstable", e)
	}
	if _, e = ReadIndependentExactMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old accepted")
	}
	if _, e = ReadIndependentCylinderMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cylinder tag widened")
	}

	base, e := os.ReadFile("testdata/independent-sphere-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	prior, e := ReadIndependentSphereMaterials(ctx, bytes.NewReader(base), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var parts []*File
	for _, p := range prior.Members {
		parts = append(parts, p.Source)
	}
	parts = append(parts, m.Members[0].Source)
	var mixedSTEP bytes.Buffer
	if e = WriteIndependentToroidalMaterials(ctx, &mixedSTEP, parts, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	mixed, e := ReadIndependentToroidalMaterials(ctx, bytes.NewReader(mixedSTEP.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var mixedJSON bytes.Buffer
	if e = WriteIndependentToroidalMaterialsJSON(ctx, &mixedJSON, mixed); e != nil {
		t.Fatal(e)
	}
	restored, e := ReadIndependentToroidalMaterialsJSON(ctx, &mixedJSON, IndependentMaterialJSONLimits{})
	if e != nil || len(restored.Members) != 8 {
		t.Fatal("mixed", e)
	}
	for i, p := range restored.Members {
		if p.SourceRootID != mixed.Members[i].SourceRootID || p.SourceBodyID != mixed.Members[i].SourceBodyID {
			t.Fatal("mixed ancestry")
		}
	}
	for _, kind := range []string{"tag", "version", "product", "shape", "ancestry", "source", "native", "unit", "vertex", "seams"} {
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
			q.Members[0].Native = json.RawMessage(`{}`)
		case "unit":
			q.LengthUnit *= 1000
		default:
			var claim map[string]any
			if json.Unmarshal(q.Members[0].Claims, &claim) != nil {
				t.Fatal("claim")
			}
			if kind == "product" {
				claim["SourceProductID"] = float64(1)
			}
			if kind == "shape" {
				claim["Torus"].(map[string]any)["MajorRadius"] = float64(9)
			}
			if kind == "vertex" {
				claim["Torus"].(map[string]any)["SourceVertexID"] = float64(1)
			}
			if kind == "seams" {
				claim["Torus"].(map[string]any)["SourceEdgeIDs"] = []int{1, 1}
			}
			if kind == "ancestry" {
				claim["Torus"].(map[string]any)["NativeFaceSourceIDs"] = []int{1, 2, 3, 4}
			}
			q.Members[0].Claims, _ = json.Marshal(claim)
		}
		bad, _ := json.Marshal(q)
		v, e := ReadIndependentToroidalMaterialsJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{})
		if e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}

	vendor, e := os.ReadFile("testdata/occt-independent-unsupported-torus.step")
	if e != nil {
		t.Fatal(e)
	}
	var forged independentMaterialEnvelope
	if json.Unmarshal(raw, &forged) != nil {
		t.Fatal("json")
	}
	forged.Source = string(vendor)
	bad, _ := json.Marshal(forged)
	if v, e := ReadIndependentToroidalMaterialsJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{}); e == nil || len(v.Members) != 0 {
		t.Fatal("rounded vendor")
	}
	if _, e = ReadIndependentSphereMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("sphere tag widened")
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 1}, {Source: Limits{MaxEntities: 1}}, {Source: Limits{MaxEntities: 20}}} {
		if v, e := ReadIndependentToroidalMaterialsJSON(ctx, bytes.NewReader(raw), l); e == nil || len(v.Members) != 0 {
			t.Fatal("limits")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadIndependentToroidalMaterialsJSON(cancelled, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TOROIDAL_MATERIALS"); oracle != "" {
		var sources []*File
		for _, member := range n.Members {
			sources = append(sources, member.Source)
		}
		var step bytes.Buffer
		if e = WriteIndependentToroidalMaterials(ctx, &step, sources, Limits{}, 1e-9); e != nil {
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
		if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Solids != 1 || q.Faces != 1 || math.Abs(q.Volume-(6*math.Pi*math.Pi)) > 1e-6 {
			t.Fatal(string(data))
		}
	}
	n.Members[0].Torus.NativeFaceSourceIDs[0]++
	var rejected bytes.Buffer
	if e = WriteIndependentToroidalMaterialsJSON(ctx, &rejected, n); e == nil || rejected.Len() != 0 {
		t.Fatal("stale writer")
	}
}

func FuzzIndependentToroidalMaterialsNative(f *testing.F) {
	raw, e := os.ReadFile("testdata/independent-model-torus.step")
	if e != nil {
		f.Fatal(e)
	}
	m, e := ReadIndependentToroidalMaterials(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteIndependentToroidalMaterialsJSON(context.Background(), &b, m); e != nil {
		f.Fatal(e)
	}
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 2<<20 {
			return
		}
		v, e := ReadIndependentToroidalMaterialsJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 2 << 20, Source: Limits{MaxEntities: 1024, MaxSolids: 7}})
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
