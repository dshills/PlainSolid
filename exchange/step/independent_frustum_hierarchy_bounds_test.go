package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestIndependentFrustumHierarchyBounds(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	var source bytes.Buffer
	if e := WriteIndependentFrustumHierarchy(ctx, &source, IndependentFrustumHierarchyModel(frustumHierarchyModel(t)), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(source.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteIndependentFrustumHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadIndependentFrustumHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundIndependentFrustumHierarchy(ctx, m, FrustumHierarchyBoundsOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Leaves) != 9 || r.SquareRoots != 0 || r.FractionBits != 0 {
		t.Fatal("counts")
	}
	fr := 0
	for _, leaf := range r.Leaves {
		if len(leaf.SourceFaceIDs) == 0 || len(leaf.Path) == 0 || leaf.ProductID == 0 || leaf.SourceBodyID == 0 {
			t.Fatal("lost ancestry")
		}
		for _, p := range leaf.Bounds.MaximumAxisPadding() {
			if p.Sign() != 0 {
				t.Fatal("nonexact bounds")
			}
		}
		if leaf.Family != "frustum" {
			continue
		}
		fr++
		lo, hi := leaf.Bounds.Min(), leaf.Bounds.Max()
		expected := [6]int64{28, 18, -2, 32, 22, 3}
		if lo[0].Sign() < 0 {
			expected = [6]int64{-32, 18, -2, -27, 22, 2}
		}
		for i := 0; i < 3; i++ {
			if lo[i].Cmp(big.NewRat(expected[i], 1)) != 0 || hi[i].Cmp(big.NewRat(expected[i+3], 1)) != 0 {
				t.Fatal("frustum extrema", lo, hi)
			}
		}
		lo[0].SetInt64(999)
		if leaf.Bounds.Min()[0].Cmp(big.NewRat(999, 1)) == 0 {
			t.Fatal("mutable observation")
		}
	}
	if fr != 2 {
		t.Fatal("frustum uses")
	}
	expected := [6]int64{-32, -20, -2, 32, 30, 15}
	lo, hi := r.Aggregate.Min(), r.Aggregate.Max()
	for i := 0; i < 3; i++ {
		if lo[i].Cmp(big.NewRat(expected[i], 1)) != 0 || hi[i].Cmp(big.NewRat(expected[i+3], 1)) != 0 {
			t.Fatal("aggregate", lo, hi)
		}
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_FRUSTUM_HIERARCHY"); bin != "" {
		file := filepath.Join(t.TempDir(), "bounds.step")
		if e = os.WriteFile(file, source.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		data, e := exec.Command(bin, "read", file).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(data))
		}
		at := bytes.LastIndex(data, []byte("{"))
		var q struct{ Bounds [6]float64 }
		if at < 0 || json.Unmarshal(data[at:], &q) != nil {
			t.Fatal(string(data))
		}
		for i, want := range expected {
			if math.Abs(q.Bounds[i]-float64(want)) > 1e-6 {
				t.Fatal("oracle bounds", q.Bounds)
			}
		}
	}
	for _, o := range []FrustumHierarchyBoundsOptions{{MaxPointEvaluations: 1}, {Source: Limits{MaxSolids: 4}}} {
		q, e := BoundIndependentFrustumHierarchy(ctx, m, o)
		if e == nil || len(q.Leaves) != 0 {
			t.Fatal("partial quota result")
		}
	}
	cancel, c := context.WithCancel(ctx)
	c()
	if q, e := BoundIndependentFrustumHierarchy(cancel, m, FrustumHierarchyBoundsOptions{}); e == nil || len(q.Leaves) != 0 {
		t.Fatal("cancel")
	}
	m.Instances[0].Path[0]++
	if q, e := BoundIndependentFrustumHierarchy(ctx, m, FrustumHierarchyBoundsOptions{}); e == nil || len(q.Leaves) != 0 {
		t.Fatal("forged path")
	}
}

func TestIndependentFrustumHierarchyBoundsNoncircular(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	model := frustumHierarchyModel(t)
	raw, e := os.ReadFile("testdata/wrapped-frustum.step")
	if e != nil {
		t.Fatal(e)
	}
	source, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range model.Products {
		if model.Products[i].ID == "frustum" {
			model.Products[i].Source = source
		}
	}
	var b bytes.Buffer
	if e = WriteIndependentFrustumHierarchy(ctx, &b, IndependentFrustumHierarchyModel(model), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range m.Products {
		if p.Frustum != nil && p.Frustum.Embedding.Circular {
			t.Fatal("not noncircular")
		}
	}
	r, e := BoundIndependentFrustumHierarchy(ctx, m, FrustumHierarchyBoundsOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if r.Aggregate.Min()[0].Cmp(big.NewRat(-32, 1)) != 0 || r.Aggregate.Max()[0].Cmp(big.NewRat(32, 1)) != 0 {
		t.Fatal("profile weights changed exact endpoint extrema")
	}
}
