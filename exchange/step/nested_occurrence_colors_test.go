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
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
	"time"
)

func nestedColorsFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-nested-occurrence-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestNestedOccurrenceColors(t *testing.T) {
	t.Parallel()
	raw := nestedColorsFixture(t)
	m, e := ReadNestedOccurrenceColoredAssembly(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadOccurrenceColoredAssembly(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("flat accepted nested")
	}
	native, e := m.NativeAssembly(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	native.Assembly.Occurrences[0].Transform[0][3]++
	delete(native.Assembly.Source.Entities, 790)
	if m.Overrides()[0].Color != (RGB{0, 0, 1}) {
		t.Fatal("native getter aliases")
	}
	paths := m.Paths()
	if len(paths) != 6 || len(m.Overrides()) != 2 {
		t.Fatal(paths, m.Overrides())
	}
	want := [][3]float64{{110, 0, 0}, {100, 20, 0}, {90, -5, 7}, {0, 110, 0}, {0, 100, 20}, {7, 90, -5}}
	colors := []RGB{{1, 0, 0}, {0, 0, 1}, {0, 1, 0}}
	for i, p := range paths {
		if [3]float64{p.Transform[0][3], p.Transform[1][3], p.Transform[2][3]} != want[i] || p.Color != colors[i%3] || p.Overridden != (i%3 == 1) {
			t.Fatal(i, p)
		}
		if i >= 3 && p.Path[1] != paths[i-3].Path[1] {
			t.Fatal("lost shared use")
		}
	}
	paths[0].Color = RGB{}
	if reflect.DeepEqual(paths, m.Paths()) {
		t.Fatal("mutable paths")
	}
	var out bytes.Buffer
	if e = WriteNestedOccurrenceColoredAssemblySource(context.Background(), &out, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadNestedOccurrenceColoredAssembly(context.Background(), &out, Limits{})
	if e != nil || !reflect.DeepEqual(n.Paths(), m.Paths()) {
		t.Fatal(e)
	}
	a := geom.Translation(geom.Vec3{X: 1 << 53})
	b := geom.Translation(geom.Vec3{X: 1})
	if _, e = nestedColorExactCompose(a, b); e == nil {
		t.Fatal("rounded composition")
	}
}

func TestNestedOccurrenceColorsReject(t *testing.T) {
	t.Parallel()
	raw := nestedColorsFixture(t)
	changes := [][2]string{
		{"#16 = CARTESIAN_POINT('',(100.,0.,0.))", "#16 = CARTESIAN_POINT('',(100.000000000000000001,0.,0.))"},
		{"#12 = CARTESIAN_POINT('',(0.,0.,0.))", "#12 = CARTESIAN_POINT('',(1.,0.,0.))"},
		{"#18 = DIRECTION('',(1.,0.,-0.))", "#18 = DIRECTION('',(1.,1.E-30,-0.))"},
		{"SHAPE_DEFINITION_REPRESENTATION(#412,#769)", "SHAPE_DEFINITION_REPRESENTATION(#406,#769)"},
		{"SHAPE_REPRESENTATION('',(#41),#398)", "SHAPE_REPRESENTATION('',(#37),#398)"},
		{"#790 = OVER_RIDING_STYLED_ITEM('overriding color',(#791),#68,#782)", "#790 = OVER_RIDING_STYLED_ITEM('overriding color',(#791),#422,#782)"},
		{"#805 = DRAUGHTING_PRE_DEFINED_COLOUR('blue')", "#805 = DRAUGHTING_PRE_DEFINED_COLOUR('green')"},
		{"#5,#31,$", "#31,#31,$"},
		{"#31,#62,$", "#31,#5,$"},
		{"#768 = SHAPE_DEFINITION_REPRESENTATION(#412,#769)", "#768 = SHAPE_DEFINITION_REPRESENTATION(#57,#769)"},
	}
	for _, c := range changes {
		bad := strings.Replace(string(raw), c[0], c[1], 1)
		if bad == string(raw) {
			t.Fatal("stale", c)
		}
		m, e := ReadNestedOccurrenceColoredAssembly(context.Background(), strings.NewReader(bad), Limits{})
		if e == nil || len(m.Paths()) != 0 {
			t.Fatal("accepted", c, e)
		}
	}
	for _, l := range []Limits{{MaxBytes: int64(len(raw) - 1)}, {MaxEntities: 804}} {
		if _, e := ReadNestedOccurrenceColoredAssembly(context.Background(), bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	flat, e := os.ReadFile("testdata/occt-ap242-occurrence-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadNestedOccurrenceColoredAssembly(context.Background(), bytes.NewReader(flat), Limits{}); e == nil {
		t.Fatal("flat admitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancel")
	}
}

func TestNestedOccurrenceColorsNative(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	m, e := ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(nestedColorsFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteNestedOccurrenceColoredAssemblyJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), out.Bytes()...)
	n, e := ReadNestedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil || !reflect.DeepEqual(m.Paths(), n.Paths()) {
		t.Fatal(e)
	}
	out.Reset()
	if e = WriteNestedOccurrenceColoredAssemblyJSON(ctx, &out, n); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("unstable", e)
	}
	if _, e = ReadOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old tag admitted")
	}
	_, old := occurrenceColorsNativeFixture(t)
	if _, e = ReadNestedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(old), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old flat tag")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadNestedOccurrenceColoredAssemblyJSON(canceled, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	afterRead, cancelRead := context.WithCancel(ctx)
	if _, e = ReadNestedOccurrenceColoredAssemblyJSON(afterRead, &occurrenceCancelReader{bytes.NewReader(raw), cancelRead}, IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel after read")
	}
	var q nestedOccurrenceColorsEnvelope
	if e = json.Unmarshal(raw, &q); e != nil {
		t.Fatal(e)
	}
	mutations := []func(*nestedOccurrenceColorsEnvelope){
		func(q *nestedOccurrenceColorsEnvelope) { q.Format = occurrenceColorsNativeFormat },
		func(q *nestedOccurrenceColorsEnvelope) { q.Version++ },
		func(q *nestedOccurrenceColorsEnvelope) { q.Paths[0].Color = RGB{0, 0, 1} },
		func(q *nestedOccurrenceColorsEnvelope) { q.Paths[0].Path[1]++ },
		func(q *nestedOccurrenceColorsEnvelope) { q.Paths[0].Transform[0][3]++ },
		func(q *nestedOccurrenceColorsEnvelope) { q.Overrides[0].OccurrenceID++ },
		func(q *nestedOccurrenceColorsEnvelope) { q.Products[2].Bodies[0].Vertices[0].X++ },
		func(q *nestedOccurrenceColorsEnvelope) { q.Source = strings.Replace(q.Source, "'blue'", "'red'", 1) },
	}
	for _, mut := range mutations {
		var bad nestedOccurrenceColorsEnvelope
		json.Unmarshal(raw, &bad)
		mut(&bad)
		b, _ := json.Marshal(bad)
		z, e := ReadNestedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || len(z.Paths()) != 0 {
			t.Fatal("forged envelope")
		}
	}
	for _, b := range [][]byte{append(append([]byte(nil), raw...), []byte("{}")...), bytes.Replace(raw, []byte("\"Version\":1"), []byte("\"Unknown\":1,\"Version\":1"), 1)} {
		if _, e = ReadNestedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("syntax")
		}
	}
	if _, e = ReadNestedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{MaxBytes: int64(len(raw) - 1)}); e == nil {
		t.Fatal("limit")
	}
	if e = WriteNestedOccurrenceColoredAssemblyJSON(ctx, occurrenceShortWriter{}, m); e != io.ErrShortWrite {
		t.Fatal(e)
	}
	m.paths[0].Color = RGB{}
	out.Reset()
	if e = WriteNestedOccurrenceColoredAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("non atomic")
	}
}

func TestNestedOccurrenceColorsLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_NESTED_OCCURRENCE")
	if bin == "" {
		t.Skip("optional pinned XCAF observer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	source := nestedColorsFixture(t)
	m, e := ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(source), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var native, step bytes.Buffer
	if e = WriteNestedOccurrenceColoredAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadNestedOccurrenceColoredAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteNestedOccurrenceColoredAssemblySource(ctx, &step, n); e != nil {
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
	if _, e = ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(freshRaw), Limits{}); e != nil {
		t.Fatal(e)
	}
	for i, raw := range [][]byte{source, step.Bytes(), freshRaw} {
		p := filepath.Join(t.TempDir(), "source.step")
		if e = os.WriteFile(p, raw, 0600); e != nil {
			t.Fatal(e)
		}
		b, e := exec.CommandContext(ctx, bin, "read", p).CombinedOutput()
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
				Color       [3]float64
				Rotation    [9]float64
				Component   bool
				Depth       int
			}
		}
		if e = json.Unmarshal(b[start:], &got); e != nil {
			t.Fatal(e, string(b))
		}
		if math.Abs(got.Volume-108) > 1e-9 || math.Abs(got.Area-252) > 1e-9 || len(got.Occurrences) != 6 {
			t.Fatal(i, got)
		}
		for j, o := range got.Occurrences {
			p := m.Paths()[j]
			if o.Depth != 2 || o.Component != p.Overridden || o.Translation != [3]float64{p.Transform[0][3], p.Transform[1][3], p.Transform[2][3]} {
				t.Fatal(i, j, o, p)
			}
			for r := 0; r < 3; r++ {
				for c := 0; c < 3; c++ {
					if o.Rotation[3*r+c] != p.Transform[r][c] {
						t.Fatal("rotation", o, p)
					}
				}
			}
			for k, v := range []float64{p.Color.R, p.Color.G, p.Color.B} {
				if math.Abs(v-o.Color[k]) > 1e-12 {
					t.Fatal(o, p)
				}
			}
		}
	}
}

func FuzzNestedOccurrenceColors(f *testing.F) {
	seed := nestedColorsFixture(f)
	f.Add(seed)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		ctx := context.Background()
		m, e := ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 1024})
		if e != nil {
			if len(m.Paths()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteNestedOccurrenceColoredAssemblySource(ctx, &out, m); e != nil {
			t.Fatal(e)
		}
	})
}

func FuzzNestedOccurrenceColorsNative(f *testing.F) {
	ctx := context.Background()
	m, e := ReadNestedOccurrenceColoredAssembly(ctx, bytes.NewReader(nestedColorsFixture(f)), Limits{})
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteNestedOccurrenceColoredAssemblyJSON(ctx, &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Add([]byte(`{"Format":"plainsolid-step-nested-contextual-appearance","Version":1}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 256<<10 {
			t.Skip()
		}
		m, e := ReadNestedOccurrenceColoredAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 256 << 10, Source: Limits{MaxEntities: 1024}})
		if e != nil {
			if len(m.Paths()) != 0 {
				t.Fatal("partial")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteNestedOccurrenceColoredAssemblyJSON(ctx, &out, m); e != nil {
			t.Fatal(e)
		}
	})
}
