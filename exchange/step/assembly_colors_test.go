package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func coloredAssemblyModel(t *testing.T) ColoredAssemblyModel {
	base := nestedAssemblyModel(t)
	m := ColoredAssemblyModel{LengthUnit: base.LengthUnit, Occurrences: base.Occurrences}
	for _, p := range base.Products {
		part := ColoredAssemblyPart{ID: p.ID, Name: p.Name}
		for _, b := range p.Bodies {
			color := RGB{.2, .4, .6}
			part.Bodies = append(part.Bodies, FaceColoredBody{b, &color, map[int]RGB{0: {.8, .1, .3}}})
		}
		m.Products = append(m.Products, part)
	}
	return m
}
func TestColoredAssemblyRoundtrip(t *testing.T) {
	t.Parallel()
	m := coloredAssemblyModel(t)
	var b bytes.Buffer
	if e := WriteColoredAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadColoredAssembly(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Products) != 3 || len(a.Assembly.Occurrences) != 3 {
		t.Fatal(a)
	}
	for _, p := range a.Assembly.Products {
		bodies := a.ProductBodies[p.DefinitionID]
		if p.Identifier == "part" {
			if len(bodies) != 1 || bodies[0].SurfaceColor == nil || *bodies[0].SurfaceColor != (RGB{.2, .4, .6}) || bodies[0].FaceColors[0] != (RGB{.8, .1, .3}) {
				t.Fatal(bodies)
			}
		} else if len(bodies) != 0 {
			t.Fatal(bodies)
		}
	}
	for _, o := range a.Assembly.Occurrences {
		for _, want := range m.Occurrences {
			if o.Identifier == want.ID && o.Transform != want.Transform {
				for i := 0; i < 3; i++ {
					for j := 0; j < 4; j++ {
						if math.Abs(o.Transform[i][j]-want.Transform[i][j]) > 1e-12 {
							t.Fatal(o, want)
						}
					}
				}
			}
		}
	}
}
func TestOCCTColoredAssembly(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-colored-assembly")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadColoredAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Occurrences) != 2 {
		t.Fatal(a)
	}
	colored := 0
	for _, bs := range a.ProductBodies {
		for _, b := range bs {
			if b.SurfaceColor != nil && len(b.FaceColors) == 1 {
				colored++
			}
		}
	}
	if colored != 1 {
		t.Fatal(a)
	}
	var b bytes.Buffer
	if e := WriteColoredAssembly(&b, coloredAssemblyModel(t)); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "ours.step")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "result.json")
	stepOracle(t, out, "step-colored-assembly", path)
	raw, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		Volume      float64
		Products    []string
		Occurrences []struct {
			Name      string
			Transform [3][4]float64
		}
		BodyColors, FaceColors [][3]float64
	}
	if e := json.Unmarshal(raw, &result); e != nil {
		t.Fatal(e)
	}
	if math.Abs(result.Volume-12) > 1e-8 || len(result.Products) != 3 || len(result.Occurrences) != 3 || len(result.BodyColors) != 1 || len(result.FaceColors) != 1 {
		t.Fatal(result)
	}
	for _, name := range []string{"Machine", "Module", "Bracket"} {
		found := false
		for _, got := range result.Products {
			found = found || got == name
		}
		if !found {
			t.Fatal(result)
		}
	}
	for _, want := range coloredAssemblyModel(t).Occurrences {
		found := false
		for _, got := range result.Occurrences {
			if got.Name != want.Name {
				continue
			}
			found = true
			for i := 0; i < 3; i++ {
				for j := 0; j < 4; j++ {
					if math.Abs(got.Transform[i][j]-want.Transform[i][j]) > 1e-10 {
						t.Fatal(got, want)
					}
				}
			}
		}
		if !found {
			t.Fatal(result)
		}
	}
	for i, v := range [3]float64{.2, .4, .6} {
		if math.Abs(result.BodyColors[0][i]-v) > 1e-6 {
			t.Fatal(result)
		}
	}
	for i, v := range [3]float64{.8, .1, .3} {
		if math.Abs(result.FaceColors[0][i]-v) > 1e-6 {
			t.Fatal(result)
		}
	}
}

func TestIndependentColoredAssembly(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/colored-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadColoredAssembly(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, bodies := range a.ProductBodies {
		for _, b := range bodies {
			if b.SurfaceColor == nil || len(b.FaceColors) != 1 {
				t.Fatal(b)
			}
			c := b.SurfaceColor
			for i, v := range []float64{c.R, c.G, c.B} {
				if math.Abs(v-[]float64{.2, .4, .6}[i]) > 1e-6 {
					t.Fatal(c)
				}
			}
			for _, c := range b.FaceColors {
				if math.Abs(c.R-.8) > 1e-6 || math.Abs(c.G-.1) > 1e-6 || math.Abs(c.B-.3) > 1e-6 {
					t.Fatal(c)
				}
			}
		}
	}
	if _, e := ReadColoredAssembly(bytes.NewReader(raw), Limits{MaxFaces: 5}); e == nil {
		t.Fatal("face budget bypass")
	}
}
func TestColoredAssemblyMalformed(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/colored-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []struct{ old, new string }{
		{"#392 = OVER_RIDING_STYLED_ITEM('overriding color',(#393),#39,#384);", "#392 = OVER_RIDING_STYLED_ITEM('overriding color',(#393),#39,#392);"},
		{"#392 = OVER_RIDING_STYLED_ITEM('overriding color',(#393),#39,#384);", "#392 = OVER_RIDING_STYLED_ITEM('overriding color',(#393),#37,#384);"},
		{"#384 = STYLED_ITEM('color',(#385),#37);", "#384 = STYLED_ITEM('color',(#385),#39);"},
	} {
		changed := bytes.Replace(raw, []byte(mutation.old), []byte(mutation.new), 1)
		if bytes.Equal(changed, raw) {
			t.Fatal("stale mutation")
		}
		if _, e := ReadColoredAssembly(bytes.NewReader(changed), Limits{}); e == nil {
			t.Fatal("accepted invalid override")
		}
	}
	m := coloredAssemblyModel(t)
	m.Products[2].Bodies[0].FaceColors[100] = RGB{1, 0, 0}
	var b bytes.Buffer
	if e := WriteColoredAssembly(&b, m); e == nil || b.Len() != 0 {
		t.Fatal(e)
	}
	if e := WriteColoredAssembly(shortWriter{}, coloredAssemblyModel(t)); e == nil {
		t.Fatal("short writer")
	}
}
func FuzzColoredAssembly(f *testing.F) {
	raw, e := os.ReadFile("testdata/colored-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadColoredAssembly(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 512, MaxFaces: 256, MaxSolids: 16, MaxEdgeUses: 2048})
	})
}
