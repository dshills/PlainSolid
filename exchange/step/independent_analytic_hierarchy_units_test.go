package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizedAnalyticHierarchy(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/independent-analytic-hierarchy-units.step")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	m, e := ReadNormalizedIndependentAnalyticHierarchy(ctx, bytes.NewReader(raw), IndependentMaterialSIUnit(RoundMillimetre), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	base, _ := os.ReadFile("testdata/independent-analytic-hierarchy.step")
	b, e := ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(base), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Hierarchy.Instances, b.Instances) {
		t.Fatal("world paths changed")
	}
	if len(m.OriginalUnits) != 6 {
		t.Fatal("units")
	}
	if _, e = ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old reader widened")
	}
	var n bytes.Buffer
	if e = WriteNormalizedIndependentAnalyticHierarchyJSON(ctx, &n, m); e != nil {
		t.Fatal(e)
	}
	q, e := ReadNormalizedIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(n.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var n2 bytes.Buffer
	if e = WriteNormalizedIndependentAnalyticHierarchyJSON(ctx, &n2, q); e != nil || !bytes.Equal(n.Bytes(), n2.Bytes()) {
		t.Fatal("stable", e)
	}
	if _, e = ReadIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(n.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old native tag")
	}
	normalized, e := mappedPlanarSourceBytes(q.Hierarchy.Assembly.Source)
	if e != nil {
		t.Fatal(e)
	}
	var retained bytes.Buffer
	if e = WriteNormalizedIndependentAnalyticHierarchySource(ctx, &retained, q.OriginalSource, q.TargetUnit, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	originalReload := retained.Bytes()
	for _, limit := range []IndependentMaterialJSONLimits{{MaxBytes: 32}, {Source: Limits{MaxEntities: 16}}} {
		if _, e = ReadNormalizedIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(n.Bytes()), limit); e == nil {
			t.Fatal("native limits")
		}
	}
	var normalizedOut bytes.Buffer
	if e = WriteIndependentAnalyticHierarchySource(ctx, &normalizedOut, q.Hierarchy.Assembly.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(normalized, normalizedOut.Bytes()) {
		t.Fatal("normalized source writer")
	}

	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
		for _, b := range [][]byte{raw, normalized, originalReload} {
			file := filepath.Join(t.TempDir(), "hierarchy.step")
			if e = os.WriteFile(file, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", file).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var q struct {
				Volume, Area            float64
				Solids, Faces, EdgeUses int
				Bounds                  [6]float64
				SurfaceCenter           [3]float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil {
				t.Fatal(string(data))
			}
			area := 52 + 24*math.Pi*math.Pi + 40*math.Pi
			volume := 24 + 12*math.Pi*math.Pi + 24*math.Pi
			if q.Solids != 5 || q.Faces != 20 || q.EdgeUses != 80 || math.Abs(q.Volume-volume) > 1e-7 || math.Abs(q.Area-area) > 1e-7 {
				t.Fatal(q)
			}
			for i, want := range [6]float64{-10, -4, -2, 30, 30, 9} {
				if math.Abs(q.Bounds[i]-want) > 2e-7 {
					t.Fatal(q.Bounds)
				}
			}
			for i, want := range [3]float64{(-9*52 + 29*12*math.Pi*math.Pi + 34.5*20*math.Pi) / area, (-2.5*52 + 23*12*math.Pi*math.Pi + 28*20*math.Pi) / area, (10*12*math.Pi*math.Pi + 6.5*20*math.Pi) / area} {
				if math.Abs(q.SurfaceCenter[i]-want) > 1e-7 {
					t.Fatal("centroid", q.SurfaceCenter)
				}
			}
		}
	}
	var env normalizedAnalyticHierarchyEnvelope
	if e = json.Unmarshal(n.Bytes(), &env); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"tag", "units", "pose", "source", "native"} {
		t.Run(name, func(t *testing.T) {
			var x normalizedAnalyticHierarchyEnvelope
			json.Unmarshal(n.Bytes(), &x)
			switch name {
			case "tag":
				x.Format = independentAnalyticHierarchyNativeFormat
			case "units":
				for k := range x.Units {
					x.Units[k] = "metre"
				}
			case "pose":
				x.Proof.Instances[0].Transform[0][3]++
			case "source":
				x.Source = "invalid"
			case "native":
				x.Proof.Proof.Members[0].Native = json.RawMessage(`{}`)
			}
			bb, _ := json.Marshal(x)
			if z, e := ReadNormalizedIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(bb), IndependentMaterialJSONLimits{}); e == nil || z.OriginalSource != nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestNormalizedAnalyticHierarchyReject(t *testing.T) {
	t.Parallel()
	raw, _ := os.ReadFile("testdata/independent-analytic-hierarchy-units.step")
	ctx := context.Background()
	target := IndependentMaterialSIUnit(RoundMillimetre)
	for _, tc := range []struct {
		name string
		b    []byte
		l    Limits
		u    IndependentMaterialSIUnit
	}{
		{"nonrepresentable", raw, Limits{}, IndependentMaterialSIUnit(RoundMetre)},
		{"unit", bytes.ReplaceAll(raw, []byte(".MILLI."), []byte(".KILO.")), Limits{}, target},
		{"limit", raw, Limits{MaxBytes: 32}, target},
		{"entitylimit", raw, Limits{MaxEntities: 16}, target},
		{"target", raw, Limits{}, "inch"},
		{"exponent", bytes.Replace(raw, []byte("0.02"), []byte("1.E-999999999"), 1), Limits{}, target},
		{"rounding", bytes.Replace(raw, []byte("0.02"), []byte("0.020000000000000000001"), 1), Limits{}, target},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, e := ReadNormalizedIndependentAnalyticHierarchy(ctx, bytes.NewReader(tc.b), tc.u, tc.l, 1e-9)
			if e == nil || m.OriginalSource != nil {
				t.Fatal("accepted")
			}
		})
	}
	nonzero, _ := os.ReadFile("testdata/independent-analytic-hierarchy-units-nonzero-origin.step")
	if _, e := ReadNormalizedIndependentAnalyticHierarchy(ctx, bytes.NewReader(nonzero), target, Limits{}, 1e-9); e == nil {
		t.Fatal("nonzero origin")
	}
	// Retain an actual source support mismatch, not just malformed syntax.
	f, _ := Parse(bytes.NewReader(raw), Limits{})
	for id, en := range f.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "TOROIDAL_SURFACE" {
			en.Records[0].Args[2].Text = "0.004"
			f.Entities[id] = en
			break
		}
	}
	forged, _ := mappedPlanarSourceBytes(f)
	if _, e := ReadNormalizedIndependentAnalyticHierarchy(ctx, bytes.NewReader(forged), target, Limits{}, 1e-9); e == nil {
		t.Fatal("support mismatch")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadNormalizedIndependentAnalyticHierarchy(cancelCtx, bytes.NewReader(raw), target, Limits{}, 1e-9); e == nil {
		t.Fatal("cancellation")
	}
	m, e := ReadNormalizedIndependentAnalyticHierarchy(ctx, bytes.NewReader(raw), target, Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	m.Hierarchy.Instances[0].Transform[0][3]++
	var out bytes.Buffer
	if e = WriteNormalizedIndependentAnalyticHierarchyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale non-atomic write")
	}
}
func FuzzNormalizedAnalyticHierarchy(f *testing.F) {
	b, _ := os.ReadFile("testdata/independent-analytic-hierarchy-units.step")
	f.Add(b)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		m, e := ReadNormalizedIndependentAnalyticHierarchy(context.Background(), bytes.NewReader(b), IndependentMaterialSIUnit(RoundMillimetre), Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}, 1e-9)
		if e != nil && m.OriginalSource != nil {
			t.Fatal("partial result")
		}
	})
}
func FuzzNormalizedAnalyticHierarchyNative(f *testing.F) {
	b, _ := os.ReadFile("testdata/independent-analytic-hierarchy-units.step")
	m, e := ReadNormalizedIndependentAnalyticHierarchy(context.Background(), bytes.NewReader(b), IndependentMaterialSIUnit(RoundMillimetre), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var n bytes.Buffer
	if e = WriteNormalizedIndependentAnalyticHierarchyJSON(context.Background(), &n, m); e != nil {
		f.Fatal(e)
	}
	f.Add(n.Bytes())
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		m, e := ReadNormalizedIndependentAnalyticHierarchyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}})
		if e != nil && m.OriginalSource != nil {
			t.Fatal("partial result")
		}
	})
}
