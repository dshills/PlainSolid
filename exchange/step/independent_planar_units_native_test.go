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

func TestIndependentPlanarUnitsNative(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	source, e := os.ReadFile("testdata/independent-planar-units.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadNormalizedIndependentPlanarMaterials(ctx, bytes.NewReader(source), PlanarMillimetre, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteNormalizedIndependentPlanarMaterialsJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	n, e := ReadNormalizedIndependentPlanarMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var again bytes.Buffer
	if e = WriteNormalizedIndependentPlanarMaterialsJSON(ctx, &again, n); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal("unstable", e)
	}
	if _, e = ReadIndependentSphereMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cross tag")
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
		if v, e := ReadNormalizedIndependentPlanarMaterialsJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}

	for _, kind := range []string{"model-speed", "pcurve-speed", "pcurve-offset", "projected-edge"} {
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
			if r.Type == "LINE" && (kind == "model-speed" || kind == "pcurve-speed" || kind == "pcurve-offset") {
				pointID := r.Args[1].Ref
				point := f.Entities[pointID]
				dimension := len(point.Records[0].Args[1].Items)
				if (kind == "model-speed" && dimension != 3) || (kind != "model-speed" && dimension != 2) {
					continue
				}
				if kind == "pcurve-offset" {
					point.Records[0].Args[1].Items[1].Text = "1."
					f.Entities[pointID] = point
				} else {
					vectorID := r.Args[2].Ref
					v := f.Entities[vectorID]
					v.Records[0].Args[2].Text = "2."
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
		if v, e := ReadNormalizedIndependentPlanarMaterialsJSON(ctx, bytes.NewReader(bad), IndependentMaterialJSONLimits{}); e == nil || len(v.Members) != 0 {
			t.Fatal(kind)
		}
	}
	if _, e = ReadNormalizedIndependentRoundMaterialsJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("round tag widened")
	}
	for _, limit := range []IndependentMaterialJSONLimits{{MaxBytes: 1}, {Source: Limits{MaxSolids: 1}}, {Source: Limits{MaxEntities: 20}}} {
		if v, e := ReadNormalizedIndependentPlanarMaterialsJSON(ctx, bytes.NewReader(raw), limit); e == nil || len(v.Members) != 0 {
			t.Fatal("limit")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNormalizedIndependentPlanarMaterialsJSON(cancelled, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_PLANAR_UNITS"); oracle != "" {
		var sources []*File
		for _, p := range n.Members {
			sources = append(sources, p.Source)
		}
		var step bytes.Buffer
		if e = WriteIndependentExactMaterials(ctx, &step, sources, Limits{}, 1e-9); e != nil {
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
		if at < 0 || json.Unmarshal(data[at:], &q) != nil || q.Solids != 2 || math.Abs(q.Volume-(24+24e9)) > 1e-3 {
			t.Fatal(string(data))
		}
	}
	n.OriginalUnits[0] = PlanarMetre
	var rejected bytes.Buffer
	if e = WriteNormalizedIndependentPlanarMaterialsJSON(ctx, &rejected, n); e == nil || rejected.Len() != 0 {
		t.Fatal("atomic stale")
	}
}
func FuzzIndependentPlanarUnitsNative(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-planar-units.step")
	if e != nil {
		f.Fatal(e)
	}
	m, e := ReadNormalizedIndependentPlanarMaterials(context.Background(), bytes.NewReader(b), PlanarMillimetre, Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteNormalizedIndependentPlanarMaterialsJSON(context.Background(), &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 2<<20 {
			return
		}
		v, e := ReadNormalizedIndependentPlanarMaterialsJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 2 << 20, Source: Limits{MaxEntities: 1024, MaxValues: 16384, MaxSolids: 4}})
		if e != nil && len(v.Members) != 0 {
			t.Fatal("partial")
		}
	})
}
