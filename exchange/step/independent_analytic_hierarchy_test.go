package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func independentAnalyticHierarchyModel(t testing.TB) IndependentAnalyticHierarchyModel {
	t.Helper()
	m := independentAnalyticDAGModel(t)
	m.Products = append(m.Products, IndependentTorusAssemblyPart{ID: "carrier", Name: "carrier"})
	for i := range m.Occurrences {
		if m.Occurrences[i].Parent == "root" && m.Occurrences[i].Child == "group" {
			m.Occurrences[i].Child = "carrier"
		}
	}
	m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: "carrier-group", Parent: "carrier", Child: "group", Transform: geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 5}}})
	return IndependentAnalyticHierarchyModel(m)
}
func TestIndependentAnalyticHierarchy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-analytic-hierarchy.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Assembly.Products) != 6 || len(a.Assembly.Occurrences) != 6 || len(a.Products) != 3 || len(a.Instances) != 5 {
		t.Fatal("counts")
	}
	for _, u := range a.Instances {
		p := a.Products[u.ProductID]
		if p.Planar != nil {
			if len(u.Path) != 1 {
				t.Fatal("planar path")
			}
		} else if len(u.Path) != 3 {
			t.Fatal("three levels")
		}
	}
	var authored bytes.Buffer
	if e = WriteIndependentAnalyticHierarchy(ctx, &authored, independentAnalyticHierarchyModel(t), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(authored.Bytes()), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteIndependentAnalyticHierarchyJSON(ctx, &native, a); e != nil {
		t.Fatal(e)
	}
	n, e := ReadIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteIndependentAnalyticHierarchyJSON(ctx, &stable, n); e != nil || !bytes.Equal(stable.Bytes(), native.Bytes()) {
		t.Fatal("stable", e)
	}
	x, _ := json.Marshal(a.Instances)
	y, _ := json.Marshal(n.Instances)
	if !bytes.Equal(x, y) {
		t.Fatal("ancestry")
	}
	var source bytes.Buffer
	if e = WriteIndependentAnalyticHierarchySource(ctx, &source, n.Assembly.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentAnalyticDAGAssembly(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("narrow widened")
	}
	if _, e = ReadIndependentAnalyticDAGAssemblyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old tag")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
		for _, b := range [][]byte{raw, authored.Bytes(), source.Bytes()} {
			file := filepath.Join(t.TempDir(), "hierarchy.step")
			if e = os.WriteFile(file, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", file).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var q struct {
				Volume, Area            float64
				Solids, Faces, EdgeUses int
				Bounds                  [6]float64
				SurfaceCenter           [3]float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil {
				t.Fatal(string(data))
			}
			area := 52 + 24*math.Pi*math.Pi + 40*math.Pi
			volume := 24 + 12*math.Pi*math.Pi + 24*math.Pi
			if q.Solids != 5 || q.Faces != 20 || q.EdgeUses != 80 || math.Abs(q.Volume-volume) > 1e-7 || math.Abs(q.Area-area) > 1e-7 {
				t.Fatal(q)
			}
			for i, want := range [6]float64{-10, -4, -2, 30, 30, 9} {
				if math.Abs(q.Bounds[i]-want) > 2e-7 {
					t.Fatal(q.Bounds)
				}
			}
			for i, want := range [3]float64{(-9*52 + 29*12*math.Pi*math.Pi + 34.5*20*math.Pi) / area, (-2.5*52 + 23*12*math.Pi*math.Pi + 28*20*math.Pi) / area, (10*12*math.Pi*math.Pi + 6.5*20*math.Pi) / area} {
				if math.Abs(q.SurfaceCenter[i]-want) > 1e-7 {
					t.Fatal("centroid", q.SurfaceCenter)
				}
			}
		}
	}
}

func hierarchyChain(m IndependentAnalyticHierarchyModel, n int) IndependentAnalyticHierarchyModel {
	for i := range m.Occurrences {
		if m.Occurrences[i].ID == "carrier-group" {
			m.Occurrences[i].Child = "chain0"
		}
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("chain%d", i)
		child := "group"
		if i+1 < n {
			child = fmt.Sprintf("chain%d", i+1)
		}
		m.Products = append(m.Products, IndependentTorusAssemblyPart{ID: id, Name: id})
		m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: id + "-use", Parent: id, Child: child, Transform: geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}})
	}
	return m
}
func TestIndependentAnalyticHierarchyBounds(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	m := hierarchyChain(independentAnalyticHierarchyModel(t), 5)
	var out bytes.Buffer
	if e := WriteIndependentAnalyticHierarchy(ctx, &out, m, Limits{}, 1e-9); e != nil {
		t.Fatal("depth8", e)
	}
	a, e := ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	depth := 0
	for _, u := range a.Instances {
		depth = max(depth, len(u.Path))
	}
	if depth != 8 {
		t.Fatal(depth)
	}
	var bad bytes.Buffer
	if e = WriteIndependentAnalyticHierarchy(ctx, &bad, hierarchyChain(independentAnalyticHierarchyModel(t), 6), Limits{}, 1e-9); e == nil || bad.Len() != 0 {
		t.Fatal("depth9")
	}
	base := independentAnalyticDAGModel(t)
	base.Occurrences = base.Occurrences[:1]
	identity := geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}
	for i := 0; i < 31; i++ {
		base.Occurrences = append(base.Occurrences, AssemblyInstance{ID: fmt.Sprintf("group%d", i), Parent: "root", Child: "group", Transform: identity})
	}
	base.Occurrences = append(base.Occurrences, AssemblyInstance{ID: "group-torus", Parent: "group", Child: "torus", Transform: identity}, AssemblyInstance{ID: "group-cylinder", Parent: "group", Child: "cylinder", Transform: identity}, AssemblyInstance{ID: "extra", Parent: "root", Child: "torus", Transform: identity})
	m = IndependentAnalyticHierarchyModel(base)
	out.Reset()
	if e = WriteIndependentAnalyticHierarchy(ctx, &out, m, Limits{}, 1e-9); e != nil {
		t.Fatal("64 leaves", e)
	}
	a, e = ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil || len(a.Instances) != 64 || len(a.Assembly.Occurrences) != 35 {
		t.Fatal("64 bound", e)
	}
	m.Occurrences[len(m.Occurrences)-1].Child = "group"
	if e = WriteIndependentAnalyticHierarchy(ctx, &bad, m, Limits{}, 1e-9); e == nil || bad.Len() != 0 {
		t.Fatal("65 expanded leaves")
	}
	for _, kind := range []string{"cycle", "unreachable", "material-parent"} {
		m := independentAnalyticHierarchyModel(t)
		switch kind {
		case "cycle":
			m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: "loop", Parent: "group", Child: "carrier", Transform: identity})
		case "unreachable":
			m.Products = append(m.Products, IndependentTorusAssemblyPart{ID: "unused"})
		case "material-parent":
			m.Occurrences[len(m.Occurrences)-1].Parent = "cylinder"
		}
		if e = WriteIndependentAnalyticHierarchy(ctx, &bad, m, Limits{}, 1e-9); e == nil || bad.Len() != 0 {
			t.Fatal(kind)
		}
	}
}
func TestIndependentAnalyticHierarchyNativeReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-analytic-hierarchy.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteIndependentAnalyticHierarchyJSON(ctx, &native, a); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"tag", "inner-tag", "path", "depth", "pose", "owner", "native", "source", "too-many"} {
		var q independentAnalyticDAGEnvelope
		if json.Unmarshal(native.Bytes(), &q) != nil {
			t.Fatal("json")
		}
		switch kind {
		case "tag":
			q.Format = independentAnalyticDAGNativeFormat
		case "inner-tag":
			q.Proof.Format = independentAnalyticDAGClaimsFormat
		case "path":
			q.Instances[1].Path[1]++
		case "depth":
			q.Instances[1].Path = make([]int64, 9)
		case "pose":
			q.Instances[1].Transform[0][3]++
		case "owner":
			q.Instances[1].ProductID = q.Instances[0].ProductID
		case "native":
			q.Proof.Members[0].Native = json.RawMessage(`{}`)
		case "source":
			q.Proof.Source = q.Proof.Source[:len(q.Proof.Source)/2]
		case "too-many":
			for len(q.Instances) < 65 {
				q.Instances = append(q.Instances, q.Instances[0])
			}
		}
		b, _ := json.Marshal(q)
		if got, e := ReadIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || len(got.Products) != 0 || len(got.Instances) != 0 {
			t.Fatal(kind)
		}
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 1}, {Source: Limits{MaxSolids: 5}}, {Source: Limits{MaxDepth: 2}}, {Source: Limits{MaxEntities: 16}}} {
		if _, e := ReadIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), l); e == nil {
			t.Fatal("limit")
		}
	}
	cancelled, c := context.WithCancel(ctx)
	c()
	if _, e := ReadIndependentAnalyticHierarchyJSON(cancelled, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	a.Instances[1].Path[1]++
	var bad bytes.Buffer
	if e = WriteIndependentAnalyticHierarchyJSON(ctx, &bad, a); e == nil || bad.Len() != 0 {
		t.Fatal("atomic stale")
	}
}
func FuzzIndependentAnalyticHierarchy(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-analytic-hierarchy.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		m, e := ReadIndependentAnalyticHierarchy(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 2048, MaxValues: 32768, MaxSolids: 16, MaxFaces: 24, MaxVertices: 100, MaxEdgeUses: 150, MaxDepth: 16}, 1e-9)
		if e != nil && (len(m.Products) != 0 || len(m.Instances) != 0) {
			t.Fatal("partial")
		}
	})
}
func FuzzIndependentAnalyticHierarchyNative(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-analytic-hierarchy.step")
	if e != nil {
		f.Fatal(e)
	}
	ctx := context.Background()
	m, e := ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(b), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentAnalyticHierarchyJSON(ctx, &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 2<<20 {
			return
		}
		m, e := ReadIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 2 << 20, Source: Limits{MaxEntities: 2048, MaxValues: 32768, MaxSolids: 16, MaxFaces: 24, MaxVertices: 100, MaxEdgeUses: 150, MaxDepth: 16}})
		if e != nil && (len(m.Products) != 0 || len(m.Instances) != 0) {
			t.Fatal("partial")
		}
	})
}
