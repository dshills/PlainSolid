package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestEditOccurrenceSurfaceColor(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ap242-occurrence-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	before, e := ReadOccurrenceColoredAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	f := before.Assembly.Assembly.Source
	edited, e := EditOccurrenceSurfaceColor(context.Background(), f, 386, RGB{1, 1, 0}, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range edited.Overrides {
		if c.Color != (RGB{1, 1, 0}) {
			t.Fatal(c)
		}
	}
	// Exclusive authored chains change only their final color records.
	for id, en := range f.Entities {
		if id == 765 || id == 773 {
			continue
		}
		if !reflect.DeepEqual(en, edited.Assembly.Assembly.Source.Entities[id]) {
			t.Fatal("unrelated record changed", id)
		}
	}
	if len(f.Entities) != len(edited.Assembly.Assembly.Source.Entities) {
		t.Fatal("unexpected new records")
	}
	if q, _ := readSurfaceStyleColour(f, 765); q != (RGB{0, 0, 1}) {
		t.Fatal("input mutated")
	}
	noOp, e := EditOccurrenceSurfaceColor(context.Background(), f, 386, RGB{0, 0, 1}, Limits{})
	if e != nil || !reflect.DeepEqual(noOp.Assembly.Assembly.Source, f) {
		t.Fatal("no-op source changed", e)
	}
	var native bytes.Buffer
	if e = WriteOccurrenceColoredAssemblyJSON(context.Background(), &native, edited); e != nil {
		t.Fatal(e)
	}
	got, e := ReadOccurrenceColoredAssemblyJSON(context.Background(), bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil || !reflect.DeepEqual(got.Overrides, edited.Overrides) {
		t.Fatal(e)
	}
}

func sharedOccurrenceColorSource(t *testing.T, whole bool) *File {
	t.Helper()
	raw, e := os.ReadFile("testdata/occt-ap242-occurrence-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	remove := map[int64]bool{}
	if whole {
		for _, id := range []int64{759, 767} {
			en := f.Entities[id]
			en.Records[0].Args[0] = Value{Kind: Aggregate, Items: []Value{{Kind: Reference, Ref: 752}}}
			f.Entities[id] = en
		}
		for _, id := range []int64{760, 761, 762, 763, 764, 765, 768, 769, 770, 771, 772, 773} {
			remove[id] = true
		}
	} else {
		for _, id := range []int64{764, 772} {
			en := f.Entities[id]
			en.Records[0].Args[1] = Value{Kind: Reference, Ref: 757}
			f.Entities[id] = en
		}
		remove[765] = true
		remove[773] = true
	}
	var order []int64
	for _, id := range f.Order {
		if remove[id] {
			delete(f.Entities, id)
		} else {
			order = append(order, id)
		}
	}
	f.Order = order
	raw, e = mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadOccurrenceColoredAssembly(bytes.NewReader(raw), Limits{}); e != nil {
		t.Fatal(e)
	}
	return f
}
func TestEditOccurrenceSharedColors(t *testing.T) {
	for _, whole := range []bool{false, true} {
		f := sharedOccurrenceColorSource(t, whole)
		edited, e := EditOccurrenceSurfaceColor(context.Background(), f, 386, RGB{1, 1, 0}, Limits{})
		if e != nil {
			t.Fatal(whole, e)
		}
		changed := map[int64]bool{764: true, 772: true}
		extra := 1
		if whole {
			changed = map[int64]bool{759: true, 767: true}
			extra = 6
		}
		for id, en := range f.Entities {
			if changed[id] {
				continue
			}
			if !reflect.DeepEqual(en, edited.Assembly.Assembly.Source.Entities[id]) {
				t.Fatal("shared original record changed", whole, id)
			}
		}
		if len(edited.Assembly.Assembly.Source.Entities) != len(f.Entities)+extra {
			t.Fatal("unexpected copy set")
		}
		if c, _ := readSurfaceStyleColour(edited.Assembly.Assembly.Source, 757); c != (RGB{1, 0, 0}) {
			t.Fatal("changed product default")
		}
		if q, e := EditOccurrenceSurfaceColor(context.Background(), f, 386, RGB{1, 1, 0}, Limits{MaxEntities: len(f.Entities)}); e == nil || len(q.Overrides) != 0 {
			t.Fatal("expanded entity budget ignored")
		}
	}
}
func TestEditOccurrenceColorReject(t *testing.T) {
	m, _ := occurrenceColorsNativeFixture(t)
	f := m.Assembly.Assembly.Source
	for _, id := range []int64{0, -1, 35, 380, 734, 99999} {
		if q, e := EditOccurrenceSurfaceColor(context.Background(), f, id, RGB{1, 1, 0}, Limits{}); e == nil || len(q.Overrides) != 0 {
			t.Fatal("invalid/unoverridden target", id)
		}
	}
	for _, c := range []RGB{{-1, 0, 0}, {2, 0, 0}, {math.NaN(), 0, 0}, {0, math.Inf(1), 0}} {
		if _, e := EditOccurrenceSurfaceColor(context.Background(), f, 386, c, Limits{}); e == nil {
			t.Fatal("invalid color")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := EditOccurrenceSurfaceColor(ctx, f, 386, RGB{1, 1, 0}, Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := EditOccurrenceSurfaceColor(context.Background(), f, 386, RGB{1, 1, 0}, Limits{MaxEntities: 700}); e == nil {
		t.Fatal("source limit")
	}
}
func TestEditOccurrenceColorsLive(t *testing.T) {
	original, _ := occurrenceColorsNativeFixture(t)
	sources := []*File{original.Assembly.Assembly.Source, sharedOccurrenceColorSource(t, false), sharedOccurrenceColorSource(t, true)}
	for _, f := range sources {
		edited, e := EditOccurrenceSurfaceColor(context.Background(), f, 386, RGB{0.25, 0.5, 0.75}, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = WriteOccurrenceColoredAssemblyJSON(context.Background(), &native, edited); e != nil {
			t.Fatal(e)
		}
		restored, e := ReadOccurrenceColoredAssemblyJSON(context.Background(), bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteOccurrenceColoredAssemblySource(&out, restored.Assembly.Assembly.Source, Limits{}); e != nil {
			t.Fatal(e)
		}
		bin := os.Getenv("PLAINSOLID_OCCT_AP242_OCCURRENCE")
		if bin == "" {
			continue
		}
		path := filepath.Join(t.TempDir(), "edited.step")
		if e = os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		data, err := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("%v: %s", err, data)
		}
		var q struct {
			Volume, Area float64
			Occurrences  []struct {
				Translation [3]float64
				Color       [3]float64
			}
		}
		at := bytes.IndexByte(data, '{')
		if at < 0 || json.Unmarshal(data[at:], &q) != nil {
			t.Fatal(string(data))
		}
		if math.Abs(q.Volume-54) > 1e-9 || math.Abs(q.Area-126) > 1e-9 || len(q.Occurrences) != 3 {
			t.Fatal(q)
		}
		want := map[[3]float64][3]float64{{10, 0, 0}: {1, 0, 0}, {0, 20, 0}: {0.25, 0.5, 0.75}, {-10, -5, 7}: {0, 1, 0}}
		for _, o := range q.Occurrences {
			expected, ok := want[o.Translation]
			if !ok {
				t.Fatal(q)
			}
			for i, v := range o.Color {
				if math.Abs(v-expected[i]) > 1e-7 {
					t.Fatal("effective sRGB changed", q)
				}
			}
		}
	}
}

func FuzzEditOccurrenceColor(f *testing.F) {
	m, _ := occurrenceColorsNativeFixture(f)
	source := m.Assembly.Assembly.Source
	f.Add(int64(386), uint8(64), uint8(128), uint8(192))
	f.Add(int64(380), uint8(0), uint8(0), uint8(255))
	f.Fuzz(func(t *testing.T, id int64, r, g, b uint8) {
		color := RGB{float64(r) / 255, float64(g) / 255, float64(b) / 255}
		q, e := EditOccurrenceSurfaceColor(context.Background(), source, id, color, Limits{})
		if e != nil {
			if len(q.Overrides) != 0 {
				t.Fatal("partial failure")
			}
			return
		}
		if id != 386 || len(q.Overrides) != 2 {
			t.Fatal("wrong selected use")
		}
		for _, c := range q.Overrides {
			if c.Color != color {
				t.Fatal("wrong color")
			}
		}
		if old, _ := readSurfaceStyleColour(source, 765); old != (RGB{0, 0, 1}) {
			t.Fatal("input mutated")
		}
	})
}
