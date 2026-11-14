package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestIndependentTorusAssembly(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-torus-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadIndependentTorusAssembly(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(a.Products) != 2 || len(a.Assembly.Occurrences) != 3 {
		t.Fatal("roles")
	}
	for _, p := range a.Products {
		var b bytes.Buffer
		if e = trimmed.WriteJSON(&b, p.Solid); e != nil {
			t.Fatal(e)
		}
		if _, e = trimmed.ReadJSON(&b, trimmed.JSONLimits{}); e != nil {
			t.Fatal(e)
		}
	}
}

func independentTorusAssemblyModel(t testing.TB) IndependentTorusAssemblyModel {
	t.Helper()
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/occt-independent-exact-materials.step")
	if e != nil {
		t.Fatal(e)
	}
	planar, e := ReadIndependentExactMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	raw, e = os.ReadFile("testdata/independent-model-torus.step")
	if e != nil {
		t.Fatal(e)
	}
	torus, e := ReadIndependentToroidalMaterials(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	return IndependentTorusAssemblyModel{LengthUnit: .001, Products: []IndependentTorusAssemblyPart{{ID: "root", Name: "root"}, {ID: "box", Name: "box", Source: planar.Members[0].Source}, {ID: "torus", Name: "torus", Source: torus.Members[0].Source}}, Occurrences: []AssemblyInstance{
		{ID: "box", Parent: "root", Child: "box", Transform: geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}},
		{ID: "first", Parent: "root", Child: "torus", Transform: geom.Affine{{1, 0, 0, 20}, {0, 1, 0, 0}, {0, 0, 1, 0}}},
		{ID: "second", Parent: "root", Child: "torus", Transform: geom.Affine{{0, 0, 1, 0}, {1, 0, 0, 20}, {0, 1, 0, 0}}}}}
}
func TestIndependentTorusAssemblyWriter(t *testing.T) {
	ctx := context.Background()
	m := independentTorusAssemblyModel(t)
	var out bytes.Buffer
	if e := WriteIndependentTorusAssembly(ctx, &out, m, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	a, e := ReadIndependentTorusAssembly(ctx, bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	uses := map[int64]int{}
	for _, u := range a.Assembly.Occurrences {
		uses[u.Child]++
	}
	for id, p := range a.Products {
		if p.SourceRootID == 0 || p.SourceProductID == 0 {
			t.Fatal("ancestry")
		}
		if p.Torus != nil {
			if uses[id] != 2 || p.Torus.Frame.Origin() != (geom.Vec3{X: 1, Y: 2, Z: 3}) {
				t.Fatal("shared local product")
			}
		}
	}
	oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY")
	if oracle != "" {
		source, e := os.ReadFile("testdata/independent-torus-assembly.step")
		if e != nil {
			t.Fatal(e)
		}
		for i, raw := range [][]byte{source, out.Bytes()} {
			p := filepath.Join(t.TempDir(), "assembly.step")
			if e = os.WriteFile(p, raw, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", p).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var q struct {
				Volume, Area  float64
				Solids, Faces int
				SurfaceCenter [3]float64
				Bounds        [6]float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil {
				t.Fatal(string(data))
			}
			v := 24 + 12*math.Pi*math.Pi
			area := 52 + 24*math.Pi*math.Pi
			if q.Solids != 3 || q.Faces != 8 || math.Abs(q.Volume-v) > 1e-8 || math.Abs(q.Area-area) > 1e-8 {
				t.Fatal(i, q)
			}
			for k, want := range [6]float64{-10, -4, -2, 25, 25, 6} {
				if math.Abs(q.Bounds[k]-want) > 2e-7 {
					t.Fatal("world bounds", q.Bounds)
				}
			}
			// Surface first moment independently combines box center and the two posed torus centers.
			for k, want := range [3]float64{(-9*52 + 24*12*math.Pi*math.Pi) / area, (-2.5*52 + 23*12*math.Pi*math.Pi) / area, (0*52 + 5*12*math.Pi*math.Pi) / area} {
				if math.Abs(q.SurfaceCenter[k]-want) > 1e-8 {
					t.Fatal("pose", i, q.SurfaceCenter)
				}
			}
		}
	}
	m.Occurrences[1].Transform[0][0] = .9999999999999999
	var bad bytes.Buffer
	if e := WriteIndependentTorusAssembly(ctx, &bad, m, Limits{}, 1e-9); e == nil || bad.Len() != 0 {
		t.Fatal("atomic pose rejection")
	}
}

func TestIndependentTorusAssemblyReject(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-torus-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"orphan", "circle", "seam", "pose", "nonbinary-pose", "composition", "units", "borrow", "duplicate-leaf", "nested", "cycle", "pcurve", "extra-body", "underflow"} {
		f, e := Parse(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		a, e := ReadIndependentTorusAssembly(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		var torus IndependentExactMaterial
		for _, p := range a.Products {
			if p.Torus != nil {
				torus = p
			}
		}
		id := int64(0)
		arg := 0
		switch kind {
		case "orphan", "extra-body":
			id = f.Order[len(f.Order)-1] + 1
			en := f.Entities[torus.Torus.SourcePointID]
			if kind == "extra-body" {
				en = f.Entities[torus.SourceBodyID]
			}
			en.ID = id
			f.Order = append(f.Order, id)
			f.Entities[id] = en
		case "circle":
			id = torus.Torus.SourceCircleIDs[0]
			arg = 2
		case "seam":
			id = torus.Torus.SourceUseIDs[0]
			arg = 4
		case "borrow":
			// A torus point borrowed from the independent planar product is invalid even
			// when it happens to have a representable coordinate.
			var point int64
			for _, p := range a.Products {
				if p.Planar != nil {
					for _, x := range p.Source.Order {
						if p.Source.Entities[x].Records[0].Type == "CARTESIAN_POINT" {
							point = x
							break
						}
					}
				}
			}
			id = torus.Torus.SourceVertexID
			en := f.Entities[id]
			en.Records[0].Args[1].Ref = point
			f.Entities[id] = en
		case "duplicate-leaf":
			id = torus.SourceRepresentationID
			en := f.Entities[id]
			for _, p := range a.Products {
				if p.Planar != nil {
					en.Records[0].Args[1].Items[1].Ref = p.SourceBodyID
				}
			}
			f.Entities[id] = en
		case "nested", "cycle":
			id = a.Assembly.Occurrences[0].ID
			en := f.Entities[id]
			en.Records[0].Args[3].Ref = torus.SourceDefinitionID
			if kind == "cycle" {
				en.Records[0].Args[4].Ref = a.Assembly.Roots[0]
			}
			f.Entities[id] = en
		case "composition":
			for _, x := range f.Order {
				en := f.Entities[x]
				if len(en.Records) == 1 && en.Records[0].Type == "ITEM_DEFINED_TRANSFORMATION" {
					from := f.Entities[en.Records[0].Args[2].Ref].Records[0].Args[1].Ref
					to := f.Entities[en.Records[0].Args[3].Ref].Records[0].Args[1].Ref
					point := f.Entities[from]
					point.Records[0].Args[1].Items[0].Text = "1."
					f.Entities[from] = point
					point = f.Entities[to]
					point.Records[0].Args[1].Items[0].Text = "18014398509481984."
					f.Entities[to] = point
					id = to
					break
				}
			}
		case "pcurve":
			id = torus.Torus.SourceCircleIDs[0]
			en := f.Entities[id]
			en.Records[0].Type = "SEAM_CURVE"
			f.Entities[id] = en
		case "units":
			for _, x := range f.Order {
				en := f.Entities[x]
				for i, r := range en.Records {
					if r.Type == "SI_UNIT" && len(r.Args) == 2 && r.Args[1].Text == "METRE" {
						en.Records[i].Args[0].Text = "CENTI"
						f.Entities[x] = en
						id = x
					}
				}
			}
		default:
			for _, x := range f.Order {
				en := f.Entities[x]
				if len(en.Records) == 1 && en.Records[0].Type == "ITEM_DEFINED_TRANSFORMATION" {
					frame := f.Entities[en.Records[0].Args[3].Ref].Records[0]
					id = frame.Args[1].Ref
					if kind == "pose" {
						id = frame.Args[3].Ref
					}
					break
				}
			}
			en := f.Entities[id]
			v := "0.1"
			if kind == "pose" {
				v = "0.0000000000000001"
			}
			if kind == "underflow" {
				v = "1.E-999999999"
			}
			en.Records[0].Args[1].Items[1].Text = v
			f.Entities[id] = en
		}
		if kind == "circle" || kind == "seam" {
			en := f.Entities[id]
			v := "3."
			if kind == "seam" {
				v = "T"
			}
			en.Records[0].Args[arg].Text = v
			f.Entities[id] = en
		}
		if id == 0 {
			t.Fatal("mutation", kind)
		}
		b, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		if got, e := ReadIndependentTorusAssembly(ctx, bytes.NewReader(b), Limits{}, 1e-9); e == nil || len(got.Products) != 0 {
			t.Fatal("accepted", kind)
		}
	}
	for _, l := range []Limits{{MaxBytes: 32}, {MaxEntities: 10}, {MaxSolids: 2}, {MaxFaces: 6}, {MaxVertices: 2}, {MaxDepth: 2}} {
		if _, e := ReadIndependentTorusAssembly(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("limits", l)
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadIndependentTorusAssembly(c, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	m := independentTorusAssemblyModel(t)
	vendor, e := os.ReadFile("testdata/occt-independent-unsupported-torus.step")
	if e != nil {
		t.Fatal(e)
	}
	m.Products[2].Source, e = Parse(bytes.NewReader(vendor), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentTorusAssembly(ctx, &out, m, Limits{}, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("rounded vendor")
	}
}
func FuzzIndependentTorusAssembly(f *testing.F) {
	b, e := os.ReadFile("testdata/independent-torus-assembly.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(b)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		m, e := ReadIndependentTorusAssembly(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 1024, MaxValues: 16384, MaxSolids: 3, MaxFaces: 20, MaxVertices: 100, MaxEdgeUses: 100, MaxDepth: 16}, 1e-9)
		if e != nil && len(m.Products) != 0 {
			t.Fatal("partial")
		}
	})
}

func TestIndependentTorusAssemblySource(t *testing.T) {
	ctx := context.Background()
	raw, e := os.ReadFile("testdata/independent-torus-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	a, e := ReadIndependentTorusAssembly(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	// Arbitrary source record order has no geometric meaning.
	for i, j := 0, len(a.Assembly.Source.Order)-1; i < j; i, j = i+1, j-1 {
		a.Assembly.Source.Order[i], a.Assembly.Source.Order[j] = a.Assembly.Source.Order[j], a.Assembly.Source.Order[i]
	}
	var b bytes.Buffer
	if e = WriteIndependentTorusAssemblySource(ctx, &b, a.Assembly.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	n, e := ReadIndependentTorusAssembly(ctx, bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for id, p := range a.Products {
		q := n.Products[id]
		if p.SourceRootID != q.SourceRootID || p.SourceBodyID != q.SourceBodyID {
			t.Fatal("changed source identity")
		}
	}
	for id, p := range a.Products {
		if p.Torus != nil {
			x, _ := json.Marshal(p.Torus)
			y, _ := json.Marshal(n.Products[id].Torus)
			if !bytes.Equal(x, y) {
				t.Fatal("changed seam ancestry")
			}
		}
	}
}
