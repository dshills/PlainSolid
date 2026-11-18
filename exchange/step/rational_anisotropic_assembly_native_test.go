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

func rationalAnisotropicNativeFixture(t testing.TB) []byte {
	t.Helper()
	m, e := ReadRationalWrappedAnisotropicStationAssembly(context.Background(), bytes.NewReader(rationalAnisotropicFixture(t)), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteRationalWrappedAnisotropicStationAssemblyJSON(context.Background(), &b, m); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestRationalAnisotropicNative(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := rationalAnisotropicNativeFixture(t)
	m, e := ReadRationalWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteRationalWrappedAnisotropicStationAssemblyJSON(ctx, &stable, m); e != nil || !bytes.Equal(raw, stable.Bytes()) {
		t.Fatal("unstable", e)
	}
	if e = WriteRationalWrappedAnisotropicStationAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	for _, edit := range []func(*rationalAnisotropicNativeEnvelope){
		func(q *rationalAnisotropicNativeEnvelope) { q.Format = anisotropicAssemblyNativeFormat },
		func(q *rationalAnisotropicNativeEnvelope) { q.Version++ },
		func(q *rationalAnisotropicNativeEnvelope) { q.Source = "ISO-10303-21;" },
		func(q *rationalAnisotropicNativeEnvelope) { q.SourceFaceIDs[0]++ },
		func(q *rationalAnisotropicNativeEnvelope) { q.Product.DefinitionID++ },
		func(q *rationalAnisotropicNativeEnvelope) { q.Occurrences[0].Source.ToPlacementID++ },
		func(q *rationalAnisotropicNativeEnvelope) { q.Occurrences[0].Matrix[0][0] = "1/3" },
		func(q *rationalAnisotropicNativeEnvelope) { q.Body.Walls[0].Surface.Weights[0] = .25 },
		func(q *rationalAnisotropicNativeEnvelope) { q.Native = json.RawMessage(`{}`) },
	} {
		var q rationalAnisotropicNativeEnvelope
		if e = json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, _ := json.Marshal(q)
		bad, e := ReadRationalWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || bad.Source != nil {
			t.Fatal("forged", e)
		}
	}
	if _, e = ReadWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old format widened")
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 10}, {Source: Limits{MaxEntities: 10}}} {
		if _, e = ReadRationalWrappedAnisotropicStationAssemblyJSON(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	cancelctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadRationalWrappedAnisotropicStationAssemblyJSON(cancelctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	m.Occurrences[0].NAUOID++
	var out bytes.Buffer
	if e = WriteRationalWrappedAnisotropicStationAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale")
	}
}
func TestRationalAnisotropicLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	src := filepath.Join(dir, "source.step")
	report := filepath.Join(dir, "report.json")
	if b, e := exec.CommandContext(ctx, bin, src, report, "filled-assembly-oblique").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	type mass struct {
		Volume, Area   float64
		Centroid       [3]float64
		Solids, Shells int
		Shared         bool
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
	m, e := ReadRationalWrappedAnisotropicStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteRationalWrappedAnisotropicStationAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadRationalWrappedAnisotropicStationAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteRationalWrappedAnisotropicStationAssemblySource(ctx, &out, m.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	dst := filepath.Join(dir, "output.step")
	if e = os.WriteFile(dst, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	if b, e = exec.CommandContext(ctx, bin, dst, report, "read-assembly").CombinedOutput(); e != nil {
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
	for i := range q.Centroid {
		if math.Abs(q.Centroid[i]-original.Centroid[i]) > 1e-8 {
			t.Fatal(q, original)
		}
	}
}
func FuzzRationalAnisotropicNative(f *testing.F) {
	f.Add(rationalAnisotropicNativeFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadRationalWrappedAnisotropicStationAssemblyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxEntities: 4096, MaxDepth: 16}})
		if e != nil && m.Source != nil {
			t.Fatal("partial")
		}
	})
}
