package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func independentAnalyticDAGModel(t testing.TB) IndependentAnalyticDAGAssemblyModel {
	t.Helper()
	m := independentAnalyticAssemblyModel(t)
	m.Products = append(m.Products, IndependentTorusAssemblyPart{ID: "group", Name: "analytic group"})
	m.Occurrences = []AssemblyInstance{
		{ID: "box", Parent: "root", Child: "box", Transform: geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}},
		{ID: "first-group", Parent: "root", Child: "group", Transform: geom.Affine{{1, 0, 0, 20}, {0, 1, 0, 0}, {0, 0, 1, 0}}},
		{ID: "second-group", Parent: "root", Child: "group", Transform: geom.Affine{{0, 0, 1, 0}, {1, 0, 0, 20}, {0, 1, 0, 0}}},
		{ID: "torus", Parent: "group", Child: "torus", Transform: geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}},
		{ID: "cylinder", Parent: "group", Child: "cylinder", Transform: geom.Affine{{1, 0, 0, 8}, {0, 1, 0, 0}, {0, 0, 1, 0}}},
	}
	return IndependentAnalyticDAGAssemblyModel(m)
}
func TestIndependentAnalyticDAGAssembly(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-analytic-dag-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadIndependentAnalyticDAGAssembly(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Products) != 3 || len(a.Assembly.Occurrences) != 5 || len(a.Instances) != 5 {
		t.Fatal("roles")
	}
	var out bytes.Buffer
	if e = WriteIndependentAnalyticDAGAssembly(ctx, &out, independentAnalyticDAGModel(t), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentAnalyticDAGAssembly(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteIndependentAnalyticDAGAssemblyJSON(ctx, &native, a); e != nil {
		t.Fatal(e)
	}
	n, e := ReadIndependentAnalyticDAGAssemblyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteIndependentAnalyticDAGAssemblyJSON(ctx, &stable, n); e != nil || !bytes.Equal(stable.Bytes(), native.Bytes()) {
		t.Fatal("native stability", e)
	}
	var restored bytes.Buffer
	if e = WriteIndependentAnalyticDAGAssemblySource(ctx, &restored, n.Assembly.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	x, _ := json.Marshal(a.Instances)
	y, _ := json.Marshal(n.Instances)
	if !bytes.Equal(x, y) {
		t.Fatal("path ancestry")
	}
	uses := map[int64]int{}
	for _, u := range n.Instances {
		uses[u.ProductID]++
		if len(u.Path) == 2 {
			p := n.Products[u.ProductID]
			center := geom.Vec3{}
			if p.Torus != nil {
				center = u.Transform.Apply(p.Torus.Frame.Origin())
				if center != (geom.Vec3{X: 21, Y: 2, Z: 3}) && center != (geom.Vec3{X: 3, Y: 21, Z: 2}) {
					t.Fatal("torus world center", center)
				}
			} else if p.Cylinder != nil {
				center = u.Transform.Apply(geom.Vec3{Z: 1.5})
				if center != (geom.Vec3{X: 28, Z: 1.5}) && center != (geom.Vec3{X: 1.5, Y: 28}) {
					t.Fatal("cylinder world center", center)
				}
			}
		}
	}
	for id, p := range n.Products {
		count := 2
		if p.Planar != nil {
			count = 1
		}
		if uses[id] != count || p.SourceBodyID != a.Products[id].SourceBodyID {
			t.Fatal("shared source bodies")
		}
	}
	if _, e = ReadIndependentAnalyticAssembly(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("flat widened")
	}
	if _, e = ReadIndependentTorusAssemblyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("torus native tag")
	}
	if _, e = ReadIndependentAnalyticAssemblyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("flat native tag")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
		for _, b := range [][]byte{raw, out.Bytes(), restored.Bytes()} {
			file := filepath.Join(t.TempDir(), "dag.step")
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
			for i, want := range [6]float64{-10, -4, -2, 30, 30, 6} {
				if math.Abs(q.Bounds[i]-want) > 2e-7 {
					t.Fatal(q.Bounds)
				}
			}
			for i, want := range [3]float64{(-9*52 + 24*12*math.Pi*math.Pi + 29.5*20*math.Pi) / area, (-2.5*52 + 23*12*math.Pi*math.Pi + 28*20*math.Pi) / area, (5*12*math.Pi*math.Pi + 1.5*20*math.Pi) / area} {
				if math.Abs(q.SurfaceCenter[i]-want) > 1e-7 {
					t.Fatal("centroid", q.SurfaceCenter)
				}
			}
		}
	}

}

func TestIndependentAnalyticDAGReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-analytic-dag-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadIndependentAnalyticDAGAssembly(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var group, planar, torus, cylinder int64
	for _, p := range a.Assembly.Products {
		if p.DefinitionID != a.Assembly.Roots[0] && a.Products[p.DefinitionID].SourceBodyID == 0 {
			group = p.DefinitionID
		}
	}
	for id, p := range a.Products {
		if p.Planar != nil {
			planar = id
		}
		if p.Torus != nil {
			torus = id
		}
		if p.Cylinder != nil {
			cylinder = id
		}
	}
	for _, kind := range []string{"cycle", "missing-reuse", "wrong-parent", "group-shape", "borrow", "orphan"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		switch kind {
		case "cycle", "wrong-parent", "missing-reuse":
			for _, u := range a.Assembly.Occurrences {
				if kind == "missing-reuse" && u.Child == group {
					en := f.Entities[u.ID]
					en.Records[0].Args[4].Ref = planar
					f.Entities[u.ID] = en
					break
				}
				if kind != "missing-reuse" && u.Parent == group {
					en := f.Entities[u.ID]
					if kind == "cycle" {
						en.Records[0].Args[4].Ref = a.Assembly.Roots[0]
					} else {
						en.Records[0].Args[3].Ref = a.Assembly.Roots[0]
					}
					f.Entities[u.ID] = en
					break
				}
			}
		case "group-shape":
			for _, p := range a.Assembly.Products {
				if p.DefinitionID == group {
					en := f.Entities[p.Representations[0]]
					en.Records[0].Type = "ADVANCED_BREP_SHAPE_REPRESENTATION"
					en.Records[0].Args[1].Items = append(en.Records[0].Args[1].Items, Value{Kind: Reference, Ref: a.Products[torus].SourceBodyID})
					f.Entities[p.Representations[0]] = en
				}
			}
		case "borrow":
			id := a.Products[cylinder].Cylinder.SourceEdges[0]
			en := f.Entities[id]
			en.Records[0].Args[3].Ref = a.Products[torus].Torus.SourceCircleIDs[0]
			f.Entities[id] = en
		case "orphan":
			id := f.Order[len(f.Order)-1] + 1
			en := f.Entities[a.Products[torus].SourceBodyID]
			en.ID = id
			f.Entities[id] = en
			f.Order = append(f.Order, id)
		}
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		if got, e := ReadIndependentAnalyticDAGAssembly(ctx, bytes.NewReader(b), Limits{}, 1e-9); e == nil || len(got.Products) != 0 || len(got.Instances) != 0 {
			t.Fatal(kind)
		}
	}
	m := independentAnalyticDAGModel(t)
	// Each source translation is exactly represented; their composed translation
	// 2^54+1 is not. Neither the emitter nor native proof may round that path.
	m.Occurrences[1].Transform[0][3] = 18014398509481984
	m.Occurrences[4].Transform[0][3] = 1
	var bad bytes.Buffer
	if e = WriteIndependentAnalyticDAGAssembly(ctx, &bad, m, Limits{}, 1e-9); e == nil || bad.Len() != 0 {
		t.Fatal("composed translation rounded")
	}
	var native bytes.Buffer
	if e = WriteIndependentAnalyticDAGAssemblyJSON(ctx, &native, a); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"tag", "inner-tag", "version", "path", "pose", "leaf", "edge", "source", "native", "root-use"} {
		var q independentAnalyticDAGEnvelope
		if json.Unmarshal(native.Bytes(), &q) != nil {
			t.Fatal("json")
		}
		switch kind {
		case "tag":
			q.Format = independentAnalyticAssemblyNativeFormat
		case "inner-tag":
			q.Proof.Format = independentAnalyticAssemblyNativeFormat
		case "version":
			q.Version++
		case "path":
			q.Instances[1].Path[0] = q.Instances[0].Path[0]
		case "pose":
			q.Instances[1].Transform[0][3]++
		case "leaf":
			q.Instances[1].ProductID = planar
		case "edge":
			q.Proof.Members[0].Claims = json.RawMessage(`{}`)
		case "source":
			q.Proof.Source = q.Proof.Source[:len(q.Proof.Source)/2]
		case "native":
			q.Proof.Members[0].Native = json.RawMessage(`{}`)
		case "root-use":
			q.Proof.Assembly.Occurrences[0].Parent = group
		}
		b, _ := json.Marshal(q)
		if got, e := ReadIndependentAnalyticDAGAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || len(got.Products) != 0 || len(got.Instances) != 0 {
			t.Fatal(kind)
		}
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 1}, {Source: Limits{MaxSolids: 4}}, {Source: Limits{MaxFaces: 12}}, {Source: Limits{MaxDepth: 2}}} {
		if _, e := ReadIndependentAnalyticDAGAssemblyJSON(ctx, bytes.NewReader(native.Bytes()), l); e == nil {
			t.Fatal("limit")
		}
	}
	cancelled, c := context.WithCancel(ctx)
	c()
	if _, e := ReadIndependentAnalyticDAGAssemblyJSON(cancelled, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	a.Instances[1].Transform[0][3]++
	if e = WriteIndependentAnalyticDAGAssemblyJSON(ctx, &bad, a); e == nil || bad.Len() != 0 {
		t.Fatal("atomic stale path")
	}
	flat, e := os.ReadFile("testdata/independent-analytic-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentAnalyticDAGAssembly(ctx, bytes.NewReader(flat), Limits{}, 1e-9); e == nil {
		t.Fatal("flat promoted")
	}
}

func FuzzIndependentAnalyticDAGAssembly(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-analytic-dag-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		m, e := ReadIndependentAnalyticDAGAssembly(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 2048, MaxValues: 32768, MaxSolids: 5, MaxFaces: 24, MaxVertices: 100, MaxEdgeUses: 150, MaxDepth: 16}, 1e-9)
		if e != nil && (len(m.Products) != 0 || len(m.Instances) != 0) {
			t.Fatal("partial")
		}
	})
}
func FuzzIndependentAnalyticDAGAssemblyNative(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-analytic-dag-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	ctx := context.Background()
	m, e := ReadIndependentAnalyticDAGAssembly(ctx, bytes.NewReader(b), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentAnalyticDAGAssemblyJSON(ctx, &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 2<<20 {
			return
		}
		m, e := ReadIndependentAnalyticDAGAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 2 << 20, Source: Limits{MaxEntities: 2048, MaxValues: 32768, MaxSolids: 5, MaxFaces: 24, MaxVertices: 100, MaxEdgeUses: 150, MaxDepth: 16}})
		if e != nil && (len(m.Products) != 0 || len(m.Instances) != 0) {
			t.Fatal("partial")
		}
	})
}
