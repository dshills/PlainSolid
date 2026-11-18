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
	"reflect"
	"strings"
	"testing"
	"time"
)

func combinedColorsFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-combined-context-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestCombinedOccurrenceColors(t *testing.T) {
	t.Parallel()
	raw := combinedColorsFixture(t)
	ctx := context.Background()
	m, e := ReadCombinedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Paths()) != 6 || len(m.SharedOverrides()) != 1 || len(m.PathOverrides()) != 1 {
		t.Fatal(m)
	}
	want := []RGB{{0, 1, 1}, {0, 0, 1}, {0, 1, 0}, {0, 1, 1}, {1, 0, 0}, {0, 1, 0}}
	for i, p := range m.Paths() {
		if p.Color != want[i] || p.Overridden != (i == 0 || i == 1 || i == 3) {
			t.Fatal(i, p)
		}
	}
	if _, e = ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old SHUO widened")
	}
	if _, e = ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old shared widened")
	}
}

func TestCombinedOccurrenceColorsLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_COMBINED_CONTEXT")
	if bin == "" {
		t.Skip("optional pinned XCAF SHUO observer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw := combinedColorsFixture(t)
	m, e := ReadCombinedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteCombinedOccurrenceColoredAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadCombinedOccurrenceColoredAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteCombinedOccurrenceColoredAssemblySource(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	fresh := filepath.Join(t.TempDir(), "fresh.step")
	if b, e := exec.CommandContext(ctx, bin, "write", fresh).CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	freshRaw, e := os.ReadFile(fresh)
	if e != nil {
		t.Fatal(e)
	}
	shifted := bytes.Replace(raw, []byte("#5,#62,$,#58,#413"), []byte("#5,#62,$,#767,#413"), 1)
	for _, source := range [][]byte{raw, out.Bytes(), freshRaw, shifted} {
		proof, e := ReadCombinedOccurrenceColoredAssembly(ctx, bytes.NewReader(source), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		file := filepath.Join(t.TempDir(), "source.step")
		os.WriteFile(file, source, 0600)
		b, e := exec.CommandContext(ctx, bin, "read", file).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		start := bytes.LastIndex(b, []byte("{\"volume\""))
		if start < 0 {
			t.Fatal(string(b))
		}
		var got struct {
			Volume, Area float64
			Occurrences  []struct {
				Translation [3]float64
				Rotation    [9]float64
				Color       [3]float64
				Component   bool
				Depth       int
			}
		}
		if e = json.Unmarshal(b[start:], &got); e != nil {
			t.Fatal(e)
		}
		if math.Abs(got.Volume-108) > 1e-9 || math.Abs(got.Area-252) > 1e-9 || len(got.Occurrences) != 6 {
			t.Fatal(got)
		}
		for i, p := range proof.Paths() {
			q := got.Occurrences[i]
			if q.Depth != 2 || q.Component != p.Overridden || q.Translation != [3]float64{p.Transform[0][3], p.Transform[1][3], p.Transform[2][3]} {
				t.Fatal(i, p, q)
			}
			for r := 0; r < 3; r++ {
				for c := 0; c < 3; c++ {
					if q.Rotation[r*3+c] != p.Transform[r][c] {
						t.Fatal("rotation")
					}
				}
			}
			for j, v := range []float64{p.Color.R, p.Color.G, p.Color.B} {
				if math.Abs(v-q.Color[j]) > 1e-12 {
					t.Fatal("color", i, p, q)
				}
			}
		}
	}
}

func TestCombinedOccurrenceColorsReject(t *testing.T) {
	t.Parallel()
	raw := string(combinedColorsFixture(t))
	ctx := context.Background()
	for _, c := range [][2]string{
		{"#769 = SHAPE_REPRESENTATION('',(#37),#398)", "#769 = SHAPE_REPRESENTATION('',(#41),#398)"},
		{"#768 = SHAPE_DEFINITION_REPRESENTATION(#406,#769)", "#768 = SHAPE_DEFINITION_REPRESENTATION(#412,#769)"},
		{"(#786),#68,#777", "(#786),#422,#777"},
		{"#5,#62,$,#58,#413", "#5,#62,$,#413,#58"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=SHAPE_DEFINITION_REPRESENTATION(#406,#769);\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		bad := strings.Replace(raw, c[0], c[1], 1)
		if bad == raw {
			t.Fatal("stale", c)
		}
		m, e := ReadCombinedOccurrenceColoredAssembly(ctx, strings.NewReader(bad), Limits{})
		if e == nil || len(m.Paths()) != 0 {
			t.Fatal("accepted", c, e)
		}
	}
	// Equal colors still do not authorize overlapping shared and path targets.
	overlap := strings.Replace(raw, "#5,#62,$,#58,#413", "#5,#62,$,#58,#407", 1)
	overlap = strings.Replace(overlap, "#775 = SHAPE_REPRESENTATION('',(#41),#398)", "#775 = SHAPE_REPRESENTATION('',(#37),#398)", 1)
	overlap = strings.Replace(overlap, "'blue'", "'cyan'", 1)
	if _, e := ReadCombinedOccurrenceColoredAssembly(ctx, strings.NewReader(overlap), Limits{}); e == nil {
		t.Fatal("overlap")
	}
	for _, b := range [][]byte{nestedColorsFixture(t), shuoFixture(t)} {
		if _, e := ReadCombinedOccurrenceColoredAssembly(ctx, bytes.NewReader(b), Limits{}); e == nil {
			t.Fatal("missing separate class")
		}
	}
	if _, e := ReadCombinedOccurrenceColoredAssembly(ctx, strings.NewReader(raw), Limits{MaxEntities: 808}); e == nil {
		t.Fatal("source limit")
	}
}
func TestCombinedOccurrenceColorsNative(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m, e := ReadCombinedOccurrenceColoredAssembly(ctx, bytes.NewReader(combinedColorsFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if want := []UnusedOccurrenceContext{{768, 769, 406, 771}}; !reflect.DeepEqual(m.UnusedContexts(), want) {
		t.Fatal(m.UnusedContexts())
	}
	native, e := m.NativeAssembly(ctx)
	if e != nil {
		t.Fatal(e)
	}
	delete(native.Assembly.Source.Entities, 774)
	var out bytes.Buffer
	if e = WriteCombinedOccurrenceColoredAssemblyJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	n, e := ReadCombinedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil || !reflect.DeepEqual(n.Paths(), m.Paths()) {
		t.Fatal(e)
	}
	out.Reset()
	if e = WriteCombinedOccurrenceColoredAssemblyJSON(ctx, &out, n); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("unstable", e)
	}
	for _, mut := range []func(*combinedOccurrenceColorsEnvelope){
		func(q *combinedOccurrenceColorsEnvelope) { q.Format = shuoOccurrenceColorsNativeFormat },
		func(q *combinedOccurrenceColorsEnvelope) { q.Version++ },
		func(q *combinedOccurrenceColorsEnvelope) { q.Shared[0].OccurrenceID = 413 },
		func(q *combinedOccurrenceColorsEnvelope) { q.Unused[0].EquivalentRepresentationID = 775 },
		func(q *combinedOccurrenceColorsEnvelope) { q.SHUO[0].Path[0] = 767 },
		func(q *combinedOccurrenceColorsEnvelope) {
			q.Paths[4].Color = RGB{0, 0, 1}
			q.Paths[4].Overridden = true
		},
		func(q *combinedOccurrenceColorsEnvelope) { q.Products[2].Bodies[0].Vertices[0].X++ },
		func(q *combinedOccurrenceColorsEnvelope) { q.Source = strings.Replace(q.Source, "'cyan'", "'red'", 1) },
	} {
		var q combinedOccurrenceColorsEnvelope
		json.Unmarshal(raw, &q)
		mut(&q)
		b, _ := json.Marshal(q)
		z, e := ReadCombinedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || len(z.Paths()) != 0 {
			t.Fatal("native tamper")
		}
	}
	if _, e = ReadSHUOOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("prior tag")
	}
	if _, e = ReadCombinedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{MaxBytes: int64(len(raw) - 1)}); e == nil {
		t.Fatal("limit")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadCombinedOccurrenceColoredAssemblyJSON(canceled, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	if e = WriteCombinedOccurrenceColoredAssemblyJSON(ctx, occurrenceShortWriter{}, m); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	m.unused[0].EquivalentRepresentationID++
	out.Reset()
	if e = WriteCombinedOccurrenceColoredAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("atomic")
	}
}
func FuzzCombinedOccurrenceColors(f *testing.F) {
	f.Add(combinedColorsFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadCombinedOccurrenceColoredAssembly(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024})
		if e != nil {
			if len(m.Paths()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteCombinedOccurrenceColoredAssemblySource(context.Background(), &out, m); e != nil {
			t.Fatal(e)
		}
	})
}

func FuzzCombinedOccurrenceColorsNative(f *testing.F) {
	ctx := context.Background()
	m, e := ReadCombinedOccurrenceColoredAssembly(ctx, bytes.NewReader(combinedColorsFixture(f)), Limits{})
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteCombinedOccurrenceColoredAssemblyJSON(ctx, &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Add([]byte(`{"Format":"plainsolid-step-combined-contextual-appearance","Version":1}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 256<<10 {
			t.Skip()
		}
		m, e := ReadCombinedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 256 << 10, Source: Limits{MaxEntities: 1024}})
		if e != nil {
			if len(m.Paths()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteCombinedOccurrenceColoredAssemblyJSON(ctx, &out, m); e != nil {
			t.Fatal(e)
		}
	})
}
