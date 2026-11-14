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

func shuoFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-shuo-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestSHUOOccurrenceColors(t *testing.T) {
	ctx := context.Background()
	raw := shuoFixture(t)
	m, e := ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Paths()) != 6 || len(m.Overrides()) != 1 {
		t.Fatal(m.Paths(), m.Overrides())
	}
	p := m.Paths()
	if !p[1].Overridden || p[1].Color != (RGB{0, 0, 1}) || p[4].Overridden || p[4].Color != (RGB{1, 0, 0}) || p[1].Path[1] != p[4].Path[1] {
		t.Fatal(p)
	}
	c := m.Overrides()[0]
	if c.SHUOID != 770 || c.Path != [2]int64{58, 413} || c.Context.ProductShapeID != 769 || c.Context.CDSRID != 409 {
		t.Fatal(c)
	}
	if _, e = ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("shared route admitted SHUO")
	}
	if _, e = ReadOccurrenceColoredAssembly(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("flat route admitted SHUO")
	}
	p[1].Color = RGB{}
	if reflect.DeepEqual(p, m.Paths()) {
		t.Fatal("mutable")
	}
	var native bytes.Buffer
	if e = WriteSHUOOccurrenceColoredAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadSHUOOccurrenceColoredAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteSHUOOccurrenceColoredAssemblySource(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadSHUOOccurrenceColoredAssembly(ctx, &out, Limits{})
	if e != nil || !reflect.DeepEqual(m.Paths(), n.Paths()) {
		t.Fatal(e)
	}
}
func TestSHUOOccurrenceColorsReject(t *testing.T) {
	raw := string(shuoFixture(t))
	for _, c := range [][2]string{
		{"#5,#62,$,#58,#413", "#31,#62,$,#58,#413"},
		{"#5,#62,$,#58,#413", "#5,#416,$,#58,#413"},
		{"#5,#62,$,#58,#413", "#5,#62,$,#413,#58"},
		{"#5,#62,$,#58,#413", "#5,#62,$,#770,#413"},
		{"#5,#62,$,#58,#413", "#5,#62,$,#58,#761"},
		{"SHAPE_REPRESENTATION('',(#41),#398)", "SHAPE_REPRESENTATION('',(#37),#398)"},
		{"(#791),#68,#782", "(#791),#422,#782"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=PRODUCT_DEFINITION_SHAPE('SHUO',$,#770);\nENDSEC;\nEND-ISO-10303-21;"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=SHAPE_DEFINITION_REPRESENTATION(#769,#771);\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		bad := strings.Replace(raw, c[0], c[1], 1)
		if bad == raw {
			t.Fatal("stale", c)
		}
		m, e := ReadSHUOOccurrenceColoredAssembly(context.Background(), strings.NewReader(bad), Limits{})
		if e == nil || len(m.Paths()) != 0 {
			t.Fatal("accepted", c, e)
		}
	}
	// Retargeting the upper use is valid and changes only the other repeated path.
	shifted := strings.Replace(raw, "#5,#62,$,#58,#413", "#5,#62,$,#767,#413", 1)
	m, e := ReadSHUOOccurrenceColoredAssembly(context.Background(), strings.NewReader(shifted), Limits{})
	if e != nil || m.Paths()[1].Overridden || !m.Paths()[4].Overridden {
		t.Fatal(e)
	}
}
func TestSHUOOccurrenceColorsLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_SHUO")
	if bin == "" {
		t.Skip("optional pinned XCAF SHUO observer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw := shuoFixture(t)
	m, e := ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteSHUOOccurrenceColoredAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadSHUOOccurrenceColoredAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteSHUOOccurrenceColoredAssemblySource(ctx, &out, m); e != nil {
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
		proof, e := ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(source), Limits{})
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

func TestSHUOOccurrenceColorsNative(t *testing.T) {
	ctx := context.Background()
	m, e := ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(shuoFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	native, e := m.NativeAssembly(ctx)
	if e != nil {
		t.Fatal(e)
	}
	delete(native.Assembly.Source.Entities, 770)
	var out bytes.Buffer
	if e = WriteSHUOOccurrenceColoredAssemblyJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	n, e := ReadSHUOOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil || !reflect.DeepEqual(m.Paths(), n.Paths()) || !reflect.DeepEqual(m.Overrides(), n.Overrides()) {
		t.Fatal(e)
	}
	out.Reset()
	if e = WriteSHUOOccurrenceColoredAssemblyJSON(ctx, &out, n); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("unstable", e)
	}
	for _, mut := range []func(*shuoOccurrenceColorsEnvelope){
		func(q *shuoOccurrenceColorsEnvelope) { q.Format = nestedOccurrenceColorsNativeFormat },
		func(q *shuoOccurrenceColorsEnvelope) { q.Version++ },
		func(q *shuoOccurrenceColorsEnvelope) { q.SHUO[0].Path[0] = 767 },
		func(q *shuoOccurrenceColorsEnvelope) { q.SHUO[0].Context.ProductShapeID = 412 },
		func(q *shuoOccurrenceColorsEnvelope) { q.Paths[4].Overridden = true; q.Paths[4].Color = RGB{0, 0, 1} },
		func(q *shuoOccurrenceColorsEnvelope) { q.Products[2].Bodies[0].Vertices[0].X++ },
		func(q *shuoOccurrenceColorsEnvelope) {
			q.Source = strings.Replace(q.Source, "#58,#413", "#767,#413", 1)
		},
	} {
		var q shuoOccurrenceColorsEnvelope
		if e = json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		mut(&q)
		b, _ := json.Marshal(q)
		z, e := ReadSHUOOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || len(z.Paths()) != 0 {
			t.Fatal("tamper")
		}
	}
	if _, e = ReadNestedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old nested tag")
	}
	_, old := occurrenceColorsNativeFixture(t)
	if _, e = ReadSHUOOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(old), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("flat tag")
	}
	if _, e = ReadSHUOOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{MaxBytes: int64(len(raw) - 1)}); e == nil {
		t.Fatal("limit")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadSHUOOccurrenceColoredAssemblyJSON(canceled, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	afterRead, cancelRead := context.WithCancel(ctx)
	if _, e = ReadSHUOOccurrenceColoredAssemblyJSON(afterRead, &occurrenceCancelReader{bytes.NewReader(raw), cancelRead}, IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel after read")
	}
	if e = WriteSHUOOccurrenceColoredAssemblyJSON(ctx, occurrenceShortWriter{}, m); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	m.claims[0].Path[0] = 767
	out.Reset()
	if e = WriteSHUOOccurrenceColoredAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("atomic")
	}
}
func FuzzSHUOOccurrenceColors(f *testing.F) {
	f.Add(shuoFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadSHUOOccurrenceColoredAssembly(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024})
		if e != nil {
			if len(m.Paths()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteSHUOOccurrenceColoredAssemblySource(context.Background(), &out, m); e != nil {
			t.Fatal(e)
		}
	})
}

func TestSHUOOccurrenceColorsLimits(t *testing.T) {
	raw := shuoFixture(t)
	ctx := context.Background()
	for _, l := range []Limits{{MaxBytes: int64(len(raw) - 1)}, {MaxEntities: 796}} {
		m, e := ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), l)
		if e == nil || len(m.Paths()) != 0 {
			t.Fatal("limit")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadSHUOOccurrenceColoredAssembly(canceled, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancel")
	}
	// This source contains a shared internal-use override, not a selected path.
	if _, e := ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(nestedColorsFixture(t)), Limits{}); e == nil {
		t.Fatal("shared source")
	}
}
func FuzzSHUOOccurrenceColorsNative(f *testing.F) {
	ctx := context.Background()
	m, e := ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(shuoFixture(f)), Limits{})
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteSHUOOccurrenceColoredAssemblyJSON(ctx, &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Add([]byte(`{"Format":"plainsolid-step-shuo-contextual-appearance","Version":1}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 256<<10 {
			t.Skip()
		}
		m, e := ReadSHUOOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 256 << 10, Source: Limits{MaxEntities: 1024}})
		if e != nil {
			if len(m.Paths()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteSHUOOccurrenceColoredAssemblyJSON(ctx, &out, m); e != nil {
			t.Fatal(e)
		}
	})
}
