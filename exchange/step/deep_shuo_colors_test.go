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

func deepSHUOFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-deep-shuo-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestOCCTDeepSHUO(t *testing.T) {
	ctx := context.Background()
	raw := deepSHUOFixture(t)
	m, e := ReadOCCTDeepSHUOAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Paths()) != 12 || m.Override().PrefixSHUOID != 801 || m.Override().TerminalSHUOID != 800 || m.Override().Path != [3]int64{54, 85, 440} {
		t.Fatal(m)
	}
	for i, p := range m.Paths() {
		if p.Overridden != (i == 1) {
			t.Fatal(i, p)
		}
	}
	if m.Paths()[1].Color != (RGB{0, 0, 1}) || m.Paths()[4].Color != (RGB{1, 0, 0}) || m.Paths()[7].Color != (RGB{1, 0, 0}) {
		t.Fatal(m.Paths())
	}
	if _, e = ReadSHUOOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old SHUO accepted")
	}
	if _, e = ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old nested accepted")
	}
	p := m.Paths()
	p[1].Color = RGB{}
	if reflect.DeepEqual(p, m.Paths()) {
		t.Fatal("alias")
	}
	var out bytes.Buffer
	if e = WriteOCCTDeepSHUOAssemblySource(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadOCCTDeepSHUOAssembly(ctx, &out, Limits{})
	if e != nil || !reflect.DeepEqual(m.Paths(), n.Paths()) {
		t.Fatal(e)
	}
}
func TestOCCTDeepSHUOReject(t *testing.T) {
	raw := string(deepSHUOFixture(t))
	for _, c := range [][2]string{
		{"#5,#89,$,#801,#440", "#5,#58,$,#801,#440"},
		{"#5,#58,$,#54,#85", "#31,#58,$,#54,#85"},
		{"#5,#89,$,#801,#440", "#5,#89,$,#800,#440"},
		{"#5,#89,$,#801,#440", "#5,#89,$,#54,#440"},
		{"#5,#58,$,#54,#85", "#5,#58,$,#85,#54"},
		{"PRODUCT_DEFINITION_SHAPE('SHUO',$,#801)", "PRODUCT_DEFINITION_SHAPE('SHUO',$,#800)"},
		{"#804 = SHAPE_REPRESENTATION('',(#68),#425)", "#804 = SHAPE_REPRESENTATION('',(#64),#425)"},
		{"(#824),#95,#815", "(#824),#449,#815"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=SPECIFIED_HIGHER_USAGE_OCCURRENCE('','',$,#5,#89,$,#801,#434);\nENDSEC;\nEND-ISO-10303-21;"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=PRODUCT_DEFINITION_SHAPE('SHUO',$,#800);\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		bad := strings.Replace(raw, c[0], c[1], 1)
		if bad == raw {
			t.Fatal("stale", c)
		}
		m, e := ReadOCCTDeepSHUOAssembly(context.Background(), strings.NewReader(bad), Limits{})
		if e == nil || len(m.Paths()) != 0 {
			t.Fatal("accepted", c, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: int64(len(raw) - 1)}, {MaxEntities: 829}} {
		if _, e := ReadOCCTDeepSHUOAssembly(context.Background(), strings.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	if _, e := ReadOCCTDeepSHUOAssembly(context.Background(), bytes.NewReader(shuoFixture(t)), Limits{}); e == nil {
		t.Fatal("shallow")
	}
}
func TestOCCTDeepSHUONative(t *testing.T) {
	ctx := context.Background()
	m, e := ReadOCCTDeepSHUOAssembly(ctx, bytes.NewReader(deepSHUOFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	native, e := m.NativeAssembly(ctx)
	if e != nil {
		t.Fatal(e)
	}
	delete(native.Assembly.Source.Entities, 800)
	var out bytes.Buffer
	if e = WriteOCCTDeepSHUOAssemblyJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	n, e := ReadOCCTDeepSHUOAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil || !reflect.DeepEqual(n.Paths(), m.Paths()) {
		t.Fatal(e)
	}
	out.Reset()
	if e = WriteOCCTDeepSHUOAssemblyJSON(ctx, &out, n); e != nil || !bytes.Equal(out.Bytes(), raw) {
		t.Fatal("unstable", e)
	}
	for _, mut := range []func(*deepSHUOEnvelope){func(q *deepSHUOEnvelope) { q.Format = shuoOccurrenceColorsNativeFormat }, func(q *deepSHUOEnvelope) { q.Version++ }, func(q *deepSHUOEnvelope) { q.SHUO.PrefixSHUOID = 800 }, func(q *deepSHUOEnvelope) { q.SHUO.Path[1] = 794 }, func(q *deepSHUOEnvelope) { q.Paths[4].Color = RGB{0, 0, 1}; q.Paths[4].Overridden = true }, func(q *deepSHUOEnvelope) { q.Products[3].Bodies[0].Vertices[0].X++ }, func(q *deepSHUOEnvelope) { q.Source = strings.Replace(q.Source, "#54,#85", "#54,#794", 1) }} {
		var q deepSHUOEnvelope
		json.Unmarshal(raw, &q)
		mut(&q)
		b, _ := json.Marshal(q)
		z, e := ReadOCCTDeepSHUOAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || len(z.Paths()) != 0 {
			t.Fatal("native tamper")
		}
	}
	if _, e = ReadSHUOOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old native")
	}
	if _, e = ReadOCCTDeepSHUOAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{MaxBytes: int64(len(raw) - 1)}); e == nil {
		t.Fatal("limit")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadOCCTDeepSHUOAssemblyJSON(canceled, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	if e = WriteOCCTDeepSHUOAssemblyJSON(ctx, occurrenceShortWriter{}, m); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	m.claim.Path[1] = 794
	out.Reset()
	if e = WriteOCCTDeepSHUOAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("atomic")
	}
}
func TestOCCTDeepSHUOLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_DEEP_SHUO")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw := deepSHUOFixture(t)
	m, e := ReadOCCTDeepSHUOAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var native, out bytes.Buffer
	if e = WriteOCCTDeepSHUOAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadOCCTDeepSHUOAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteOCCTDeepSHUOAssemblySource(ctx, &out, m); e != nil {
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
	mid := bytes.Replace(raw, []byte("#54,#85"), []byte("#54,#794"), 1)
	outer := bytes.Replace(raw, []byte("#54,#85"), []byte("#799,#85"), 1)
	for k, source := range [][]byte{raw, out.Bytes(), freshRaw, mid, outer} {
		p, e := ReadOCCTDeepSHUOAssembly(ctx, bytes.NewReader(source), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		index := 1
		if k == 3 {
			index = 4
		}
		if k == 4 {
			index = 7
		}
		for i, q := range p.Paths() {
			if q.Overridden != (i == index) {
				t.Fatal("wrong path", k, i, q)
			}
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
		if math.Abs(got.Volume-216) > 1e-9 || math.Abs(got.Area-504) > 1e-9 || len(got.Occurrences) != 12 {
			t.Fatal(got)
		}
		for i, q := range p.Paths() {
			g := got.Occurrences[i]
			if g.Depth != 3 || g.Component != q.Overridden || g.Translation != [3]float64{q.Transform[0][3], q.Transform[1][3], q.Transform[2][3]} {
				t.Fatal(i, g, q)
			}
			for r := 0; r < 3; r++ {
				for c := 0; c < 3; c++ {
					if g.Rotation[3*r+c] != q.Transform[r][c] {
						t.Fatal("rotation")
					}
				}
			}
			for j, v := range []float64{q.Color.R, q.Color.G, q.Color.B} {
				if math.Abs(v-g.Color[j]) > 1e-12 {
					t.Fatal("color", i, g, q)
				}
			}
		}
	}
}

func FuzzOCCTDeepSHUO(f *testing.F) {
	f.Add(deepSHUOFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		m, e := ReadOCCTDeepSHUOAssembly(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024})
		if e != nil {
			if len(m.Paths()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteOCCTDeepSHUOAssemblySource(context.Background(), &out, m); e != nil {
			t.Fatal(e)
		}
	})
}
func FuzzOCCTDeepSHUONative(f *testing.F) {
	ctx := context.Background()
	m, e := ReadOCCTDeepSHUOAssembly(ctx, bytes.NewReader(deepSHUOFixture(f)), Limits{})
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteOCCTDeepSHUOAssemblyJSON(ctx, &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Add([]byte(`{"Format":"plainsolid-step-occt-depth-three-shuo","Version":1}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 256<<10 {
			t.Skip()
		}
		m, e := ReadOCCTDeepSHUOAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 256 << 10, Source: Limits{MaxEntities: 1024}})
		if e != nil {
			if len(m.Paths()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteOCCTDeepSHUOAssemblyJSON(ctx, &out, m); e != nil {
			t.Fatal(e)
		}
	})
}
