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

func TestOCCTRationalSphericalHierarchy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/occt-rational-spherical-hierarchy.step")
	m, e := ReadOCCTRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Products) != 4 || len(m.Instances) != 7 || len(m.Relationships) != 8 {
		t.Fatal("counts")
	}
	if _, e = ReadRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("neutral mode widened")
	}
	found := false
	for _, r := range m.Relationships {
		a := r.Matrix()
		if a[0][0].Cmp(big.NewRat(9, 25)) == 0 {
			found = true
			if a[0][3].Cmp(big.NewRat(677, 50)) != 0 || a[1][3].Cmp(big.NewRat(-368, 25)) != 0 {
				t.Fatal("relative frame")
			}
			a[0][3].SetInt64(99)
			if r.Matrix()[0][3].Cmp(big.NewRat(677, 50)) != 0 {
				t.Fatal("mutable")
			}
		}
	}
	if !found {
		t.Fatal("noncommuting relationship")
	}
	var native bytes.Buffer
	if e = WriteOCCTRationalSphericalHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	n, e := ReadOCCTRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteOCCTRationalSphericalHierarchyJSON(ctx, &stable, n); e != nil || !bytes.Equal(native.Bytes(), stable.Bytes()) {
		t.Fatal("stable", e)
	}
	if _, e = ReadRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old tag")
	}
	for i, u := range m.Instances {
		if !reflect.DeepEqual(u.Matrix(), n.Instances[i].Matrix()) || !reflect.DeepEqual(u.Path, n.Instances[i].Path) {
			t.Fatal("paths")
		}
	}
	baseRaw, _ := os.ReadFile("testdata/rational-spherical-hierarchy.step")
	base, e := ReadRationalSphericalHierarchy(ctx, bytes.NewReader(baseRaw), Limits{}, 1e-9)
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
			t.Fatal("local body changed")
		}
	}
	var source bytes.Buffer
	if e = WriteOCCTRationalSphericalHierarchySource(ctx, &source, n.Source, Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	// Independent analytic support extrema and local surface first moments under
	// each exact reported occurrence. No source pose-composition helper is used.
	bounds := [6]float64{math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	moment := [3]float64{}
	for _, u := range n.Instances {
		p := n.Products[u.ProductID]
		a := u.Matrix()
		var local geom.Vec3
		var area float64
		switch {
		case p.Planar != nil:
			local = geom.Vec3{X: -9, Y: -2.5}
			area = 52
		case p.Torus != nil:
			local = geom.Vec3{X: 1, Y: 2, Z: 3}
			area = 12 * math.Pi * math.Pi
		case p.Cylinder != nil:
			local = geom.Vec3{Z: 1.5}
			area = 20 * math.Pi
		case p.Sphere != nil:
			local = geom.Vec3{X: 1, Y: 2, Z: 3}
			area = 16 * math.Pi
		default:
			t.Fatal("family")
		}
		c, e := u.ApplyExact(local)
		if e != nil {
			t.Fatal(e)
		}
		if p.Torus != nil && a[0][0].Cmp(big.NewRat(9, 25)) == 0 {
			if c[0].Cmp(big.NewRat(3169, 250)) != 0 || c[1].Cmp(big.NewRat(-2821, 125)) != 0 || c[2].Cmp(big.NewRat(33, 25)) != 0 {
				t.Fatal("composed torus center")
			}
		}
		for i := 0; i < 3; i++ {
			center, _ := c[i].Float64()
			axis, _ := a[i][2].Float64()
			extent := 0.
			switch {
			case p.Planar != nil:
				extent = []float64{1, 1.5, 2}[i]
			case p.Sphere != nil:
				extent = 2
			case p.Torus != nil:
				extent = 3*math.Sqrt(1-axis*axis) + 1
			case p.Cylinder != nil:
				extent = 1.5*math.Abs(axis) + 2*math.Sqrt(1-axis*axis)
			}
			bounds[i] = math.Min(bounds[i], center-extent)
			bounds[i+3] = math.Max(bounds[i+3], center+extent)
			moment[i] += area * center
		}
	}
	area := 52 + 24*math.Pi*math.Pi + 72*math.Pi
	volume := 24 + 12*math.Pi*math.Pi + (24+64./3)*math.Pi
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
			if q.Solids != 7 || q.Faces != 22 || q.EdgeUses != 88 || math.Abs(q.Area-area) > 1e-7 || math.Abs(q.Volume-volume) > 1e-7 {
				t.Fatal(q)
			}
			for i, x := range bounds {
				if math.Abs(q.Bounds[i]-x) > 2e-7 {
					t.Fatal("bounds", q.Bounds, bounds)
				}
			}
			for i, x := range moment {
				if math.Abs(q.SurfaceCenter[i]-x/area) > 1e-7 {
					t.Fatal("moment", q.SurfaceCenter, moment)
				}
			}
		}
	}
}
func TestOCCTRationalSphericalHierarchyReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw, _ := os.ReadFile("testdata/occt-rational-spherical-hierarchy.step")
	m, e := ReadOCCTRationalSphericalHierarchy(ctx, bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var edge ExactNAUOOccurrence
	for _, r := range m.Relationships {
		if r.Matrix()[0][0].Cmp(big.NewRat(9, 25)) == 0 {
			edge = r
		}
	}
	for _, kind := range []string{"almost", "nonterminating", "owner", "bodysharing", "huge"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := Parse(bytes.NewReader(raw), Limits{})
			fr, _ := simple(f, edge.FromPlacementID, "AXIS2_PLACEMENT_3D", 4)
			switch kind {
			case "almost":
				en := f.Entities[fr.Args[2].Ref]
				en.Records[0].Args[1].Items[0].Text = "0.00000000000000001"
				f.Entities[fr.Args[2].Ref] = en
			case "nonterminating":
				en := f.Entities[fr.Args[3].Ref]
				en.Records[0].Args[1].Items[0].Text = "5."
				en.Records[0].Args[1].Items[1].Text = "12."
				f.Entities[fr.Args[3].Ref] = en
			case "owner":
				en := f.Entities[edge.TransformationID]
				en.Records[0].Args[2].Ref = edge.ToPlacementID
				f.Entities[edge.TransformationID] = en
			case "bodysharing":
				for _, en := range f.Entities {
					if len(en.Records) == 1 && en.Records[0].Type == "SPHERICAL_SURFACE" {
						b, _ := simple(f, en.Records[0].Args[1].Ref, "AXIS2_PLACEMENT_3D", 4)
						en := f.Entities[edge.FromPlacementID]
						en.Records[0].Args[1] = b.Args[1]
						f.Entities[edge.FromPlacementID] = en
						break
					}
				}
			case "huge":
				en := f.Entities[fr.Args[1].Ref]
				en.Records[0].Args[1].Items[0].Text = "1.E-999999999"
				f.Entities[fr.Args[1].Ref] = en
			}
			b, _ := mappedPlanarSourceBytes(f)
			if n, e := ReadOCCTRationalSphericalHierarchy(ctx, bytes.NewReader(b), Limits{}, 1e-9); e == nil || n.Source != nil {
				t.Fatal("accepted")
			}
		})
	}
	var native bytes.Buffer
	if e = WriteOCCTRationalSphericalHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"tag", "pose", "from", "path", "native"} {
		t.Run(kind, func(t *testing.T) {
			var q rationalSphericalHierarchyEnvelope
			json.Unmarshal(native.Bytes(), &q)
			switch kind {
			case "tag":
				q.Format = rationalSphericalHierarchyFormat
			case "pose":
				q.Relationships[0].Matrix[0][3] = "0.1"
			case "from":
				q.Relationships[0].Source.FromPlacementID++
			case "path":
				q.Instances[0].Path[0]++
			case "native":
				q.LocalProof.Proof.Members[0].Native = json.RawMessage(`{}`)
			}
			b, _ := json.Marshal(q)
			if n, e := ReadOCCTRationalSphericalHierarchyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil || n.Source != nil {
				t.Fatal("accepted")
			}
		})
	}
	m.Instances[0].Path[0]++
	var out bytes.Buffer
	if e = WriteOCCTRationalSphericalHierarchyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale")
	}
	for _, l := range []Limits{{MaxBytes: 32}, {MaxEntities: 16}, {MaxSolids: 3}} {
		if _, e = ReadOCCTRationalSphericalHierarchy(ctx, bytes.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("source limits")
		}
	}
	for _, l := range []IndependentMaterialJSONLimits{{MaxBytes: 32}, {Source: Limits{MaxEntities: 16}}} {
		if _, e = ReadOCCTRationalSphericalHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), l); e == nil {
			t.Fatal("native limits")
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadOCCTRationalSphericalHierarchy(cancelled, bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("cancel")
	}
}
func FuzzOCCTRationalSphericalHierarchy(f *testing.F) {
	raw, _ := os.ReadFile("testdata/occt-rational-spherical-hierarchy.step")
	f.Add(raw)
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 131072 {
			return
		}
		m, e := ReadOCCTRationalSphericalHierarchy(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}, 1e-9)
		if e != nil && m.Source != nil {
			t.Fatal("partial")
		}
	})
}
func FuzzOCCTRationalSphericalHierarchyNative(f *testing.F) {
	raw, _ := os.ReadFile("testdata/occt-rational-spherical-hierarchy.step")
	m, e := ReadOCCTRationalSphericalHierarchy(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		f.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteOCCTRationalSphericalHierarchyJSON(context.Background(), &out, m); e != nil {
		f.Fatal(e)
	}
	f.Add(out.Bytes())
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			return
		}
		m, e := ReadOCCTRationalSphericalHierarchyJSON(context.Background(), bytes.NewReader(b), IndependentMaterialJSONLimits{MaxBytes: 1 << 20, Source: Limits{MaxBytes: 131072, MaxEntities: 1024, MaxValues: 16384, MaxDepth: 16}})
		if e != nil && m.Source != nil {
			t.Fatal("partial")
		}
	})
}
