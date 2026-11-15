package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func nestedRationalAnisotropicNativeFixture(t testing.TB) []byte {
	t.Helper()
	m, e := ReadNestedRationalWrappedAnisotropicStationAssembly(context.Background(), bytes.NewReader(nestedRationalAnisotropicFixture(t)), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationAssemblyJSON(context.Background(), &b, m); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestNestedRationalAnisotropicNative(t *testing.T) {
	ctx := context.Background()
	raw := nestedRationalAnisotropicNativeFixture(t)
	m, e := ReadNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, &stable, m); e != nil || !bytes.Equal(raw, stable.Bytes()) {
		t.Fatal("unstable", e)
	}
	if e = WriteNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	for _, edit := range []func(*nestedRationalAnisotropicNativeEnvelope){
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Format = anisotropicAssemblyNativeFormat },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Version++ },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.GroupID++ },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Instances[0].Path[1]++ },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Instances[0].Matrix[0][0] = "1/3" },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Source = "ISO-10303-21;" },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.SourceFaceIDs[0]++ },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Product.DefinitionID++ },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Occurrences[0].Source.ToPlacementID++ },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Occurrences[0].Matrix[0][0] = "1/3" },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Body.Walls[0].Surface.Weights[0] = .25 },
		func(q *nestedRationalAnisotropicNativeEnvelope) { q.Native = json.RawMessage(`{}`) },
	} {
		var q nestedRationalAnisotropicNativeEnvelope
		if e = json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, _ := json.Marshal(q)
		bad, e := ReadNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || bad.Source != nil {
			t.Fatal("forged", e)
		}
	}
	if _, e = ReadWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old format widened")
	}
	if _, e = ReadRationalWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("flat rational tag widened")
	}
	if _, e = ReadNestedWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("signed nested tag widened")
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 10}, {Source: Limits{MaxEntities: 10}}} {
		if _, e = ReadNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	cancelctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNestedRationalWrappedAnisotropicStationAssemblyJSON(cancelctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	m.Occurrences[0].NAUOID++
	var out bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale")
	}
}
func TestNestedRationalAnisotropicLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	src := filepath.Join(dir, "source.step")
	report := filepath.Join(dir, "report.json")
	if b, e := exec.CommandContext(ctx, bin, src, report, "filled-nested-oblique").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	type mass struct {
		Volume, Area   float64
		Centroid       [3]float64
		Solids, Shells int
		Shared         bool
		LeafPlacements [][3][4]float64
	}
	var original mass
	b, e := os.ReadFile(report)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &original); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(src)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadNestedRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadNestedRationalWrappedAnisotropicStationAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationAssemblySource(ctx, &out, m.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	dst := filepath.Join(dir, "output.step")
	if e = os.WriteFile(dst, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	if b, e = exec.CommandContext(ctx, bin, dst, report, "read-nested-assembly").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	b, e = os.ReadFile(report)
	if e != nil {
		t.Fatal(e)
	}
	var q mass
	if e = json.Unmarshal(b, &q); e != nil {
		t.Fatal(e)
	}
	if !q.Shared || q.Solids != 2 || q.Shells != 2 || math.Abs(q.Volume-original.Volume) > 1e-8 || math.Abs(q.Area-original.Area) > 1e-8 {
		t.Fatal(q, original)
	}
	if len(q.LeafPlacements) != len(m.Instances) {
		t.Fatal("leaf poses")
	}
	for i, u := range m.Instances {
		matrix := u.Matrix()
		for r := 0; r < 3; r++ {
			for c := 0; c < 4; c++ {
				v, _ := matrix[r][c].Float64()
				if math.Abs(q.LeafPlacements[i][r][c]-v) > 1e-14 {
					t.Fatal("exact composed pose", q.LeafPlacements[i], matrix)
				}
			}
		}
	}
	for i := range q.Centroid {
		if math.Abs(q.Centroid[i]-original.Centroid[i]) > 1e-8 {
			t.Fatal(q, original)
		}
	}
}
func FuzzNestedRationalAnisotropicNative(f *testing.F) {
	f.Add(nestedRationalAnisotropicNativeFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadNestedRationalWrappedAnisotropicStationAssemblyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxEntities: 4096, MaxDepth: 16}})
		if e != nil && m.Source != nil {
			t.Fatal("partial")
		}
	})
}
