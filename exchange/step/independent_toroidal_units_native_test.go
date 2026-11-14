package step

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIndependentToroidalUnitsNative(t *testing.T) {
	ctx := context.Background()
	source, e := os.ReadFile("testdata/independent-toroidal-units.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadNormalizedIndependentToroidalMaterials(ctx, bytes.NewReader(source), IndependentMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteNormalizedIndependentToroidalMaterialsJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	n, e := ReadNormalizedIndependentToroidalMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteNormalizedIndependentToroidalMaterialsJSON(ctx, &again, n); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("unstable", e)
	}
	for _, read := range []func(context.Context, io.Reader, IndependentMaterialJSONLimits) (IndependentExactMaterialCollection, error){
		ReadIndependentExactMaterialsJSON, ReadIndependentCylinderMaterialsJSON,
		ReadIndependentSphereMaterialsJSON, ReadIndependentToroidalMaterialsJSON,
	} {
		if _, err := read(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); err == nil {
			t.Fatal("unnormalized tag accepted normalized torus envelope")
		}
	}
	for _, kind := range []string{"tag", "version", "target", "units", "original", "normalized", "root", "native"} {
		var q normalizedRoundEnvelope
		if json.Unmarshal(raw, &q) != nil {
			t.Fatal("json")
		}
		switch kind {
		case "tag":
			q.Format = independentSphereNativeFormat
		case "version":
			q.Version++
		case "target":
			q.TargetUnit = RoundMetre
		case "units":
			q.OriginalUnits[1] = RoundMillimetre
		case "original":
			q.OriginalSource = q.OriginalSource[:len(q.OriginalSource)/2]
		case "normalized":
			q.Normalized.Source = q.OriginalSource
		case "native":
			q.Normalized.Members[1].Native = json.RawMessage(`{}`)
		case "root":
			var claim map[string]any
			if json.Unmarshal(q.Normalized.Members[1].Claims, &claim) != nil {
				t.Fatal("claim")
			}
			claim["SourceRootID"] = float64(1)
			q.Normalized.Members[1].Claims, _ = json.Marshal(claim)
		}
		b, _ := json.Marshal(q)
		if v, e := ReadNormalizedIndependentToroidalMaterialsJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}

	for _, kind := range []string{"model-speed", "pcurve-speed", "pcurve-offset", "projected-edge", "round-speed-leak"} {
		var q normalizedRoundEnvelope
		if json.Unmarshal(raw, &q) != nil {
			t.Fatal("json")
		}
		f, e := Parse(bytes.NewReader([]byte(q.OriginalSource)), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		changed := false
		for _, id := range f.Order {
			en := f.Entities[id]
			if len(en.Records) != 1 {
				continue
			}
			r := en.Records[0]
			if r.Type == "LINE" && (kind == "model-speed" || kind == "pcurve-speed" || kind == "pcurve-offset" || kind == "round-speed-leak") {
				pointID := r.Args[1].Ref
				point := f.Entities[pointID]
				dimension := len(point.Records[0].Args[1].Items)
				if ((kind == "model-speed" || kind == "round-speed-leak") && dimension != 3) || (kind != "model-speed" && kind != "round-speed-leak" && dimension != 2) {
					continue
				}
				if kind == "pcurve-offset" {
					point.Records[0].Args[1].Items[1].Text = "1."
					f.Entities[pointID] = point
				} else {
					vectorID := r.Args[2].Ref
					v := f.Entities[vectorID]
					v.Records[0].Args[2].Text = "2."
					if kind == "round-speed-leak" {
						v.Records[0].Args[2].Text = "0.001"
					}
					f.Entities[vectorID] = v
				}
				changed = true
			}
			if r.Type == "VERTEX_POINT" && kind == "projected-edge" {
				pointID := r.Args[1].Ref
				point := f.Entities[pointID]
				point.Records[0].Args[1].Items[0].Text = "1."
				f.Entities[pointID] = point
				changed = true
			}
			if changed {
				break
			}
		}
		if !changed {
			t.Fatal("missing role")
		}
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		q.OriginalSource = string(b)
		bad, _ := json.Marshal(q)
		if v, e := ReadNormalizedIndependentToroidalMaterialsJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{}); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	if _, e = ReadNormalizedIndependentRoundMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("round tag widened")
	}
	if _, e = ReadNormalizedIndependentPlanarMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("planar tag widened")
	}

	if _, e = ReadNormalizedIndependentMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("mixed tag widened")
	}
	for _, kind := range []string{"torus-major", "torus-minor", "torus-vertex", "torus-seam"} {
		var q normalizedRoundEnvelope
		if json.Unmarshal(raw, &q) != nil {
			t.Fatal("json")
		}
		var claim map[string]any
		if json.Unmarshal(q.Normalized.Members[3].Claims, &claim) != nil {
			t.Fatal("claim")
		}
		torus := claim["Torus"].(map[string]any)
		switch kind {
		case "torus-major":
			torus["MajorRadius"] = float64(1)
		case "torus-minor":
			torus["MinorRadius"] = float64(1)
		case "torus-vertex":
			torus["SourceVertexID"] = float64(1)
		case "torus-seam":
			torus["SourceEdgeIDs"] = []int{1, 1}
		}
		q.Normalized.Members[3].Claims, _ = json.Marshal(claim)
		bad, _ := json.Marshal(q)
		if v, e := ReadNormalizedIndependentToroidalMaterialsJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{}); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	for _, limit := range []IndependentMaterialJSONLimits{{MaxBytes: 1}, {Source: Limits{MaxSolids: 1}}, {Source: Limits{MaxEntities: 20}}} {
		if v, e := ReadNormalizedIndependentToroidalMaterialsJSON(ctx, bytes.NewReader(raw), limit); e == nil || len(v.Members) != 0 {
			t.Fatal("limit")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNormalizedIndependentToroidalMaterialsJSON(cancelled, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TOROIDAL_UNITS"); oracle != "" {
		var sources []*File
		for _, p := range n.Members {
			sources = append(sources, p.Source)
		}
		var step bytes.Buffer
		if e = WriteIndependentToroidalMaterials(ctx, &step, sources, Limits{}, 1e-9); e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(t.TempDir(), "native.step")
		if e = os.WriteFile(p, step.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		data, e := exec.Command(oracle, "read", p).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		at := bytes.LastIndex(data, []byte("{"))
		var q struct {
			Solids int
			Volume float64
		}
		if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Solids != 4 || math.Abs(q.Volume-(24e9+12*math.Pi+32e9*math.Pi/3+6e9*math.Pi*math.Pi)) > 1e-3 {
			t.Fatal(string(data))
		}
	}
	n.OriginalUnits[0] = IndependentMetre
	var rejected bytes.Buffer
	if e = WriteNormalizedIndependentToroidalMaterialsJSON(ctx, &rejected, n); e == nil || rejected.Len() != 0 {
		t.Fatal("atomic stale")
	}
}
func FuzzIndependentToroidalUnitsNative(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-toroidal-units.step")
	if e != nil {
		f.Fatal(e)
	}
	m, e := ReadNormalizedIndependentToroidalMaterials(context.Background(), bytes.NewReader(b), IndependentMillimetre, Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteNormalizedIndependentToroidalMaterialsJSON(context.Background(), &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 2<<20 {
			return
		}
		v, e := ReadNormalizedIndependentToroidalMaterialsJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 2 << 20, Source: Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 4}})
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
