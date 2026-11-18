package step

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"strings"
	"testing"
)

func coloredWrappedFixture(t testing.TB) ColoredWrappedMaterialAssemblyModel {
	t.Helper()
	raw, e := os.ReadFile("testdata/wrapped-rim-fillet.step")
	if e != nil {
		t.Fatal(e)
	}
	body, e := ReadWrappedRimFillet(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	body.CapNames = [2]string{"bottom", "top"}
	for i := range body.Walls {
		body.Walls[i].Name = []string{"lower blend", "lateral", "upper blend"}[i]
	}
	return ColoredWrappedMaterialAssemblyModel{LengthUnit: .001, Products: []ColoredWrappedMaterialAssemblyPart{{ID: "root", Name: "assembly"}, {ID: "part", Name: "fillet", Bodies: []ColoredWrappedMaterialBody{{Body: WrappedMaterialBody{RimFillet: &body}, SurfaceColor: &RGB{.2, .4, .6}, FaceColors: map[string]RGB{"bottom": {.8, .1, .3}, "lower blend": {.1, .8, .3}}}}}}, Occurrences: []AssemblyInstance{{ID: "one", Name: "first", Parent: "root", Child: "part", Transform: geom.Identity()}, {ID: "two", Name: "second", Parent: "root", Child: "part", Transform: geom.Translation(geom.Vec3{X: 16})}}}
}
func TestColoredWrappedMaterialAssembly(t *testing.T) {
	slowtest.Skip(t)
	m := coloredWrappedFixture(t)
	var b bytes.Buffer
	if e := WriteColoredWrappedMaterialAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadColoredWrappedMaterialAssembly(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, bs := range a.ProductBodies {
		for _, v := range bs {
			count++
			if v.Body.RimFillet == nil || v.SurfaceColor == nil || *v.SurfaceColor != (RGB{.2, .4, .6}) || len(v.FaceColors) != 2 || v.FaceColors["bottom"] != (RGB{.8, .1, .3}) {
				t.Fatal(v)
			}
		}
	}
	if count != 1 || len(a.Assembly.Occurrences) != 2 {
		t.Fatal(count)
	}

	// Source shell ordering is not a style binding: keep original face targets.
	f, err := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	remap := map[int64]int64{}
	for _, id := range f.Order {
		remap[id] = id
	}
	lines := strings.Split(b.String(), "\n")
	for _, id := range f.Order {
		en := f.Entities[id]
		if len(en.Records) != 1 || en.Records[0].Type != "CLOSED_SHELL" {
			continue
		}
		items := en.Records[0].Args[1].Items
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
		for i, line := range lines {
			if strings.HasPrefix(line, fmt.Sprintf("#%d=", id)) {
				lines[i] = fmt.Sprintf("#%d=%s;", id, assemblyEntity(en, remap))
			}
		}
	}
	shuffled, err := ReadColoredWrappedMaterialAssembly(strings.NewReader(strings.Join(lines, "\n")), Limits{}, WrappedSurfaceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, bs := range shuffled.ProductBodies {
		for _, body := range bs {
			if body.FaceColors["bottom"] != (RGB{.8, .1, .3}) {
				t.Fatal("face order changed style", body)
			}
		}
	}
	var again bytes.Buffer
	if e = WriteColoredWrappedMaterialAssembly(&again, m); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("nondeterministic", e)
	}
	for _, kind := range []string{"unknown", "empty", "duplicate", "nan"} {
		t.Run(kind, func(t *testing.T) {
			m := coloredWrappedFixture(t)
			v := &m.Products[1].Bodies[0]
			switch kind {
			case "unknown":
				v.FaceColors["missing"] = RGB{}
			case "empty":
				v.FaceColors[""] = RGB{}
			case "duplicate":
				v.Body.RimFillet.CapNames[1] = "bottom"
			case "nan":
				v.SurfaceColor = &RGB{R: math.NaN()}
			}
			var out bytes.Buffer
			if e := WriteColoredWrappedMaterialAssembly(&out, m); e == nil || out.Len() != 0 {
				t.Fatal("invalid styles emitted")
			}
		})
	}
	// Explicit void families retain body defaults without changing geometry scope.
	raw, e := os.ReadFile("testdata/wrapped-cavity.step")
	if e != nil {
		t.Fatal(e)
	}
	cavity, e := ReadWrappedCavity(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Products[1].Bodies = append(m.Products[1].Bodies, ColoredWrappedMaterialBody{Body: WrappedMaterialBody{Cavity: &cavity}, SurfaceColor: &RGB{.3, .4, .5}})
	b.Reset()
	if e = WriteColoredWrappedMaterialAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadColoredWrappedMaterialAssembly(&b, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
}
func TestColoredWrappedMaterialAssemblyOCCT(t *testing.T) {
	var b bytes.Buffer
	if e := WriteColoredWrappedMaterialAssembly(&b, coloredWrappedFixture(t)); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/colored.step"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/colors.json"
	stepOracle(t, out, "step-colored-assembly", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Occurrences            []struct{ Name string }
		BodyColors, FaceColors [][3]float64
	}
	if e = json.Unmarshal(raw, &got); e != nil || len(got.Occurrences) != 2 || len(got.BodyColors) != 1 || len(got.FaceColors) != 2 {
		t.Fatal(got, e)
	}
	for i, want := range []float64{.2, .4, .6} {
		if math.Abs(got.BodyColors[0][i]-want) > 1e-6 {
			t.Fatal(got)
		}
	}
	for _, want := range [][3]float64{{.8, .1, .3}, {.1, .8, .3}} {
		found := false
		for _, rgb := range got.FaceColors {
			if math.Abs(rgb[0]-want[0])+math.Abs(rgb[1]-want[1])+math.Abs(rgb[2]-want[2]) < 1e-6 {
				found = true
			}
		}
		if !found {
			t.Fatal(got)
		}
	}
}
