package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestPartitionedLensAssembly(t *testing.T) {
	m := lensAssemblyFixture(t)
	m.Products[1].Bodies[0] = partitionedLensFixture(t)
	var b bytes.Buffer
	if e := WritePartitionedSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	a, e := ReadPartitionedSplineLensAssembly(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Occurrences) != 2 || a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal("lost sharing")
	}
	pid := a.Assembly.Occurrences[0].Child
	if len(a.ProductBodies[pid]) != 1 || len(a.ProductPieces[pid]) != 6 {
		t.Fatal("lost product ancestry")
	}
	if _, e = ReadSplineLensAssembly(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("old reader broadened")
	}
	if _, e = ReadPartitionedSplineLensAssembly(bytes.NewReader(b.Bytes()), Limits{MaxEdgeUses: 11}, CurvedOvalOptions{}); e == nil {
		t.Fatal("edge budget")
	}
	m.Occurrences[0].Transform[0][0] = 2
	b.Reset()
	if e = WritePartitionedSplineLensAssembly(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("scaled instance accepted")
	}
}
func TestPartitionedLensAssemblyOCCT(t *testing.T) {
	source := t.TempDir() + "/source.step"
	stepOracle(t, source, "write-step-partitioned-spline-lens-assembly")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadPartitionedSplineLensAssembly(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Occurrences) != 2 || a.Assembly.Occurrences[0].Child != a.Assembly.Occurrences[1].Child {
		t.Fatal("independent source sharing")
	}
	pid := a.Assembly.Occurrences[0].Child
	if len(a.ProductPieces[pid]) != 6 {
		t.Fatal("source partition")
	}
	m := lensAssemblyFixture(t)
	m.Products[1].Bodies = a.ProductBodies[pid]
	m.LengthUnit = a.Assembly.LengthUnit
	var b bytes.Buffer
	if e = WritePartitionedSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	target := t.TempDir() + "/native.step"
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	output := t.TempDir() + "/mass.json"
	stepOracle(t, output, "step-mass-precise", target)
	raw, e = os.ReadFile(output)
	if e != nil {
		t.Fatal(e)
	}
	var mass struct{ Volume, Area float64 }
	if e = json.Unmarshal(raw, &mass); e != nil {
		t.Fatal(e)
	}
	r := 15. / 64
	if math.Abs(mass.Volume-math.Pi*math.Pow(r, 4)) > 1e-9 || math.Abs(mass.Area-8*math.Pi/3*(math.Pow(1+r*r, 1.5)-1)) > 1e-8 {
		t.Fatal(mass)
	}
}

func FuzzPartitionedLensAssembly(f *testing.F) {
	raw, e := os.ReadFile("testdata/partitioned-spline-lens-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadPartitionedSplineLensAssembly(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxEntities: 1024, MaxValues: 20000, MaxFaces: 4, MaxVertices: 512, MaxEdgeUses: 32, MaxDepth: 24}, CurvedOvalOptions{MaxOperations: 2000000, MaxCoefficientBits: 1024, MaxCells: 32})
	})
}

func TestPartitionedLensAssemblyCumulativeBudget(t *testing.T) {
	m := lensAssemblyFixture(t)
	m.Products[1].Bodies[0] = partitionedLensFixture(t)
	other := m.Products[1]
	other.ID = "other"
	m.Products = append(m.Products, other)
	m.Occurrences[1].Child = "other"
	var b bytes.Buffer
	if e := WritePartitionedSplineLensAssembly(&b, m); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadPartitionedSplineLensAssembly(bytes.NewReader(b.Bytes()), Limits{MaxEdgeUses: 23}, CurvedOvalOptions{}); e == nil {
		t.Fatal("per-product budget bypass")
	}
	if _, e := ReadPartitionedSplineLensAssembly(bytes.NewReader(b.Bytes()), Limits{MaxEdgeUses: 24}, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
}
