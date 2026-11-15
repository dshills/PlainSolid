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

func knottedRationalNURBSFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-knotted-rational-nurbs-planar.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestKnottedRationalNURBSPlanarAssembly(t *testing.T) {
	ctx := context.Background()
	raw := knottedRationalNURBSFixture(t)
	m, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.planar.Surfaces()) != 12 || len(m.PCurves()) != 48 || len(m.Geometry.Assembly.Occurrences) != 3 {
		t.Fatal("source roles")
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil || !reflect.DeepEqual(f, m.Geometry.Assembly.Source) {
		t.Fatal("original source", e)
	}
	if _, e = ReadCompleteAffineNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("single-span rational support route widened")
	}
	if _, e = ReadRationalUVNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("rational UV-only route widened")
	}
	if _, e = ReadTensorRationalAffineNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("polynomial UV route widened")
	}
	if _, e = ReadRationalAffineNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("degree2x1 route widened")
	}
	if _, e = ReadObliqueNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old reader widened")
	}
	if _, e = ReadMultispanNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old multi-span path widened")
	}
	if _, e = ReadElevatedNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("single-span gate widened")
	}
	if _, e = ReadQuadraticNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("quadratic gate widened")
	}
	for _, s := range m.planar.Surfaces() {
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
	areas := map[float64]float64{4050: 1710, 2700: 1440}
	for _, bodies := range m.Geometry.ProductBodies {
		for _, b := range bodies {
			mass, e := b.Body.Solid.MassProperties()
			if e != nil || areas[mass.Volume] != mass.SurfaceArea {
				t.Fatal(mass, e)
			}
		}
	}
	surfaces := m.planar.Surfaces()
	surfaces[0].Poles[1][1][0] = "forged"
	if m.planar.Surfaces()[0].Poles[1][1][0] == "forged" {
		t.Fatal("mutable evidence")
	}
	var source bytes.Buffer
	if e = WriteKnottedRationalNURBSPlanarAssemblySource(ctx, &source, m, Limits{}); e != nil || !bytes.Equal(source.Bytes(), raw) {
		t.Fatal("source identity", e)
	}
	for i, j := 0, len(f.Order)-1; i < j; i, j = i+1, j-1 {
		f.Order[i], f.Order[j] = f.Order[j], f.Order[i]
	}
	reordered, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(reordered), Limits{}); e != nil {
		t.Fatal("DE order", e)
	}
}
func knottedRationalNURBSNativeFixture(t testing.TB) []byte {
	t.Helper()
	m, e := ReadKnottedRationalNURBSPlanarAssembly(context.Background(), bytes.NewReader(knottedRationalNURBSFixture(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteKnottedRationalNURBSPlanarAssemblyJSON(context.Background(), &out, m); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestKnottedRationalNURBSNative(t *testing.T) {
	ctx := context.Background()
	raw := knottedRationalNURBSNativeFixture(t)
	m, e := ReadKnottedRationalNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var stable bytes.Buffer
	if e = WriteKnottedRationalNURBSPlanarAssemblyJSON(ctx, &stable, m); e != nil || !bytes.Equal(raw, stable.Bytes()) {
		t.Fatal("stable", e)
	}
	if e = WriteKnottedRationalNURBSPlanarAssemblyJSON(ctx, sideDiskShortWriter{}, m); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	if e = WriteKnottedRationalNURBSPlanarAssemblySource(ctx, sideDiskShortWriter{}, m, Limits{}); !errors.Is(e, io.ErrShortWrite) {
		t.Fatal(e)
	}
	if _, e = ReadKnottedRationalNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(append(append([]byte(nil), raw...), []byte("{}")...)), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("trailing")
	}
	if _, e = ReadKnottedRationalNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{Source: Limits{MaxEntities: 10}}); e == nil {
		t.Fatal("source limit")
	}
	cc, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadKnottedRationalNURBSPlanarAssemblyJSON(cc, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("cancel")
	}
	for _, edit := range []func(*knottedRationalNURBSNativeEnvelope){func(q *knottedRationalNURBSNativeEnvelope) { q.Format = nurbsPlanarAssemblyNativeFormat }, func(q *knottedRationalNURBSNativeEnvelope) { q.Source = "ISO-10303-21;" }, func(q *knottedRationalNURBSNativeEnvelope) { q.Surfaces[0].PoleIDs[1][1]++ }, func(q *knottedRationalNURBSNativeEnvelope) { q.Curves[0].EdgeID++ }, func(q *knottedRationalNURBSNativeEnvelope) { q.Surfaces[0].Poles[1][1][0] = "0" }, func(q *knottedRationalNURBSNativeEnvelope) { q.Geometry.Occurrences[0].Transform[0][3]++ }, func(q *knottedRationalNURBSNativeEnvelope) {
		for i := range q.Geometry.Products {
			if len(q.Geometry.Products[i].Bodies) > 0 {
				q.Geometry.Products[i].Bodies[0].Vertices[0].X++
				return
			}
		}
	}} {
		var q knottedRationalNURBSNativeEnvelope
		if e = json.Unmarshal(raw, &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, _ := json.Marshal(q)
		bad, e := ReadKnottedRationalNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{})
		if e == nil || bad.original != "" {
			t.Fatal("forged", e)
		}
	}
	if _, e = ReadCompleteAffineNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("single-span rational support route widened")
	}
	if _, e = ReadRationalUVNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("rational UV-only route widened")
	}
	if _, e = ReadTensorRationalAffineNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("polynomial UV route widened")
	}
	if _, e = ReadRationalAffineNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("degree2x1 route widened")
	}
	if _, e = ReadObliqueNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{}); e == nil {
		t.Fatal("old tag")
	}
	if _, e = ReadKnottedRationalNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(raw), IndependentMaterialJSONLimits{MaxBytes: 10}); e == nil {
		t.Fatal("limit")
	}
	m.planar.surfaces[0].FaceID++
	var out bytes.Buffer
	if e = WriteKnottedRationalNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("stale source")
	}
	if e = WriteKnottedRationalNURBSPlanarAssemblyJSON(ctx, &out, m); e == nil || out.Len() != 0 {
		t.Fatal("stale native")
	}
}
func TestKnottedRationalNURBSLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_KNOTTED_RATIONAL_NURBS_PLANAR")
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
		m, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		var native bytes.Buffer
		if e = WriteKnottedRationalNURBSPlanarAssemblyJSON(ctx, &native, m); e != nil {
			t.Fatal(e)
		}
		m, e = ReadKnottedRationalNURBSPlanarAssemblyJSON(ctx, &native, IndependentMaterialJSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var out bytes.Buffer
		if e = WriteKnottedRationalNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e != nil {
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
			if math.Abs(q.Volume-10800) > 1e-9 || math.Abs(q.Area-4860) > 1e-9 || q.Solids != 3 || q.Faces != 18 || len(q.Colors) != 2 {
				t.Fatal(q)
			}
			want := [3]float64{9.3125, 14.5, 12.0625}
			for i := range want {
				if math.Abs(q.Centroid[i]-want[i]) > 1e-12 {
					t.Fatal(q)
				}
			}
		}
	}
}
func FuzzKnottedRationalNURBSPlanar(f *testing.F) {
	f.Add(knottedRationalNURBSFixture(f))
	mixed, e := os.ReadFile("testdata/occt-ap242-knotted-rational-shifted-nurbs-planar.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(mixed)
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadKnottedRationalNURBSPlanarAssembly(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 4096, MaxDepth: 16})
		if e != nil && m.original != "" {
			t.Fatal("partial")
		}
	})
}

func TestKnottedRationalNURBSHomogeneousProof(t *testing.T) {
	ctx := context.Background()
	raw := knottedRationalNURBSFixture(t)
	m, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.RationalSupports()) != 12 {
		t.Fatal("rational support count")
	}
	s := m.RationalSupports()[0]
	for _, ev := range m.RationalSupports() {
		if ev.Weights != [4][4]string{{"1", "3/2", "5/2", "3"}, {"3/2", "9/4", "15/4", "9/2"}, {"5/2", "15/4", "25/4", "15/2"}, {"3", "9/2", "15/2", "9"}} || ev.UKnots != [3]string{"0", "3/2", "3"} || ev.VKnots != [3]string{"0", "3", "6"} {
			t.Fatal(ev)
		}
	}
	cp := m.RationalSupports()
	cp[0].Weights[1][0] = "99"
	if m.RationalSupports()[0].Weights[1][0] == "99" {
		t.Fatal("mutable weights")
	}
	rec := func(f *File, kind string) *Record {
		en := f.Entities[s.SurfaceID]
		for i := range en.Records {
			if en.Records[i].Type == kind {
				return &en.Records[i]
			}
		}
		t.Fatal("missing record")
		return nil
	}
	for name, edit := range map[string]func(*File){
		"weight":          func(f *File) { rec(f, "RATIONAL_B_SPLINE_SURFACE").Args[0].Items[1].Items[0].Text = "3" },
		"zero-weight":     func(f *File) { rec(f, "RATIONAL_B_SPLINE_SURFACE").Args[0].Items[1].Items[0].Text = "0" },
		"negative-weight": func(f *File) { rec(f, "RATIONAL_B_SPLINE_SURFACE").Args[0].Items[1].Items[0].Text = "-2" },
		"near-weight": func(f *File) {
			rec(f, "RATIONAL_B_SPLINE_SURFACE").Args[0].Items[1].Items[0].Text = "2.000000000000000001"
		},
		"weight-underflow": func(f *File) { rec(f, "RATIONAL_B_SPLINE_SURFACE").Args[0].Items[1].Items[0].Text = "1.E-999999999" },
		"degree":           func(f *File) { rec(f, "B_SPLINE_SURFACE").Args[0].Text = "2.000000000000000001" },
		"pole":             func(f *File) { f.Entities[s.PoleIDs[1][0]].Records[0].Args[1].Items[0].Text = "99" },
		"near-pole": func(f *File) {
			v := &f.Entities[s.PoleIDs[1][0]].Records[0].Args[1].Items[0]
			q, e := liftSourceRat(*v)
			if e != nil {
				t.Fatal(e)
			}
			q.Add(q, big.NewRat(1, 1000000000000000000))
			v.Text = q.FloatString(18)
		},
		"sharing":       func(f *File) { rec(f, "B_SPLINE_SURFACE").Args[2].Items[1].Items[0].Ref = s.PoleIDs[0][0] },
		"wrong-face":    func(f *File) { f.Entities[s.FaceID].Records[0].Args[2].Ref = m.RationalSupports()[1].SurfaceID },
		"inside-knot":   func(f *File) { rec(f, "B_SPLINE_SURFACE_WITH_KNOTS").Args[2].Items[1].Text = "1.25" },
		"discontinuous": func(f *File) { rec(f, "B_SPLINE_SURFACE_WITH_KNOTS").Args[0].Items[1].Text = "3" },
		"false-spec":    func(f *File) { rec(f, "B_SPLINE_SURFACE_WITH_KNOTS").Args[4].Text = "PIECEWISE_BEZIER_KNOTS" },
		"knots":         func(f *File) { rec(f, "B_SPLINE_SURFACE_WITH_KNOTS").Args[0].Items[0].Text = "2" },
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
			q, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{})
			if e == nil || q.original != "" || len(q.Geometry.ProductBodies) != 0 {
				t.Fatal("admitted", e)
			}
		})
	}
	// A common positive rescaling of all weights has exactly the same quotient,
	// but its original raw basis is preserved rather than normalized away.
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	w := rec(f, "RATIONAL_B_SPLINE_SURFACE")
	for i := range w.Args[0].Items {
		for j := range w.Args[0].Items[i].Items {
			v := &w.Args[0].Items[i].Items[j]
			q, e := liftSourceRat(*v)
			if e != nil {
				t.Fatal(e)
			}
			q.Mul(q, big.NewRat(2, 1))
			v.Text = q.FloatString(8)
		}
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	scaled, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{})
	if e != nil || scaled.RationalSupports()[0].Weights[2][1] != "15/2" {
		t.Fatal("raw scaled basis", e)
	}
	var native bytes.Buffer
	if e = WriteKnottedRationalNURBSPlanarAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*knottedRationalNURBSNativeEnvelope){func(q *knottedRationalNURBSNativeEnvelope) { q.Supports[0].FaceID++ }, func(q *knottedRationalNURBSNativeEnvelope) { q.Supports[0].UKnots[1] = "1/4" }, func(q *knottedRationalNURBSNativeEnvelope) { q.Supports[0].Multiplicity[1] = 2 }, func(q *knottedRationalNURBSNativeEnvelope) { q.Supports[0].Weights[1][0] = "3" }, func(q *knottedRationalNURBSNativeEnvelope) { q.Supports[0].Poles[1][0][0] = "99" }} {
		var q knottedRationalNURBSNativeEnvelope
		if e = json.Unmarshal(native.Bytes(), &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, e := json.Marshal(q)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadKnottedRationalNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("forged homogeneous proof")
		}
	}
	for _, l := range []Limits{{MaxBytes: 10}, {MaxEntities: 10}, {MaxValues: 100}, {MaxDepth: 1}} {
		if _, e = ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = ReadKnottedRationalNURBSPlanarAssembly(cctx, bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("cancel")
	}
	if _, e = ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(rationalAffineNURBSFixture(t)), Limits{}); e == nil {
		t.Fatal("degree2x1 accepted")
	}
	if _, e = ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(multispanNURBSFixture(t)), Limits{}); e == nil {
		t.Fatal("no rational support")
	}
}

func TestKnottedRationalNURBSCurveProof(t *testing.T) {
	ctx := context.Background()
	raw := knottedRationalNURBSFixture(t)
	m, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.PCurves()) != 48 {
		t.Fatal("UV count")
	}
	for _, c := range m.PCurves() {
		if c.Weights != [3]string{"1", "2", "3"} || c.EdgeID == 0 || c.SurfaceID == 0 || c.PCurveID == 0 {
			t.Fatal(c)
		}
	}
	c := m.PCurves()[0]
	other := m.PCurves()[1]
	cp := m.PCurves()
	cp[0].Weights[1] = "forged"
	if m.PCurves()[0].Weights[1] == "forged" {
		t.Fatal("mutable UV")
	}
	rec := func(f *File, kind string) *Record {
		en := f.Entities[c.CurveID]
		for i := range en.Records {
			if en.Records[i].Type == kind {
				return &en.Records[i]
			}
		}
		t.Fatal("missing record")
		return nil
	}
	for name, edit := range map[string]func(*File){
		"weight":      func(f *File) { rec(f, "RATIONAL_B_SPLINE_CURVE").Args[0].Items[1].Text = "3" },
		"zero":        func(f *File) { rec(f, "RATIONAL_B_SPLINE_CURVE").Args[0].Items[1].Text = "0" },
		"negative":    func(f *File) { rec(f, "RATIONAL_B_SPLINE_CURVE").Args[0].Items[1].Text = "-2" },
		"near-weight": func(f *File) { rec(f, "RATIONAL_B_SPLINE_CURVE").Args[0].Items[1].Text = "2.000000000000000001" },
		"pole":        func(f *File) { f.Entities[c.PoleIDs[1]].Records[0].Args[1].Items[0].Text = "99" },
		"coherent-wrong-line": func(f *File) {
			for _, pid := range c.PoleIDs {
				v := &f.Entities[pid].Records[0].Args[1].Items[0]
				q, e := liftSourceRat(*v)
				if e != nil {
					t.Fatal(e)
				}
				q.Add(q, big.NewRat(1, 1))
				v.Text = q.FloatString(8)
			}
		},
		"wrong-speed": func(f *File) {
			v := &rec(f, "B_SPLINE_CURVE_WITH_KNOTS").Args[1].Items[1]
			q, e := liftSourceRat(*v)
			if e != nil {
				t.Fatal(e)
			}
			q.Mul(q, big.NewRat(2, 1))
			v.Text = q.FloatString(8)
		},
		"reverse-parameter": func(f *File) {
			p := rec(f, "B_SPLINE_CURVE").Args[1].Items
			w := rec(f, "RATIONAL_B_SPLINE_CURVE").Args[0].Items
			p[0], p[2] = p[2], p[0]
			w[0], w[2] = w[2], w[0]
		},
		"shared-pole": func(f *File) { rec(f, "B_SPLINE_CURVE").Args[1].Items[1].Ref = other.PoleIDs[1] },
		"wrong-owner": func(f *File) { f.Entities[c.PCurveID].Records[0].Args[1].Ref = other.SurfaceID },
		"duplicate-component": func(f *File) {
			en := f.Entities[c.CurveID]
			en.Records = append(en.Records, en.Records[0])
			f.Entities[c.CurveID] = en
		},
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
			q, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{})
			if e == nil || q.original != "" || len(q.Geometry.ProductBodies) != 0 {
				t.Fatal("admitted", e)
			}
		})
	}
	var native bytes.Buffer
	if e = WriteKnottedRationalNURBSPlanarAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*knottedRationalNURBSNativeEnvelope){func(q *knottedRationalNURBSNativeEnvelope) { q.UV[0].EdgeID++ }, func(q *knottedRationalNURBSNativeEnvelope) { q.UV[0].Weights[1] = "3" }, func(q *knottedRationalNURBSNativeEnvelope) { q.UV[0].Poles[1][0] = "99" }} {
		var q knottedRationalNURBSNativeEnvelope
		if e = json.Unmarshal(native.Bytes(), &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, e := json.Marshal(q)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadKnottedRationalNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("forged UV proof")
		}
	}
	if _, e = ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(tensorRationalAffineNURBSFixture(t)), Limits{}); e == nil {
		t.Fatal("no rational UV")
	}
}

func TestKnottedRationalNURBSModelProof(t *testing.T) {
	ctx := context.Background()
	raw := knottedRationalNURBSFixture(t)
	m, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.ModelCurves()) != 24 {
		t.Fatal("model count", len(m.ModelCurves()))
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range m.ModelCurves() {
		if c.Degree != 3 || c.Count != 5 || c.Multiplicity[1] != 1 || c.Knots[2] == "" {
			t.Fatal(c)
		}
		sc, e := simple(f, c.SurfaceCurveID, "SURFACE_CURVE", 4)
		if e != nil || sc.Args[1].Ref != c.CurveID {
			t.Fatal("model owner", e)
		}
		edge, e := simple(f, c.EdgeID, "EDGE_CURVE", 5)
		if e != nil || edge.Args[3].Ref != c.SurfaceCurveID {
			t.Fatal("edge owner", e)
		}
	}
	copies := m.ModelCurves()
	copies[0].Poles[1][0] = "forged"
	if m.ModelCurves()[0].Poles[1][0] == "forged" {
		t.Fatal("mutable models")
	}
	c := m.ModelCurves()[0]
	other := m.ModelCurves()[1]
	for name, edit := range map[string]func(*File){
		"same-owner-wrong-speed": func(f *File) {
			values := f.Entities[c.CurveID].Records[0].Args[7].Items
			for i := range values {
				q, e := liftSourceRat(values[i])
				if e != nil {
					t.Fatal(e)
				}
				q.Mul(q, big.NewRat(2, 1))
				values[i].Text = q.FloatString(8)
			}
		},
		"coherent-model-shift": func(f *File) {
			for i := 0; i < c.Count; i++ {
				v := &f.Entities[c.PoleIDs[i]].Records[0].Args[1].Items[0]
				q, e := liftSourceRat(*v)
				if e != nil {
					t.Fatal(e)
				}
				q.Add(q, big.NewRat(1, 1))
				v.Text = q.FloatString(8)
			}
		},
		"inside-span": func(f *File) { f.Entities[c.PoleIDs[2]].Records[0].Args[1].Items[0].Text = "99" },
		"almost-line": func(f *File) {
			v := &f.Entities[c.PoleIDs[2]].Records[0].Args[1].Items[0]
			q, e := liftSourceRat(*v)
			if e != nil {
				t.Fatal(e)
			}
			q.Add(q, big.NewRat(1, 1000000000000000000))
			v.Text = q.FloatString(18)
		},
		"shared-curve": func(f *File) { f.Entities[other.SurfaceCurveID].Records[0].Args[1].Ref = c.CurveID },
		"shared-pole":  func(f *File) { f.Entities[c.CurveID].Records[0].Args[2].Items[1].Ref = other.PoleIDs[1] },
		"reversed-basis": func(f *File) {
			v := f.Entities[c.CurveID].Records[0].Args[2].Items
			for i, j := 0, len(v)-1; i < j; i, j = i+1, j-1 {
				v[i], v[j] = v[j], v[i]
			}
		},
		"knots":        func(f *File) { f.Entities[c.CurveID].Records[0].Args[7].Items[1].Text = "0" },
		"multiplicity": func(f *File) { f.Entities[c.CurveID].Records[0].Args[6].Items[1].Text = "4" },
		"sense":        func(f *File) { f.Entities[c.EdgeID].Records[0].Args[4].Text = "F" },
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
			q, e := ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(b), Limits{})
			if e == nil || q.original != "" || len(q.Geometry.ProductBodies) != 0 {
				t.Fatal("admitted", e)
			}
		})
	}
	var native bytes.Buffer
	if e = WriteKnottedRationalNURBSPlanarAssemblyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*knottedRationalNURBSNativeEnvelope){func(q *knottedRationalNURBSNativeEnvelope) { q.Models[0].EdgeID++ }, func(q *knottedRationalNURBSNativeEnvelope) { q.Models[0].Poles[1][0] = "99" }, func(q *knottedRationalNURBSNativeEnvelope) { q.Models[0].Multiplicity[1]++ }} {
		var q knottedRationalNURBSNativeEnvelope
		if e = json.Unmarshal(native.Bytes(), &q); e != nil {
			t.Fatal(e)
		}
		edit(&q)
		b, e := json.Marshal(q)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ReadKnottedRationalNURBSPlanarAssemblyJSON(ctx, bytes.NewReader(b), IndependentMaterialJSONLimits{}); e == nil {
			t.Fatal("native model corruption")
		}
	}
	if _, e = ReadKnottedRationalNURBSPlanarAssembly(ctx, bytes.NewReader(multispanNURBSFixture(t)), Limits{}); e == nil {
		t.Fatal("no elevated model evidence")
	}
}
