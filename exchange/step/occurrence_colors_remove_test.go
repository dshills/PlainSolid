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
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRemoveOccurrenceColorRestoresSource(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"occt-ap242-palette-assembly.step", "occt-ap242-occurrence-colors.step"} {
		raw, e := os.ReadFile("testdata/" + file)
		if e != nil {
			t.Fatal(e)
		}
		source, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		created, e := SetOccurrenceSurfaceColor(context.Background(), source, 380, RGB{1, 1, 0}, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		result, e := RemoveOccurrenceSurfaceColor(context.Background(), created.Assembly.Assembly.Source, 380, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(result.Removed) != 1 || !reflect.DeepEqual(result.Appearance.Assembly.Assembly.Source, source) {
			t.Fatal("creation/removal did not restore exact original graph")
		}
		again, e := RemoveOccurrenceSurfaceColor(context.Background(), result.Appearance.Assembly.Assembly.Source, 380, Limits{})
		if e != nil || len(again.Removed) != 0 || !reflect.DeepEqual(result.Appearance, again.Appearance) {
			t.Fatal("no-op changed source", e)
		}
	}
}
func TestRemoveOccurrenceSharedFill(t *testing.T) {
	t.Parallel()
	for _, whole := range []bool{false, true} {
		source := sharedOccurrenceColorSource(t, whole)
		result, e := RemoveOccurrenceSurfaceColor(context.Background(), source, 386, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(result.Removed) != 2 || len(result.Appearance.Overrides) != 0 {
			t.Fatal(result.Removed)
		}
		id := int64(757)
		if whole {
			id = 752
		}
		if !reflect.DeepEqual(source.Entities[id], result.Appearance.Assembly.Assembly.Source.Entities[id]) {
			t.Fatal("shared default removed")
		}
		var raw bytes.Buffer
		if e = WriteFlatColoredAssemblySource(context.Background(), &raw, result.Appearance.Assembly.Assembly.Source, Limits{}); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadColoredAssembly(bytes.NewReader(raw.Bytes()), Limits{}); e != nil {
			t.Fatal(e)
		}
		if _, e = ReadOccurrenceColoredAssembly(bytes.NewReader(raw.Bytes()), Limits{}); e == nil {
			t.Fatal("contextual-only reader accepted no overrides")
		}
	}
}

func TestRemoveOccurrenceColorReject(t *testing.T) {
	t.Parallel()
	m, _ := occurrenceColorsNativeFixture(t)
	source := m.Assembly.Assembly.Source
	for _, id := range []int64{0, -1, 35, 41, 99999} {
		if q, e := RemoveOccurrenceSurfaceColor(context.Background(), source, id, Limits{}); e == nil || len(q.Removed) != 0 || q.Appearance.Assembly.Assembly.Source != nil {
			t.Fatal("unknown target", id)
		}
	}
	if _, e := RemoveOccurrenceSurfaceColor(context.Background(), source, 386, Limits{MaxBytes: 1}); e == nil {
		t.Fatal("byte limit")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := RemoveOccurrenceSurfaceColor(ctx, source, 386, Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e := WriteFlatColoredAssemblySource(ctx, &out, source, Limits{}); !errors.Is(e, context.Canceled) || out.Len() != 0 {
		t.Fatal(e)
	}
	bad, e := mappedPlanarSourceBytes(source)
	if e != nil {
		t.Fatal(e)
	}
	text := strings.Replace(string(bad), "SHAPE_REPRESENTATION('',(#19),#371)", "SHAPE_REPRESENTATION('',(#15),#371)", 1)
	if text == string(bad) {
		t.Fatal("stale mutation")
	}
	invalid, e := Parse(strings.NewReader(text), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = RemoveOccurrenceSurfaceColor(context.Background(), invalid, 386, Limits{}); e == nil {
		t.Fatal("malformed ownership")
	}
	if e = WriteFlatColoredAssemblySource(context.Background(), &out, invalid, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("non-atomic invalid export")
	}
	if e = WriteFlatColoredAssemblySource(context.Background(), occurrenceShortWriter{}, source, Limits{}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	result, e := RemoveOccurrenceSurfaceColor(context.Background(), source, 386, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if e = WriteOccurrenceColoredAssemblyJSON(context.Background(), &out, result.Appearance); e == nil || out.Len() != 0 {
		t.Fatal("old contextual native tag widened to default-only source")
	}
}
func TestRemoveOccurrenceColorLive(t *testing.T) {
	t.Parallel()
	original, _ := occurrenceColorsNativeFixture(t)
	added, e := SetOccurrenceSurfaceColor(context.Background(), original.Assembly.Assembly.Source, 380, RGB{1, 1, 0}, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i, source := range []*File{original.Assembly.Assembly.Source, added.Assembly.Assembly.Source, sharedOccurrenceColorSource(t, true)} {
		result, e := RemoveOccurrenceSurfaceColor(context.Background(), source, 386, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if len(result.Removed) != 2 {
			t.Fatal(result.Removed)
		}
		if i == 1 {
			var native bytes.Buffer
			if e = WriteOccurrenceColoredAssemblyJSON(context.Background(), &native, result.Appearance); e != nil {
				t.Fatal(e)
			}
			restored, err := ReadOccurrenceColoredAssemblyJSON(context.Background(), bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
			if err != nil {
				t.Fatal(err)
			}
			result.Appearance = restored
		}
		var out bytes.Buffer
		if e = WriteFlatColoredAssemblySource(context.Background(), &out, result.Appearance.Assembly.Assembly.Source, Limits{}); e != nil {
			t.Fatal(e)
		}
		if i != 1 {
			if _, e = ReadColoredAssembly(bytes.NewReader(out.Bytes()), Limits{}); e != nil {
				t.Fatal(e)
			}
		}
		bin := os.Getenv("PLAINSOLID_OCCT_AP242_OCCURRENCE")
		if bin == "" {
			continue
		}
		path := filepath.Join(t.TempDir(), "removed.step")
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
		want := map[[3]float64][3]float64{{10, 0, 0}: {1, 0, 0}, {0, 20, 0}: {1, 0, 0}, {-10, -5, 7}: {0, 1, 0}}
		if i == 1 {
			want[[3]float64{10, 0, 0}] = [3]float64{1, 1, 0}
		}
		for _, o := range q.Occurrences {
			expected, ok := want[o.Translation]
			if !ok {
				t.Fatal(q)
			}
			for k, v := range o.Color {
				if math.Abs(v-expected[k]) > 1e-12 {
					t.Fatal("default/other override changed", q)
				}
			}
		}
	}
}
func FuzzRemoveOccurrenceColor(f *testing.F) {
	m, _ := occurrenceColorsNativeFixture(f)
	source := m.Assembly.Assembly.Source
	f.Add(int64(386))
	f.Add(int64(380))
	f.Add(int64(35))
	f.Fuzz(func(t *testing.T, id int64) {
		result, e := RemoveOccurrenceSurfaceColor(context.Background(), source, id, Limits{})
		if e != nil {
			if len(result.Removed) != 0 || result.Appearance.Assembly.Assembly.Source != nil {
				t.Fatal("partial failure")
			}
			return
		}
		if id != 380 && id != 386 && id != 734 {
			t.Fatal("unknown target accepted")
		}
		for _, c := range result.Removed {
			if c.OccurrenceID != id {
				t.Fatal("wrong claims removed")
			}
		}
	})
}
