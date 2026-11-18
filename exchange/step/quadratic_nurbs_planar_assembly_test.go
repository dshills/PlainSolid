package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func quadraticNURBSFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-quadratic-nurbs-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestQuadraticNURBSPlanarAssembly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := quadraticNURBSFixture(t)
	m, e := ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Surfaces()) != 12 || len(m.PCurves()) != 48 || len(m.Geometry.Assembly.Occurrences) != 3 {
		t.Fatal("source roles")
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil || !reflect.DeepEqual(f, m.Geometry.Assembly.Source) {
		t.Fatal("original source", e)
	}
	if _, e = ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old reader widened")
	}
	for _, s := range m.Surfaces() {
		if s.FaceID == 0 || s.SurfaceID == 0 || s.PoleIDs[1][1] == 0 {
			t.Fatal(s)
		}
		r, e := simple(f, s.FaceID, "ADVANCED_FACE", 4)
		if e != nil || r.Args[2].Ref != s.SurfaceID {
			t.Fatal("surface owner")
		}
	}
	for _, c := range m.PCurves() {
		r, e := simple(f, c.EdgeID, "EDGE_CURVE", 5)
		if e != nil || r.Args[3].Ref != c.SurfaceCurveID {
			t.Fatal("model owner")
		}
		p, e := simple(f, c.PCurveID, "PCURVE", 3)
		if e != nil || p.Args[1].Ref != c.SurfaceID {
			t.Fatal("pcurve owner")
		}
	}
	areas := map[float64]float64{750: 550, 250: 250}
	for _, bodies := range m.Geometry.ProductBodies {
		for _, b := range bodies {
			mass, e := b.Body.Solid.MassProperties()
			if e != nil || areas[mass.Volume] != mass.SurfaceArea {
				t.Fatal(mass, e)
			}
		}
	}
	surfaces := m.Surfaces()
	surfaces[0].Poles[1][1][0] = "forged"
	if m.Surfaces()[0].Poles[1][1][0] == "forged" {
		t.Fatal("mutable evidence")
	}
	var source bytes.Buffer
	if e = WriteQuadraticNURBSPlanarAssemblySource(ctx, &source, m, Limits{}); e != nil || !bytes.Equal(source.Bytes(), raw) {
		t.Fatal("source identity", e)
	}
	for i, j := 0, len(f.Order)-1; i < j; i, j = i+1, j-1 {
		f.Order[i], f.Order[j] = f.Order[j], f.Order[i]
	}
	reordered, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewReader(reordered), Limits{}); e != nil {
		t.Fatal("DE order", e)
	}
}
func TestQuadraticNURBSPlanarReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := quadraticNURBSFixture(t)
	m, e := ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	s, c := m.Surfaces()[0], m.PCurves()[0]
	for name, edit := range map[string]func(*File){
		"interior": func(f *File) { f.Entities[s.PoleIDs[1][1]].Records[0].Args[1].Items[0].Text = "99." },
		"near-affine": func(f *File) {
			p := &f.Entities[s.PoleIDs[1][1]].Records[0].Args[1].Items[0]
			q, _ := liftSourceRat(*p)
			q.Add(q, big.NewRat(1, 1000000000000000000))
			p.Text = q.FloatString(18)
		},
		"pcurve-midpoint": func(f *File) {
			p := &f.Entities[c.PoleIDs[1]].Records[0].Args[1].Items[0]
			q, _ := liftSourceRat(*p)
			q.Add(q, big.NewRat(1, 1000000000000000000))
			p.Text = q.FloatString(18)
		},
		"degree-decimal": func(f *File) { f.Entities[s.SurfaceID].Records[0].Args[1].Text = "2.00000000000000001" },
		"knot-domain":    func(f *File) { f.Entities[c.CurveID].Records[0].Args[7].Items[1].Text = "99." },
		"wrong-owner":    func(f *File) { f.Entities[c.PCurveID].Records[0].Args[1].Ref = m.Surfaces()[3].SurfaceID },
		"pole-sharing":   func(f *File) { f.Entities[c.CurveID].Records[0].Args[2].Items[1].Ref = c.PoleIDs[0] },
		"underflow":      func(f *File) { f.Entities[s.PoleIDs[1][1]].Records[0].Args[1].Items[0].Text = "1.E-999999999" },
	} {
		t.Run(name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			edit(f)
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			q, e := ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{})
			if e == nil || q.original != "" || len(q.Geometry.ProductBodies) != 0 {
				t.Fatal("admitted", e)
			}
		})
	}
	for _, l := range []Limits{{MaxBytes: 10}, {MaxEntities: 10}} {
		if _, e = ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	cancelctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadQuadraticNURBSPlanarAssembly(cancelctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancel")
	}
}
func quadraticNURBSNativeFixture(t testing.TB) []byte {
	t.Helper()
	m, e := ReadQuadraticNURBSPlanarAssembly(context.Background(), bytes.NewReader(quadraticNURBSFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteQuadraticNURBSPlanarAssemblyJSON(context.Background(), &out, m); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestQuadraticNURBSNative(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	raw := quadraticNURBSNativeFixture(t)
	m, e := ReadQuadraticNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteQuadraticNURBSPlanarAssemblyJSON(ctx, &stable, m); e != nil || !bytes.Equal(raw, stable.Bytes()) {
		t.Fatal("stable", e)
	}
	if e = WriteQuadraticNURBSPlanarAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	if e = WriteQuadraticNURBSPlanarAssemblySource(ctx, sideDiskShortWriter{}, m, Limits{}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	if _, e = ReadQuadraticNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(append(append([]byte(nil), raw...), []byte("{}")...)), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("trailing")
	}
	if _, e = ReadQuadraticNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{Source: Limits{MaxEntities: 10}}); e == nil {
		t.Fatal("source limit")
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadQuadraticNURBSPlanarAssemblyJSON(cc, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	for _, edit := range []func(*quadraticNURBSNativeEnvelope){func(q *quadraticNURBSNativeEnvelope) { q.Format = nurbsPlanarAssemblyNativeFormat }, func(q *quadraticNURBSNativeEnvelope) { q.Source = "ISO-10303-21;" }, func(q *quadraticNURBSNativeEnvelope) { q.Surfaces[0].PoleIDs[1][1]++ }, func(q *quadraticNURBSNativeEnvelope) { q.Curves[0].EdgeID++ }, func(q *quadraticNURBSNativeEnvelope) { q.Surfaces[0].Poles[1][1][0] = "0" }, func(q *quadraticNURBSNativeEnvelope) { q.Geometry.Occurrences[0].Transform[0][3]++ }, func(q *quadraticNURBSNativeEnvelope) {
		for i := range q.Geometry.Products {
			if len(q.Geometry.Products[i].Bodies) > 0 {
				q.Geometry.Products[i].Bodies[0].Vertices[0].X++
				return
			}
		}
	}} {
		var q quadraticNURBSNativeEnvelope
		if e = json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, _ := json.Marshal(q)
		bad, e := ReadQuadraticNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || bad.original != "" {
			t.Fatal("forged", e)
		}
	}
	if _, e = ReadObliqueNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old tag")
	}
	if _, e = ReadQuadraticNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{MaxBytes: 10}); e == nil {
		t.Fatal("limit")
	}
	m.surfaces[0].FaceID++
	var out bytes.Buffer
	if e = WriteQuadraticNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("stale source")
	}
	if e = WriteQuadraticNURBSPlanarAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale native")
	}
}
func TestQuadraticNURBSLive(t *testing.T) {
	t.Parallel()
	bin := os.Getenv("PLAINSOLID_OCCT_QUADRATIC_NURBS_PLANAR")
	if bin == "" {
		t.Skip("optional pinned OCCT")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, mode := range []string{"write", "write-shifted"} {
		dir := t.TempDir()
		src := filepath.Join(dir, "source.step")
		if b, e := exec.CommandContext(ctx, bin, mode, src).CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
		raw, e := os.ReadFile(src)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = WriteQuadraticNURBSPlanarAssemblyJSON(ctx, &native, m); e != nil {
			t.Fatal(e)
		}
		m, e = ReadQuadraticNURBSPlanarAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteQuadraticNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e != nil {
			t.Fatal(e)
		}
		dst := filepath.Join(dir, "output.step")
		if e = os.WriteFile(dst, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		for _, path := range []string{src, dst} {
			b, e := exec.CommandContext(ctx, bin, "read", path).Output()
			if e != nil {
				t.Fatal(e, string(b))
			}
			var q struct {
				Volume, Area  float64
				Solids, Faces int
				Centroid      [3]float64
				Colors        [][3]float64
			}
			if e = json.Unmarshal(b, &q); e != nil {
				t.Fatal(e, string(b))
			}
			if math.Abs(q.Volume-1750) > 1e-9 || math.Abs(q.Area-1350) > 1e-9 || q.Solids != 3 || q.Faces != 18 || len(q.Colors) != 2 {
				t.Fatal(q)
			}
			want := [3]float64{69.0 / 14, 66.0 / 7, 99.0 / 14}
			for i := range want {
				if math.Abs(q.Centroid[i]-want[i]) > 1e-12 {
					t.Fatal(q)
				}
			}
		}
	}
}
func FuzzQuadraticNURBSPlanar(f *testing.F) {
	f.Add(quadraticNURBSFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadQuadraticNURBSPlanarAssembly(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 4096, MaxDepth: 16})
		if e != nil && m.original != "" {
			t.Fatal("partial")
		}
	})
}
