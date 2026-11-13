package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func rationalLensStyle() LensStyle {
	a, b, c := RGB{R: .25, G: .5, B: .75}, RGB{R: .75, G: .25, B: .5}, RGB{R: .125, G: .5, B: .875}
	return LensStyle{SurfaceColor: &a, FaceColors: [2]*RGB{&b, &c}}
}
func TestRationalLensStyle(t *testing.T) {
	m := rationalConcurrentLensFixture(t)
	m.Style = rationalLensStyle()
	var b bytes.Buffer
	if e := WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadRationalSplineLensBody(&b, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got.Style, m.Style) {
		t.Fatal(got.Style, m.Style)
	}
	m.Style.FaceColors[0].R = 2
	b.Reset()
	if e = WriteRationalSplineLensBody(&b, m, CurvedOvalOptions{}); e == nil || b.Len() != 0 {
		t.Fatal("invalid style", e)
	}
}
func TestRationalLensAssemblyStyles(t *testing.T) {
	m := lensAssemblyFixture(t)
	r := rationalConcurrentLensFixture(t)
	m.Products[1].Bodies = []SplineLensBody{r.Body}
	m.Products[1].RationalSeamCurves = r.Curves
	m.Products[1].Style = rationalLensStyle()
	var b bytes.Buffer
	if e := WriteRationalSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadRationalSplineLensAssembly(&b, Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pid := a.Assembly.Occurrences[0].Child
	if len(a.ProductRationalCurves[pid]) != 7 || !reflect.DeepEqual(a.ProductStyles[pid], m.Products[1].Style) {
		t.Fatal("raw product/style loss")
	}
}

func TestIndependentRationalLensAssemblyStyles(t *testing.T) {
	raw, e := os.ReadFile("testdata/rational-lens-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadRationalSplineLensAssembly(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pid := a.Assembly.Occurrences[0].Child
	if len(a.Assembly.Occurrences) != 2 || pid != a.Assembly.Occurrences[1].Child || !nearLensStyle(a.ProductStyles[pid], rationalLensStyle()) {
		t.Fatal("independent product/styles", a.ProductStyles)
	}
}
func TestRationalLensAssemblyStylesOCCT(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-rational-lens-assembly")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadRationalSplineLensAssembly(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	pid := a.Assembly.Occurrences[0].Child
	m := lensAssemblyFixture(t)
	m.LengthUnit = a.Assembly.LengthUnit
	m.Products[1].Bodies = a.ProductBodies[pid]
	m.Products[1].RationalSeamCurves = a.ProductRationalCurves[pid]
	m.Products[1].Style = a.ProductStyles[pid]
	var b bytes.Buffer
	if e = WriteRationalSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/target.step"
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/styles.json"
	stepOracle(t, out, "step-colored-assembly", path)
	raw, e = os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Occurrences            []struct{ Name string }
		BodyColors, FaceColors [][3]float64
	}
	if e = json.Unmarshal(raw, &got); e != nil {
		t.Fatal(e)
	}
	if len(got.Occurrences) != 2 || len(got.BodyColors) != 1 || len(got.FaceColors) != 2 {
		t.Fatal(got)
	}
	if !nearLensRGB(got.BodyColors[0], [3]float64{.25, .5, .75}) {
		t.Fatal(got)
	}
	for _, want := range [][3]float64{{.75, .25, .5}, {.125, .5, .875}} {
		found := false
		for _, c := range got.FaceColors {
			found = found || nearLensRGB(c, want)
		}
		if !found {
			t.Fatal(got)
		}
	}
}

func nearLensRGB(a, b [3]float64) bool {
	return math.Abs(a[0]-b[0])+math.Abs(a[1]-b[1])+math.Abs(a[2]-b[2]) < 1e-6
}
func nearLensStyle(a, b LensStyle) bool {
	aa := []*RGB{a.SurfaceColor, a.FaceColors[0], a.FaceColors[1]}
	bb := []*RGB{b.SurfaceColor, b.FaceColors[0], b.FaceColors[1]}
	for i, c := range aa {
		d := bb[i]
		if c == nil || d == nil {
			return false
		}
		if !nearLensRGB([3]float64{c.R, c.G, c.B}, [3]float64{d.R, d.G, d.B}) {
			return false
		}
	}
	return true
}

func TestRationalLensRejectsOccurrenceStyle(t *testing.T) {
	raw, e := os.ReadFile("testdata/rational-lens-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	occ := int64(0)
	for _, id := range f.Order {
		if _, e := simple(f, id, "NEXT_ASSEMBLY_USAGE_OCCURRENCE", 6); e == nil {
			occ = id
			break
		}
	}
	if occ == 0 {
		t.Fatal("missing occurrence")
	}
	changed := false
	for _, id := range f.Order {
		r, e := ovalRecord(f, id, "STYLED_ITEM", 3)
		if e == nil {
			r.Args[2] = Value{Kind: Reference, Ref: occ}
			ovalSet(f, id, r)
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("style mutation missed")
	}
	if _, e = ReadRationalSplineLensAssembly(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("occurrence style silently accepted")
	}
}

func FuzzRationalLensAssembly(f *testing.F) {
	raw, e := os.ReadFile("testdata/rational-lens-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadRationalSplineLensAssembly(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxEntities: 1024, MaxValues: 20000, MaxFaces: 4, MaxVertices: 512, MaxEdgeUses: 32, MaxDepth: 24}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}
