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

func TestSetOccurrenceColorCreate(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"occt-ap242-palette-assembly.step", "occt-ap242-occurrence-colors.step"} {
		raw, e := os.ReadFile("testdata/" + file)
		if e != nil {
			t.Fatal(e)
		}
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := SetOccurrenceSurfaceColor(context.Background(), f, 380, RGB{1, 1, 0}, Limits{})
		if e != nil {
			t.Fatal(file, e)
		}
		var claim OccurrenceSurfaceColor
		for _, c := range m.Overrides {
			if c.OccurrenceID == 380 {
				claim = c
			}
		}
		if claim.OccurrenceID != 380 || claim.Color != (RGB{1, 1, 0}) {
			t.Fatal(m.Overrides)
		}
		if len(m.Assembly.Assembly.Source.Entities) != len(f.Entities)+10 {
			t.Fatal("wrong authored graph size")
		}
		changed := 0
		for id, en := range f.Entities {
			got := m.Assembly.Assembly.Source.Entities[id]
			if reflect.DeepEqual(en, got) {
				continue
			}
			changed++
			if len(en.Records) != 1 || en.Records[0].Type != "MECHANICAL_DESIGN_GEOMETRIC_PRESENTATION_REPRESENTATION" {
				t.Fatal("unrelated entity changed", id)
			}
			old, current := en.Records[0].Args, got.Records[0].Args
			if !reflect.DeepEqual(old[0], current[0]) || !reflect.DeepEqual(old[2], current[2]) || len(current[1].Items) != len(old[1].Items)+1 || !reflect.DeepEqual(old[1].Items, current[1].Items[:len(old[1].Items)]) || current[1].Items[len(old[1].Items)].Ref != claim.StyledItemID {
				t.Fatal("presentation edit was not append-only")
			}
		}
		if changed != 1 {
			t.Fatal("wrong original mutation count", changed)
		}
		again, e := SetOccurrenceSurfaceColor(context.Background(), m.Assembly.Assembly.Source, 380, RGB{1, 1, 0}, Limits{})
		if e != nil || !reflect.DeepEqual(again.Assembly.Assembly.Source, m.Assembly.Assembly.Source) {
			t.Fatal("no-op changed source", e)
		}
		var out bytes.Buffer
		if e = WriteOccurrenceColoredAssemblyJSON(context.Background(), &out, m); e != nil {
			t.Fatal(e)
		}
		restored, e := ReadOccurrenceColoredAssemblyJSON(context.Background(), bytes.NewReader(out.Bytes()), IndependentMaterialJSONLimits{})
		if e != nil || !reflect.DeepEqual(restored.Overrides, m.Overrides) {
			t.Fatal(e)
		}
	}
}

func TestSetOccurrenceColorReject(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-ap242-palette-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []int64{-1, 0, 5, 35, 41, 99999} {
		if q, e := SetOccurrenceSurfaceColor(context.Background(), f, id, RGB{1, 1, 0}, Limits{}); e == nil || len(q.Overrides) != 0 {
			t.Fatal("wrong target", id)
		}
	}
	if q, e := SetOccurrenceSurfaceColor(context.Background(), f, 380, RGB{1, 1, 0}, Limits{MaxEntities: len(f.Entities)}); e == nil || len(q.Overrides) != 0 {
		t.Fatal("creation exceeded entity budget")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := SetOccurrenceSurfaceColor(ctx, f, 380, RGB{1, 1, 0}, Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := SetOccurrenceSurfaceColor(context.Background(), f, 380, RGB{math.NaN(), 0, 0}, Limits{}); e == nil {
		t.Fatal("invalid color")
	}
	// The author never invents a product default to make a style chain possible.
	raw, e = os.ReadFile("testdata/occt-exact-nauo-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	base, e := ReadColoredAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = SetOccurrenceSurfaceColor(context.Background(), base.Assembly.Source, base.Assembly.Occurrences[0].ID, RGB{1, 1, 0}, Limits{}); e == nil {
		t.Fatal("invented body default")
	}
	// Per-face precedence is excluded from this whole-body authoring subset.
	model := coloredAssemblyModel(t)
	// Flatten the fixture so rejection specifically exercises face precedence.
	model.Products = []ColoredAssemblyPart{model.Products[0], model.Products[2]}
	model.Occurrences = model.Occurrences[1:]
	for i := range model.Occurrences {
		model.Occurrences[i].Parent = "root"
	}
	var colored bytes.Buffer
	if e = WriteColoredAssembly(&colored, model); e != nil {
		t.Fatal(e)
	}
	base, e = ReadColoredAssembly(bytes.NewReader(colored.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range base.Assembly.Occurrences {
		if len(base.ProductBodies[o.Child]) == 0 {
			continue
		}
		if _, e = SetOccurrenceSurfaceColor(context.Background(), base.Assembly.Source, o.ID, RGB{1, 1, 0}, Limits{}); e == nil {
			t.Fatal("accepted face-style ambiguity")
		}
	}
}
func TestSetOccurrenceColorLive(t *testing.T) {
	t.Parallel()
	for variant, file := range []string{"occt-ap242-palette-assembly.step", "occt-ap242-occurrence-colors.step"} {
		raw, e := os.ReadFile("testdata/" + file)
		if e != nil {
			t.Fatal(e)
		}
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		m, e := SetOccurrenceSurfaceColor(context.Background(), f, 380, RGB{0.25, 0.5, 0.75}, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = WriteOccurrenceColoredAssemblyJSON(context.Background(), &native, m); e != nil {
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
		if _, e = ReadColoredAssembly(bytes.NewReader(out.Bytes()), Limits{}); e == nil {
			t.Fatal("older reader admitted contextual authoring")
		}
		bin := os.Getenv("PLAINSOLID_OCCT_AP242_OCCURRENCE")
		if bin == "" {
			continue
		}
		path := filepath.Join(t.TempDir(), "created.step")
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
		want := map[[3]float64][3]float64{{10, 0, 0}: {0.25, 0.5, 0.75}, {0, 20, 0}: {1, 0, 0}, {-10, -5, 7}: {0, 0, 1}}
		if variant == 1 {
			want[[3]float64{0, 20, 0}] = [3]float64{0, 0, 1}
			want[[3]float64{-10, -5, 7}] = [3]float64{0, 1, 0}
		}
		for _, o := range q.Occurrences {
			expected, ok := want[o.Translation]
			if !ok {
				t.Fatal(q)
			}
			for i, v := range o.Color {
				if math.Abs(v-expected[i]) > 1e-7 {
					t.Fatal("collateral occurrence color", q)
				}
			}
		}
	}
}
func FuzzSetOccurrenceColor(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-ap242-palette-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	source, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		f.Fatal(e)
	}
	f.Add(int64(380), uint8(64), uint8(128), uint8(192))
	f.Add(int64(35), uint8(255), uint8(0), uint8(0))
	f.Fuzz(func(t *testing.T, id int64, r, g, b uint8) {
		q, e := SetOccurrenceSurfaceColor(context.Background(), source, id, RGB{float64(r) / 255, float64(g) / 255, float64(b) / 255}, Limits{})
		if e != nil {
			if len(q.Overrides) != 0 {
				t.Fatal("partial failure")
			}
			return
		}
		if len(q.Overrides) != 1 || q.Overrides[0].OccurrenceID != id {
			t.Fatal("wrong occurrence")
		}
	})
}
