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
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestRationalSphericalHierarchy(t *testing.T) {
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	ctx := context.Background()
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Products) != 4 || len(m.Instances) != 7 || len(m.Relationships) != 8 {
		t.Fatal("counts")
	}
	if _, e = ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("old widened")
	}
	found := false
	for _, u := range m.Instances {
		mat := u.Matrix()
		if mat[0][0].Cmp(big.NewRat(3, 5)) == 0 {
			found = true
			mat[0][0].SetInt64(99)
			if u.Matrix()[0][0].Cmp(big.NewRat(3, 5)) != 0 {
				t.Fatal("mutable")
			}
			if _, e = u.ApplyRepresented(geom.Vec3{X: 1}); e == nil {
				t.Fatal("rounded")
			}
		}
	}
	if !found {
		t.Fatal("oblique")
	}

	var native bytes.Buffer
	if e = WriteRationalSphericalHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteRationalSphericalHierarchyJSON(ctx, &stable, n); e != nil || !bytes.Equal(native.Bytes(), stable.Bytes()) {
		t.Fatal("stable", e)
	}
	if _, e = ReadIndependentSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old tag")
	}
	for i, u := range m.Instances {
		if !reflect.DeepEqual(u.Matrix(), n.Instances[i].Matrix()) || !reflect.DeepEqual(u.Path, n.Instances[i].Path) {
			t.Fatal("ancestry")
		}
	}
	baseRaw, _ := os.ReadFile("testdata/independent-spherical-hierarchy.step")
	base, e := ReadIndependentSphericalHierarchy(ctx, bytes.NewReader(baseRaw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for id, p := range m.Products {
		a, e := prismNativeBytes(p.Solid)
		if e != nil {
			t.Fatal(e)
		}
		b, e := prismNativeBytes(base.Products[id].Solid)
		if e != nil || !bytes.Equal(a, b) {
			t.Fatal("local native body changed", e)
		}
	}
	var source bytes.Buffer
	if e = WriteRationalSphericalHierarchySource(ctx, &source, n.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_INDEPENDENT_TORUS_ASSEMBLY"); oracle != "" {
		for _, b := range [][]byte{raw, source.Bytes()} {
			file := filepath.Join(t.TempDir(), "rational.step")
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
			for i, x := range [6]float64{-23, -20, -5, 32.4, 30, 15} {
				if math.Abs(q.Bounds[i]-x) > 2e-7 {
					t.Fatal(q.Bounds)
				}
			}
			for i, x := range [3]float64{(-9*52 + 35*12*math.Pi*math.Pi + 36.5*20*math.Pi - 20*16*math.Pi) / a, (-2.5*52 + 23*12*math.Pi*math.Pi + 28*20*math.Pi - 6*16*math.Pi) / a, (6*12*math.Pi*math.Pi - 2.5*20*math.Pi + 16*16*math.Pi) / a} {
				if math.Abs(q.SurfaceCenter[i]-x) > 1e-7 {
					t.Fatal("moments", q.SurfaceCenter)
				}
			}
		}
	}
}
func TestRationalSphericalHierarchyReject(t *testing.T) {
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var edge ExactNAUOOccurrence
	for _, u := range m.Relationships {
		if u.Matrix()[0][0].Cmp(big.NewRat(3, 5)) == 0 && u.Matrix()[0][2].Cmp(big.NewRat(4, 5)) == 0 {
			edge = u
		}
	}
	for _, u := range m.Instances {
		if m.Products[u.ProductID].Cylinder != nil && u.Matrix()[0][0].Cmp(big.NewRat(3, 5)) == 0 {
			a := u.Matrix()
			if a[0][3].Cmp(big.NewRat(144, 5)) != 0 || a[2][3].Cmp(big.NewRat(-17, 5)) != 0 {
				t.Fatal("exact composed translation")
			}
		}
	}
	for _, kind := range []string{"almost", "nonterminating", "sharedbody", "nonneutral", "owner"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := Parse(bytes.NewReader(raw), Limits{})
			fr, _ := simple(f, edge.ToPlacementID, "AXIS2_PLACEMENT_3D", 4)
			switch kind {
			case "almost":
				en := f.Entities[fr.Args[3].Ref]
				en.Records[0].Args[1].Items[0].Text = "3.00000000000000001"
				f.Entities[fr.Args[3].Ref] = en
			case "nonterminating":
				for j, coord := range [][3]string{{"5.", "0.", "-12."}, {"12.", "0.", "5."}} {
					id := fr.Args[3-j].Ref
					en := f.Entities[id]
					for k, s := range coord {
						en.Records[0].Args[1].Items[k].Text = s
					}
					f.Entities[id] = en
				}
			case "sharedbody":
				for _, en := range f.Entities {
					if len(en.Records) == 1 && en.Records[0].Type == "SPHERICAL_SURFACE" {
						b, _ := simple(f, en.Records[0].Args[1].Ref, "AXIS2_PLACEMENT_3D", 4)
						en := f.Entities[edge.ToPlacementID]
						en.Records[0].Args[1] = b.Args[1]
						f.Entities[edge.ToPlacementID] = en
						break
					}
				}
			case "nonneutral":
				fr, _ := simple(f, edge.FromPlacementID, "AXIS2_PLACEMENT_3D", 4)
				en := f.Entities[fr.Args[1].Ref]
				en.Records[0].Args[1].Items[0].Text = "1."
				f.Entities[fr.Args[1].Ref] = en
			case "owner":
				en := f.Entities[edge.TransformationID]
				en.Records[0].Args[2].Ref = edge.ToPlacementID
				f.Entities[edge.TransformationID] = en
			}
			b, _ := mappedPlanarSourceBytes(f)
			if n, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(b), Limits{}, 1e-9); e == nil || n.Source != nil {
				t.Fatal("accepted")
			}
		})
	}
	var native bytes.Buffer
	if e = WriteRationalSphericalHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"matrix", "path", "relationship", "native", "tag", "privateview"} {
		t.Run(kind, func(t *testing.T) {
			var q rationalSphericalHierarchyEnvelope
			json.Unmarshal(native.Bytes(), &q)
			switch kind {
			case "matrix":
				q.Instances[0].Matrix[0][0] = "0.6"
			case "path":
				q.Instances[0].Path[0]++
			case "relationship":
				q.Relationships[0].Source.ToPlacementID++
			case "native":
				q.LocalProof.Proof.Members[0].Native = json.RawMessage(`{}`)
			case "tag":
				q.Format = independentSphericalHierarchyFormat
			case "privateview":
				q.LocalProof.Proof.Source = q.Source
			}
			b, _ := json.Marshal(q)
			if n, e := ReadRationalSphericalHierarchyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || n.Source != nil {
				t.Fatal("accepted")
			}
		})
	}
	m.Instances[0].Path[0]++
	var out bytes.Buffer
	if e = WriteRationalSphericalHierarchyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale")
	}
	for _, l := range []Limits{{MaxBytes: 32}, {MaxEntities: 16}, {MaxSolids: 3}} {
		if _, e = ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("source limits")
		}
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 16}, {Source: Limits{MaxEntities: 16}}} {
		if _, e = ReadRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), l); e == nil {
			t.Fatal("native limits")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadRationalSphericalHierarchy(cancelled, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
}
func FuzzRationalSphericalHierarchy(f *testing.F) {
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		m, e := ReadRationalSphericalHierarchy(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}, 1e-9)
		if e != nil && m.Source != nil {
			t.Fatal("partial")
		}
	})
}
func FuzzRationalSphericalHierarchyNative(f *testing.F) {
	raw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	m, e := ReadRationalSphericalHierarchy(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteRationalSphericalHierarchyJSON(context.Background(), &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		m, e := ReadRationalSphericalHierarchyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}})
		if e != nil && m.Source != nil {
			t.Fatal("partial")
		}
	})
}
