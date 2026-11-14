package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestOccurrenceColoredAssembly(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ap242-occurrence-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadColoredAssembly(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("legacy accepted contextual style")
	}
	m, e := ReadOccurrenceColoredAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Overrides) != 2 || m.Overrides[0].OccurrenceID != 386 || m.Overrides[1].OccurrenceID != 386 || m.Overrides[0].Color != (RGB{0, 0, 1}) {
		t.Fatal(m.Overrides)
	}
	var out bytes.Buffer
	if e = WriteOccurrenceColoredAssemblySource(&out, m.Assembly.Assembly.Source, Limits{}); e != nil {
		t.Fatal(e)
	}
	n, e := ReadOccurrenceColoredAssembly(bytes.NewReader(out.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(n.Overrides, m.Overrides) {
		t.Fatal(n, e)
	}
}

func TestOccurrenceColorsReject(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ap242-occurrence-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{
		{"#737 = SHAPE_REPRESENTATION('',(#19),#371)", "#737 = SHAPE_REPRESENTATION('',(#15),#371)"},
		{"#736 = SHAPE_DEFINITION_REPRESENTATION(#385,#737)", "#736 = SHAPE_DEFINITION_REPRESENTATION(#379,#737)"},
		{"#737 = SHAPE_REPRESENTATION('',(#19),#371)", "#737 = SHAPE_REPRESENTATION('',(#19),#725)"},
		{"#758 = OVER_RIDING_STYLED_ITEM('overriding color',(#759),#41,#750)", "#758 = OVER_RIDING_STYLED_ITEM('overriding color',(#759),#395,#750)"},
		{"#758 = OVER_RIDING_STYLED_ITEM('overriding color',(#759),#41,#750)", "#758 = OVER_RIDING_STYLED_ITEM('overriding color',(#759),#41,#766)"},
		{"#773 = DRAUGHTING_PRE_DEFINED_COLOUR('blue')", "#773 = DRAUGHTING_PRE_DEFINED_COLOUR('green')"},
		{"#760 = SURFACE_STYLE_USAGE(.BOTH.,#761)", "#760 = SURFACE_STYLE_USAGE(.POSITIVE.,#761)"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=SHAPE_DEFINITION_REPRESENTATION(#385,#737);\nENDSEC;\nEND-ISO-10303-21;"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=DRAUGHTING_PRE_DEFINED_COLOUR('blue');\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		bad := strings.Replace(string(raw), change[0], change[1], 1)
		if bad == string(raw) {
			t.Fatal("stale mutation", change)
		}
		if q, e := ReadOccurrenceColoredAssembly(strings.NewReader(bad), Limits{}); e == nil || len(q.Overrides) != 0 || len(q.Assembly.ProductBodies) != 0 {
			t.Fatal("accepted inconsistent contextual style", change)
		}
		f, e := Parse(strings.NewReader(bad), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteOccurrenceColoredAssemblySource(&out, f, Limits{}); e == nil || out.Len() != 0 {
			t.Fatal("non-atomic invalid source export")
		}
	}
	if _, e := ReadOccurrenceColoredAssembly(bytes.NewReader(raw), Limits{MaxBytes: int64(len(raw) - 1)}); e == nil {
		t.Fatal("byte limit")
	}
	if _, e := ReadOccurrenceColoredAssembly(bytes.NewReader(raw), Limits{MaxEntities: 700}); e == nil {
		t.Fatal("entity limit")
	}
}

func TestOccurrenceColorsRetargetAndLive(t *testing.T) {
	raw, e := os.ReadFile("testdata/occt-ap242-occurrence-colors.step")
	if e != nil {
		t.Fatal(e)
	}
	retarget := strings.ReplaceAll(string(raw), "SHAPE_DEFINITION_REPRESENTATION(#385,", "SHAPE_DEFINITION_REPRESENTATION(#379,")
	retarget = strings.ReplaceAll(retarget, "SHAPE_REPRESENTATION('',(#19),#371)", "SHAPE_REPRESENTATION('',(#15),#371)")
	for variant, source := range []string{string(raw), retarget} {
		m, e := ReadOccurrenceColoredAssembly(strings.NewReader(source), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		wanted := int64(386)
		if variant == 1 {
			wanted = 380
		}
		for _, c := range m.Overrides {
			if c.OccurrenceID != wanted {
				t.Fatal("inferred wrong use", c)
			}
		}
		var out bytes.Buffer
		if e = WriteOccurrenceColoredAssemblySource(&out, m.Assembly.Assembly.Source, Limits{}); e != nil {
			t.Fatal(e)
		}
		bin := os.Getenv("PLAINSOLID_OCCT_AP242_OCCURRENCE")
		if bin == "" {
			continue
		}
		for _, data := range []string{source, out.String()} {
			path := filepath.Join(t.TempDir(), "colors.step")
			if e = os.WriteFile(path, []byte(data), 0600); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			result, err := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
			cancel()
			if err != nil {
				t.Fatalf("%v: %s", err, result)
			}
			var q struct {
				Volume, Area float64
				Occurrences  []struct {
					Translation [3]float64
					Color       [3]float64
				}
			}
			at := bytes.IndexByte(result, '{')
			if at < 0 || json.Unmarshal(result[at:], &q) != nil {
				t.Fatal(string(result))
			}
			if math.Abs(q.Volume-54) > 1e-9 || math.Abs(q.Area-126) > 1e-9 || len(q.Occurrences) != 3 {
				t.Fatal(q)
			}
			expected := map[[3]float64][3]float64{{10, 0, 0}: {1, 0, 0}, {0, 20, 0}: {0, 0, 1}, {-10, -5, 7}: {0, 1, 0}}
			if variant == 1 {
				expected[[3]float64{10, 0, 0}] = [3]float64{0, 0, 1}
				expected[[3]float64{0, 20, 0}] = [3]float64{1, 0, 0}
			}
			for _, o := range q.Occurrences {
				want, ok := expected[o.Translation]
				if !ok || want != o.Color {
					t.Fatal("effective occurrence color", q)
				}
			}
		}
	}
}

func FuzzOccurrenceColoredAssembly(f *testing.F) {
	raw, e := os.ReadFile("testdata/occt-ap242-occurrence-colors.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<16 {
			return
		}
		_, _ = ReadOccurrenceColoredAssembly(bytes.NewReader(b), Limits{MaxBytes: 1 << 16, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16})
	})
}
