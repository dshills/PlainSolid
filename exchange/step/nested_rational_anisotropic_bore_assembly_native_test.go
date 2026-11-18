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
	"plainsolid/internal/slowtest"
	"testing"
	"time"
)

func nestedRationalAnisotropicBoreNativeFixture(t testing.TB) []byte {
	t.Helper()
	m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssembly(context.Background(), bytes.NewReader(nestedRationalAnisotropicBoreFixture(t)), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(context.Background(), &b, m); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestNestedRationalAnisotropicBoreNative(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := nestedRationalAnisotropicBoreNativeFixture(t)
	m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, &stable, m); e != nil || !bytes.Equal(raw, stable.Bytes()) {
		t.Fatal("unstable", e)
	}
	if e = WriteNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	for _, edit := range []func(*nestedRationalAnisotropicBoreNativeEnvelope){
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Format = anisotropicAssemblyNativeFormat },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Version++ },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.GroupID++ },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Instances[0].Path[1]++ },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Instances[0].Matrix[0][0] = "1/3" },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Source = "ISO-10303-21;" },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.SourceFaceIDs[0]++ },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Product.DefinitionID++ },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Occurrences[0].Source.ToPlacementID++ },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Occurrences[0].Matrix[0][0] = "1/3" },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Body.Walls[0].Surface.Weights[0] = .25 },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Inner.Walls[0].SideEdges[0]++ },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Inner.Walls[0].Surface.Weights[0] = .25 },
		func(q *nestedRationalAnisotropicBoreNativeEnvelope) { q.Native = json.RawMessage(`{}`) },
	} {
		var q nestedRationalAnisotropicBoreNativeEnvelope
		if e = json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, _ := json.Marshal(q)
		bad, e := ReadNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || bad.Source != nil {
			t.Fatal("forged", e)
		}
	}
	if _, e = ReadWrappedAnisotropicStationBoreAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old format widened")
	}
	if _, e = ReadRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("flat rational tag widened")
	}
	if _, e = ReadNestedWrappedAnisotropicStationBoreAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("signed nested tag widened")
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 10}, {Source: Limits{MaxEntities: 10}}} {
		if _, e = ReadNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	cancelctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(cancelctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	m.Occurrences[0].NAUOID++
	var out bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale")
	}
}
func TestNestedRationalAnisotropicBoreLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	src := filepath.Join(dir, "source.step")
	report := filepath.Join(dir, "report.json")
	if b, e := exec.CommandContext(ctx, bin, src, report, "bore-nested-oblique").CombinedOutput(); e != nil {
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
	m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteNestedRationalWrappedAnisotropicStationBoreAssemblySource(ctx, &out, m.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
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
func FuzzNestedRationalAnisotropicBoreNative(f *testing.F) {
	f.Add(nestedRationalAnisotropicBoreNativeFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadNestedRationalWrappedAnisotropicStationBoreAssemblyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxEntities: 4096, MaxDepth: 16}})
		if e != nil && m.Source != nil {
			t.Fatal("partial")
		}
	})
}
