package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAP242PaletteAssembly(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-ap242-palette-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(raw, []byte("AP242_MANAGED_MODEL_BASED_3D_ENGINEERING")) {
		t.Fatal("not original AP242")
	}
	a, e := ReadColoredAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Occurrences) != 3 {
		t.Fatal(a.Assembly.Occurrences)
	}
	colors := map[RGB]int{}
	for _, bs := range a.ProductBodies {
		for _, b := range bs {
			if b.SurfaceColor != nil {
				colors[*b.SurfaceColor]++
			}
		}
	}
	if colors[RGB{1, 0, 0}] != 1 || colors[RGB{0, 0, 1}] != 1 || len(colors) != 2 {
		t.Fatal(colors)
	}
	for _, name := range []string{"RED", "orange", "", "red ", "grey"} {
		bad := strings.Replace(string(raw), "DRAUGHTING_PRE_DEFINED_COLOUR('red')", "DRAUGHTING_PRE_DEFINED_COLOUR('"+name+"')", 1)
		if bad == string(raw) {
			t.Fatal("stale mutation")
		}
		if q, e := ReadColoredAssembly(strings.NewReader(bad), Limits{}); e == nil || len(q.ProductBodies) != 0 {
			t.Fatal("accepted unknown palette", name)
		}
	}
}

func TestAP242PaletteRoundtripLive(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-ap242-palette-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadColoredAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	model := ColoredAssemblyModel{LengthUnit: a.Assembly.LengthUnit}
	key := func(id int64) string { return strconv.FormatInt(id, 10) }
	for _, p := range a.Assembly.Products {
		model.Products = append(model.Products, ColoredAssemblyPart{ID: key(p.DefinitionID), Name: p.Name, Bodies: a.ProductBodies[p.DefinitionID]})
	}
	for _, o := range a.Assembly.Occurrences {
		model.Occurrences = append(model.Occurrences, AssemblyInstance{ID: o.Identifier, Name: o.Name, Parent: key(o.Parent), Child: key(o.Child), Transform: o.Transform})
	}
	var output bytes.Buffer
	if e = WriteColoredAssembly(&output, model); e != nil {
		t.Fatal(e)
	}
	b, e := ReadColoredAssembly(bytes.NewReader(output.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(b.Assembly.Occurrences) != 3 || b.Assembly.LengthUnit != a.Assembly.LengthUnit {
		t.Fatal("occurrence or unit loss")
	}
	products := map[int64]string{}
	for _, p := range b.Assembly.Products {
		products[p.DefinitionID] = p.Identifier
	}
	for _, got := range b.Assembly.Occurrences {
		matched := false
		for _, want := range a.Assembly.Occurrences {
			if got.Identifier != want.Identifier {
				continue
			}
			matched = true
			if got.Name != want.Name || got.Transform != want.Transform || products[got.Parent] != key(want.Parent) || products[got.Child] != key(want.Child) {
				t.Fatal("occurrence semantics changed", got, want)
			}
		}
		if !matched {
			t.Fatal("unknown occurrence", got)
		}
	}
	for _, p := range b.Assembly.Products {
		source, err := strconv.ParseInt(p.Identifier, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		before, after := a.ProductBodies[source], b.ProductBodies[p.DefinitionID]
		if len(before) != len(after) {
			t.Fatal("local product loss")
		}
		for i := range before {
			if before[i].SurfaceColor == nil || after[i].SurfaceColor == nil || *before[i].SurfaceColor != *after[i].SurfaceColor {
				t.Fatal("color loss")
			}
		}
	}
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_PALETTE")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_AP242_PALETTE for live XCAF readback")
	}
	for _, data := range [][]byte{raw, output.Bytes()} {
		path := filepath.Join(t.TempDir(), "palette.step")
		if e = os.WriteFile(path, data, 0600); e != nil {
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
			Colors       [][3]float64
		}
		at := bytes.IndexByte(result, '{')
		if at < 0 || json.Unmarshal(result[at:], &q) != nil {
			t.Fatal(string(result))
		}
		if math.Abs(q.Volume-54) > 1e-9 || math.Abs(q.Area-126) > 1e-9 || len(q.Colors) != 2 {
			t.Fatal(q)
		}
		colors := map[[3]float64]int{}
		for _, c := range q.Colors {
			colors[c]++
		}
		if colors[[3]float64{1, 0, 0}] != 1 || colors[[3]float64{0, 0, 1}] != 1 {
			t.Fatal(q)
		}
	}
}

func TestDraughtingPaletteExact(t *testing.T) {
	t.Parallel()
	names := []string{"black", "white", "red", "green", "blue", "yellow", "magenta", "cyan"}
	want := []RGB{{0, 0, 0}, {1, 1, 1}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {1, 1, 0}, {1, 0, 1}, {0, 1, 1}}
	for i, name := range names {
		f := &File{Entities: map[int64]Entity{1: {ID: 1, Records: []Record{{Type: "DRAUGHTING_PRE_DEFINED_COLOUR", Args: []Value{{Kind: String, Text: name}}}}}}}
		got, e := readSurfaceStyleColour(f, 1)
		if e != nil || got != want[i] {
			t.Fatal(name, got, e)
		}
	}
}

func TestAP242PaletteOwnershipReject(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/occt-ap242-palette-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, change := range [][2]string{
		{"STYLED_ITEM('color',(#747),#41)", "STYLED_ITEM('color',(#747),#395)"},
		{"PRESENTATION_STYLE_ASSIGNMENT((#748))", "PRESENTATION_STYLE_BY_CONTEXT((#748),#10)"},
		{"DRAUGHTING_PRE_DEFINED_COLOUR('red')", "PRE_DEFINED_COLOUR('red')"},
		{"ENDSEC;\nEND-ISO-10303-21;", "#9999=DRAUGHTING_PRE_DEFINED_COLOUR('red');\nENDSEC;\nEND-ISO-10303-21;"},
	} {
		bad := strings.Replace(string(raw), change[0], change[1], 1)
		if bad == string(raw) {
			t.Fatal("stale mutation", change[0])
		}
		if q, e := ReadColoredAssembly(strings.NewReader(bad), Limits{}); e == nil || len(q.ProductBodies) != 0 {
			t.Fatal("accepted bad ownership", change)
		}
	}
	if _, e := ReadColoredAssembly(bytes.NewReader(raw), Limits{MaxBytes: int64(len(raw) - 1)}); e == nil {
		t.Fatal("byte bound")
	}
}
