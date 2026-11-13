package step

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func coloredSplineAssemblyFixture(t testing.TB) ColoredSplineShellAssemblyModel {
	t.Helper()
	base := splineAssemblyFixture(t)
	m := ColoredSplineShellAssemblyModel{LengthUnit: base.LengthUnit, Occurrences: base.Occurrences}
	for _, p := range base.Products {
		part := ColoredSplineShellAssemblyPart{ID: p.ID, Name: p.Name}
		for _, b := range p.Bodies {
			rgb := RGB{.2, .4, .6}
			part.Bodies = append(part.Bodies, ColoredSplineShellBody{Body: b, SurfaceColor: &rgb, FaceColors: map[int]RGB{0: {.8, .1, .3}}})
		}
		m.Products = append(m.Products, part)
	}
	return m
}
func TestColoredSplineShellAssembly(t *testing.T) {
	m := coloredSplineAssemblyFixture(t)
	var b bytes.Buffer
	if e := WriteColoredSplineShellAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadColoredSplineShellAssembly(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, p := range a.Assembly.Products {
		for _, body := range a.ProductBodies[p.DefinitionID] {
			count++
			if body.SurfaceColor == nil || *body.SurfaceColor != (RGB{.2, .4, .6}) || len(body.FaceColors) != 1 || body.FaceColors[0] != (RGB{.8, .1, .3}) || len(body.Body.Faces) != 6 {
				t.Fatal(body)
			}
		}
	}
	if count != 1 || len(a.Assembly.Occurrences) != 2 {
		t.Fatal(a)
	}
	if _, e := ReadSplineShellAssembly(bytes.NewReader(b.Bytes()), Limits{}); e != nil {
		t.Fatal("uncolored geometry API changed", e)
	}
	var again bytes.Buffer
	if e = WriteColoredSplineShellAssembly(&again, m); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("nondeterministic styles", e)
	}
	// The unsupported target is syntactically valid and remains in the assembly.
	f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	remap := map[int64]int64{}
	var occurrence int64
	for _, id := range f.Order {
		remap[id] = id
		for _, r := range f.Entities[id].Records {
			if r.Type == "NEXT_ASSEMBLY_USAGE_OCCURRENCE" {
				occurrence = id
			}
		}
	}
	for _, id := range f.Order {
		ent := f.Entities[id]
		if len(ent.Records) != 1 || ent.Records[0].Type != "STYLED_ITEM" {
			continue
		}
		ent.Records[0].Args[2] = Value{Kind: Reference, Ref: occurrence}
		lines := strings.Split(b.String(), "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, fmt.Sprintf("#%d=", id)) {
				lines[i] = fmt.Sprintf("#%d=%s;", id, assemblyEntity(ent, remap))
			}
		}
		if _, e := ReadColoredSplineShellAssembly(strings.NewReader(strings.Join(lines, "\n")), Limits{}); e == nil {
			t.Fatal("occurrence override accepted")
		}
		break
	}
}
func TestColoredSplineShellReject(t *testing.T) {
	for _, kind := range []string{"index", "nan", "negative", "invalid-body"} {
		t.Run(kind, func(t *testing.T) {
			m := coloredSplineAssemblyFixture(t)
			b := &m.Products[1].Bodies[0]
			switch kind {
			case "index":
				b.FaceColors[6] = RGB{}
			case "nan":
				b.SurfaceColor = &RGB{math.NaN(), 0, 0}
			case "negative":
				b.FaceColors[-1] = RGB{}
			case "invalid-body":
				b.Body.Faces = b.Body.Faces[:5]
			}
			var out bytes.Buffer
			if e := WriteColoredSplineShellAssembly(&out, m); e == nil || out.Len() != 0 {
				t.Fatal(e, out.Len())
			}
		})
	}
}
func TestOCCTColoredSplineShellAssembly(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-colored-spline-assembly")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadColoredSplineShellAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, bs := range a.ProductBodies {
		for _, b := range bs {
			if b.SurfaceColor == nil || len(b.FaceColors) != 1 {
				t.Fatal(b)
			}
			count++
		}
	}
	if count != 1 || len(a.Assembly.Occurrences) != 2 {
		t.Fatal(a)
	}
	var b bytes.Buffer
	if e = WriteColoredSplineShellAssembly(&b, coloredSplineAssemblyFixture(t)); e != nil {
		t.Fatal(e)
	}
	native := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(native, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{source, native} {
		out := filepath.Join(t.TempDir(), "colors.json")
		stepOracle(t, out, "step-colored-assembly", path)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var got struct {
			Volume                 float64
			Products               []string
			Occurrences            []struct{ Name string }
			BodyColors, FaceColors [][3]float64
		}
		if e = json.Unmarshal(data, &got); e != nil || math.Abs(got.Volume-10) > 1e-7 || len(got.Products) != 2 || len(got.Occurrences) != 2 || len(got.BodyColors) != 1 || len(got.FaceColors) != 1 {
			t.Fatal(got, e)
		}
		for i, v := range []float64{.2, .4, .6} {
			if math.Abs(got.BodyColors[0][i]-v) > 1e-6 {
				t.Fatal(got)
			}
		}
		for i, v := range []float64{.8, .1, .3} {
			if math.Abs(got.FaceColors[0][i]-v) > 1e-6 {
				t.Fatal(got)
			}
		}
	}
}

func TestIndependentColoredSplineAssembly(t *testing.T) {
	raw, e := os.ReadFile("testdata/colored-spline-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadColoredSplineShellAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, bs := range a.ProductBodies {
		for _, b := range bs {
			count++
			if b.SurfaceColor == nil || len(b.FaceColors) != 1 {
				t.Fatal(b)
			}
			c := b.SurfaceColor
			for i, v := range []float64{c.R, c.G, c.B} {
				if math.Abs(v-[]float64{.2, .4, .6}[i]) > 1e-6 {
					t.Fatal(c)
				}
			}
			for index, c := range b.FaceColors {
				if index < 0 || index >= 6 {
					t.Fatal(index)
				}
				for i, v := range []float64{c.R, c.G, c.B} {
					if math.Abs(v-[]float64{.8, .1, .3}[i]) > 1e-6 {
						t.Fatal(c)
					}
				}
			}
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
}
func FuzzColoredSplineAssembly(f *testing.F) {
	raw, e := os.ReadFile("testdata/colored-spline-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadColoredSplineShellAssembly(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 4096, MaxValues: 32768, MaxDepth: 24, MaxVertices: 256, MaxFaces: 32, MaxSolids: 8, MaxEdgeUses: 256})
	})
}
