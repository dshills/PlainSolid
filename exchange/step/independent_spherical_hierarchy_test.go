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
	"reflect"
	"testing"
)

func sphericalHierarchyModel(t testing.TB) IndependentSphericalHierarchyModel {
	t.Helper()
	m := IndependentTorusAssemblyModel(independentAnalyticHierarchyModel(t))
	raw, e := os.ReadFile("testdata/occt-independent-sphere-product.step")
	if e != nil {
		t.Fatal(e)
	}
	source, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	m.Products = append(m.Products, IndependentTorusAssemblyPart{ID: "sphere", Name: "sphere", Source: source})
	for i, p := range [][3]float64{{-20, 10, 0}, {0, -20, 10}} {
		id := "sphere-a"
		if i == 1 {
			id = "sphere-b"
		}
		m.Occurrences = append(m.Occurrences, AssemblyInstance{ID: id, Parent: "root", Child: "sphere", Transform: geom.Affine{{1, 0, 0, p[0]}, {0, 1, 0, p[1]}, {0, 0, 1, p[2]}}})
	}
	return IndependentSphericalHierarchyModel(m)
}
func TestIndependentSphericalHierarchy(t *testing.T) {
	slowtest.Skip(t)
	raw, e := os.ReadFile("testdata/independent-spherical-hierarchy.step")
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	m, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Products) != 4 || len(m.Assembly.Products) != 7 || len(m.Instances) != 7 {
		t.Fatal("counts")
	}
	var sphereID int64
	for id, p := range m.Products {
		if p.Sphere != nil {
			sphereID = id
		}
	}
	if sphereID == 0 {
		t.Fatal("sphere")
	}
	uses := 0
	for _, u := range m.Instances {
		if u.ProductID == sphereID {
			uses++
			if len(u.Path) != 1 {
				t.Fatal("path")
			}
		}
	}
	if uses != 2 {
		t.Fatal("shared sphere")
	}
	if _, e = ReadIndependentAnalyticHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old reader widened")
	}
	var authored bytes.Buffer
	if e = WriteIndependentSphericalHierarchy(ctx, &authored, sphericalHierarchyModel(t), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(authored.Bytes()), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteIndependentSphericalHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadIndependentSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Instances, n.Instances) {
		t.Fatal("paths")
	}
	var stable bytes.Buffer
	if e = WriteIndependentSphericalHierarchyJSON(ctx, &stable, n); e != nil || !bytes.Equal(stable.Bytes(), native.Bytes()) {
		t.Fatal("stable", e)
	}
	if _, e = ReadIndependentAnalyticHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old native tag")
	}
	var source bytes.Buffer
	if e = WriteIndependentSphericalHierarchySource(ctx, &source, n.Assembly.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
		for _, b := range [][]byte{raw, authored.Bytes(), source.Bytes()} {
			file := filepath.Join(t.TempDir(), "sphere.step")
			if e = os.WriteFile(file, b, 0600); e != nil {
				t.Fatal(e)
			}
			data, e := exec.Command(oracle, "read", file).CombinedOutput()
			if e != nil {
				t.Fatal(e, string(data))
			}
			at := bytes.LastIndex(data, []byte("{"))
			var q struct {
				Area, Volume            float64
				Solids, Faces, EdgeUses int
				Bounds                  [6]float64
				SurfaceCenter           [3]float64
			}
			if at < 0 || json.Unmarshal(data[at:], &q) != nil {
				t.Fatal(string(data))
			}
			a := 52 + 24*math.Pi*math.Pi + 72*math.Pi
			v := 24 + 12*math.Pi*math.Pi + (24+64./3)*math.Pi
			if q.Solids != 7 || q.Faces != 22 || q.EdgeUses != 88 || math.Abs(q.Area-a) > 1e-7 || math.Abs(q.Volume-v) > 1e-7 {
				t.Fatal(q)
			}
			for i, x := range [6]float64{-21, -20, -2, 30, 30, 15} {
				if math.Abs(q.Bounds[i]-x) > 2e-7 {
					t.Fatal(q.Bounds)
				}
			}
			for i, x := range [3]float64{(-9*52 + 29*12*math.Pi*math.Pi + 34.5*20*math.Pi - 18*16*math.Pi) / a, (-2.5*52 + 23*12*math.Pi*math.Pi + 28*20*math.Pi - 6*16*math.Pi) / a, (10*12*math.Pi*math.Pi + 6.5*20*math.Pi + 16*16*math.Pi) / a} {
				if math.Abs(q.SurfaceCenter[i]-x) > 1e-7 {
					t.Fatal("moments", q.SurfaceCenter)
				}
			}
		}
	}
	// Sphere is optional only in this new mode; the original three-family
	// source still has its old proof and encodes under its unchanged old tag.
	base, _ := os.ReadFile("testdata/independent-analytic-hierarchy.step")
	noSphere, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(base), Limits{}, 1e-9)
	if e != nil || len(noSphere.Products) != 3 {
		t.Fatal("optional", e)
	}
	var opt bytes.Buffer
	if e = WriteIndependentSphericalHierarchyJSON(ctx, &opt, noSphere); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadIndependentSphericalHierarchyJSON(ctx, bytes.NewReader(opt.Bytes()), IndependentMaterialJSONLimits{}); e != nil {
		t.Fatal(e)
	}
}
func TestIndependentSphericalHierarchyReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/independent-spherical-hierarchy.step")
	m, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"radius", "pole", "owner"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := Parse(bytes.NewReader(raw), Limits{})
			var sphere, point, other int64
			for id, en := range f.Entities {
				if len(en.Records) != 1 {
					continue
				}
				r := en.Records[0]
				if r.Type == "SPHERICAL_SURFACE" {
					sphere = id
				}
				if r.Type == "VERTEX_LOOP" {
					v, _ := simple(f, r.Args[1].Ref, "VERTEX_POINT", 2)
					point = v.Args[1].Ref
				}
				if r.Type == "CYLINDRICAL_SURFACE" {
					other = r.Args[1].Ref
				}
			}
			switch kind {
			case "radius":
				en := f.Entities[sphere]
				en.Records[0].Args[2].Text = "3."
				f.Entities[sphere] = en
			case "pole":
				en := f.Entities[point]
				en.Records[0].Args[1].Items[2].Text = "2."
				f.Entities[point] = en
			case "owner":
				en := f.Entities[sphere]
				en.Records[0].Args[1].Ref = other
				f.Entities[sphere] = en
			}
			b, _ := mappedPlanarSourceBytes(f)
			if n, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(b), Limits{}, 1e-9); e == nil || n.Assembly.Source != nil {
				t.Fatal("accepted")
			}
		})
	}
	var native bytes.Buffer
	if e = WriteIndependentSphericalHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"tag", "path", "native", "sphereframe"} {
		t.Run(kind, func(t *testing.T) {
			var q independentAnalyticDAGEnvelope
			json.Unmarshal(native.Bytes(), &q)
			switch kind {
			case "tag":
				q.Format = independentAnalyticHierarchyNativeFormat
			case "path":
				q.Instances[0].Path[0]++
			case "native":
				q.Proof.Members[0].Native = json.RawMessage(`{}`)
			case "sphereframe":
				for i := range q.Proof.Members {
					var c map[string]json.RawMessage
					json.Unmarshal(q.Proof.Members[i].Claims, &c)
					if _, ok := c["SphereFrame"]; ok {
						c["SphereFrame"] = json.RawMessage(`[[1,0,0,99],[0,1,0,0],[0,0,1,0]]`)
						q.Proof.Members[i].Claims, _ = json.Marshal(c)
					}
				}
			}
			b, _ := json.Marshal(q)
			if n, e := ReadIndependentSphericalHierarchyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || n.Assembly.Source != nil {
				t.Fatal("accepted")
			}
		})
	}
	m.Instances[0].Transform[0][3]++
	var out bytes.Buffer
	if e = WriteIndependentSphericalHierarchyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale")
	}
	model := sphericalHierarchyModel(t)
	copy := model.Products[len(model.Products)-1]
	copy.ID = "extra-sphere"
	model.Products = append(model.Products, copy)
	model.Occurrences = append(model.Occurrences, AssemblyInstance{ID: "extra", Parent: "root", Child: "extra-sphere", Transform: geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}}})
	out.Reset()
	if e = WriteIndependentSphericalHierarchy(ctx, &out, model, Limits{}, 1e-9); e == nil || out.Len() != 0 {
		t.Fatal("duplicate family")
	}
}
func TestIndependentSphericalHierarchyLimits(t *testing.T) {
	raw, _ := os.ReadFile("testdata/independent-spherical-hierarchy.step")
	ctx := context.Background()
	for _, l := range []Limits{{MaxSolids: 3}, {MaxBytes: 32}, {MaxEntities: 16}, {MaxFaces: 1}} {
		if m, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(raw), l, 1e-9); e == nil || m.Assembly.Source != nil {
			t.Fatal("limit")
		}
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := ReadIndependentSphericalHierarchy(cancelCtx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
	m, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentSphericalHierarchyJSON(cancelCtx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("cancel write")
	}
	if e = WriteIndependentSphericalHierarchyJSON(ctx, &out, m); e != nil {
		t.Fatal(e)
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 32}, {Source: Limits{MaxSolids: 3}}} {
		if _, e = ReadIndependentSphericalHierarchyJSON(ctx, bytes.NewReader(out.Bytes()), l); e == nil {
			t.Fatal("native limits")
		}
	}
}
func FuzzIndependentSphericalHierarchy(f *testing.F) {
	raw, _ := os.ReadFile("testdata/independent-spherical-hierarchy.step")
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		m, e := ReadIndependentSphericalHierarchy(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}, 1e-9)
		if e != nil && m.Assembly.Source != nil {
			t.Fatal("partial")
		}
	})
}
func FuzzIndependentSphericalHierarchyNative(f *testing.F) {
	raw, _ := os.ReadFile("testdata/independent-spherical-hierarchy.step")
	m, e := ReadIndependentSphericalHierarchy(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteIndependentSphericalHierarchyJSON(context.Background(), &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		m, e := ReadIndependentSphericalHierarchyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}})
		if e != nil && m.Assembly.Source != nil {
			t.Fatal("partial")
		}
	})
}
