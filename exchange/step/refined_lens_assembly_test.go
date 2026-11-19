package step

import (
	"bytes"
	"os"
	"testing"
)

func TestRefinedLensAssembly(t *testing.T) {
	t.Parallel()
	m := lensAssemblyFixture(t)
	r := refinedCurveLensFixture(t)
	m.Products[1].Bodies = []SplineLensBody{r.Body}
	m.Products[1].SeamCurves = r.Curves
	var b bytes.Buffer
	if e := WriteRefinedSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadRefinedSplineLensAssembly(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Occurrences) != 2 || a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal("sharing")
	}
	pid := a.Assembly.Occurrences[0].Child
	if len(a.ProductCurves[pid]) != 4 {
		t.Fatal("curve bases")
	}
	b.Reset()
	if e = WritePartitionedSplineLensAssembly(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("old writer dropped source bases")
	}
}
func TestRefinedLensAssemblyOCCT(t *testing.T) {
	t.Parallel()
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-refined-curve-lens-assembly")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadRefinedSplineLensAssembly(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Occurrences) != 2 || a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal("independent sharing")
	}
	pid := a.Assembly.Occurrences[0].Child
	m := lensAssemblyFixture(t)
	m.Products[1].Bodies = a.ProductBodies[pid]
	m.Products[1].SeamCurves = a.ProductCurves[pid]
	var b bytes.Buffer
	if e = WriteRefinedSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	target := t.TempDir() + "/native.step"
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	stepOracle(t, t.TempDir()+"/mass.json", "step-mass-precise", target)
}

func FuzzRefinedLensAssembly(f *testing.F) {
	raw, e := os.ReadFile("testdata/refined-curve-lens-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadRefinedSplineLensAssembly(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxEntities: 1024, MaxValues: 20000, MaxFaces: 4, MaxVertices: 512, MaxEdgeUses: 32, MaxDepth: 24}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}

func TestRefinedLensAssemblyRejectsUnownedCurves(t *testing.T) {
	t.Parallel()
	m := lensAssemblyFixture(t)
	r := refinedCurveLensFixture(t)
	m.Products[1].Bodies = []SplineLensBody{r.Body}
	m.Products[1].SeamCurves = r.Curves
	m.Products[0].SeamCurves = r.Curves
	var b bytes.Buffer
	if e := WriteRefinedSplineLensAssembly(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("unowned curves accepted", e)
	}
}
