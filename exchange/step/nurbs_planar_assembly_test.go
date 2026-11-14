package step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func nurbsPlanarFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-ap242-nurbs-planar-assembly.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestNURBSPlanarAssembly(t *testing.T) {
	raw := nurbsPlanarFixture(t)
	ctx := context.Background()
	m, e := ReadNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Surfaces()) != 12 || len(m.Curves()) != 24 || len(m.PCurves()) != 48 || len(m.Geometry.Assembly.Occurrences) != 3 || m.Geometry.Assembly.LengthUnit != .001 {
		t.Fatal("source role loss")
	}
	colors := map[RGB]int{}
	volumes := map[float64]int{}
	for _, bs := range m.Geometry.ProductBodies {
		for _, b := range bs {
			p, e := b.Body.Solid.MassProperties()
			if e != nil {
				t.Fatal(e)
			}
			volumes[p.Volume]++
			if b.SurfaceColor == nil {
				t.Fatal("missing color")
			}
			colors[*b.SurfaceColor]++
		}
	}
	if volumes[24] != 1 || volumes[6] != 1 || colors[RGB{1, 0, 0}] != 1 || colors[RGB{0, 0, 1}] != 1 {
		t.Fatal(volumes, colors)
	}
	if _, e = ReadColoredAssembly(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old planar reader widened")
	}
	if _, e = ReadAffineSplineShells(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old affine reader widened")
	}
	// Exact shifted owner basis really occurs in the independent source.
	shifted := false
	for _, p := range m.PCurves() {
		if p.Intercept[0] != "0" || p.Intercept[1] != "0" {
			shifted = true
		}
	}
	if !shifted {
		t.Fatal("fixture missing shifted parameters")
	}
	old := m.Surfaces()[0]
	copy := m.Surfaces()
	copy[0].Origin[0] = "forged"
	if m.Surfaces()[0] != old {
		t.Fatal("mutable evidence")
	}
	var out bytes.Buffer
	if e = WriteNURBSPlanarAssemblySource(ctx, &out, m, Limits{}); e != nil || !bytes.Equal(raw, out.Bytes()) {
		t.Fatal("source changed", e)
	}
	n, e := ReadNURBSPlanarAssembly(ctx, bytes.NewReader(out.Bytes()), Limits{})
	if e != nil || !reflect.DeepEqual(m.surfaces, n.surfaces) || !reflect.DeepEqual(m.pcurves, n.pcurves) || !reflect.DeepEqual(m.Geometry.Assembly.Occurrences, n.Geometry.Assembly.Occurrences) {
		t.Fatal("roundtrip ancestry", e)
	}
}
func TestNURBSPlanarAssemblyReject(t *testing.T) {
	raw := nurbsPlanarFixture(t)
	m, e := ReadNURBSPlanarAssembly(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	s, c, p := m.Surfaces()[0], m.Curves()[0], m.PCurves()[0]
	mutations := map[string]func(*File){
		"rounded support":    func(f *File) { f.Entities[s.PoleIDs[0]].Records[0].Args[1].Items[0].Text = "1.E-30" },
		"curved support":     func(f *File) { f.Entities[s.PoleIDs[3]].Records[0].Args[1].Items[0].Text = "1." },
		"surface knot drift": func(f *File) { f.Entities[s.SurfaceID].Records[0].Args[10].Items[1].Text = "4.00000000000000000001" },
		"model knot drift":   func(f *File) { f.Entities[c.CurveID].Records[0].Args[7].Items[1].Text = "4.00000000000000000001" },
		"model nonlinear":    func(f *File) { f.Entities[c.CurveID].Records[0].Args[1].Text = "2" },
		"uv nonlinear":       func(f *File) { f.Entities[p.CurveID].Records[0].Args[1].Text = "2" },
		"uv phase":           func(f *File) { f.Entities[p.PoleIDs[0]].Records[0].Args[1].Items[0].Text = "1." },
		"uv rounded":         func(f *File) { f.Entities[p.PoleIDs[0]].Records[0].Args[1].Items[0].Text = "1.E-30" },
		"uv outside chart":   func(f *File) { f.Entities[p.PoleIDs[1]].Records[0].Args[1].Items[0].Text = "99." },
		"uv interval":        func(f *File) { f.Entities[p.CurveID].Records[0].Args[7].Items[1].Text = "5." },
		"wrong owner":        func(f *File) { f.Entities[p.PCurveID].Records[0].Args[1].Ref = m.Surfaces()[1].SurfaceID },
		"borrow poles":       func(f *File) { f.Entities[c.CurveID].Records[0].Args[2].Items[0].Ref = m.Curves()[1].PoleIDs[0] },
		"edge sense":         func(f *File) { f.Entities[c.EdgeID].Records[0].Args[4].Text = "F" },
		"underflow":          func(f *File) { f.Entities[s.PoleIDs[0]].Records[0].Args[1].Items[0].Text = "1.E-999999999" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			mutate(f)
			b, e := mappedPlanarSourceBytes(f)
			if e != nil {
				t.Fatal(e)
			}
			got, e := ReadNURBSPlanarAssembly(context.Background(), bytes.NewReader(b), Limits{})
			if e == nil || got.OriginalSTEP() != "" || len(got.Geometry.ProductBodies) != 0 {
				t.Fatal("accepted invalid source", e)
			}
		})
	}
	// Reordering records is harmless; no role is recovered by file position.
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i, j := 0, len(f.Order)-1; i < j; i, j = i+1, j-1 {
		f.Order[i], f.Order[j] = f.Order[j], f.Order[i]
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadNURBSPlanarAssembly(context.Background(), bytes.NewReader(b), Limits{}); e != nil {
		t.Fatal("reordered source", e)
	}
}
func TestNURBSPlanarAssemblyLimits(t *testing.T) {
	raw := nurbsPlanarFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := ReadNURBSPlanarAssembly(ctx, bytes.NewReader(raw), Limits{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxBytes: int64(len(raw) - 1)}, {MaxEntities: 600}, {MaxDepth: 2}, {MaxSolids: 1}} {
		if m, e := ReadNURBSPlanarAssembly(context.Background(), bytes.NewReader(raw), l); e == nil || m.original != "" {
			t.Fatal("limit accepted", l)
		}
	}
	var out bytes.Buffer
	if e := WriteNURBSPlanarAssemblySource(context.Background(), &out, NURBSPlanarAssembly{}, Limits{}); e == nil || out.Len() != 0 {
		t.Fatal("non atomic invalid write")
	}
}
func TestNURBSPlanarAssemblyLive(t *testing.T) {
	raw := nurbsPlanarFixture(t)
	m, e := ReadNURBSPlanarAssembly(context.Background(), bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	model := ColoredAssemblyModel{LengthUnit: m.Geometry.Assembly.LengthUnit}
	key := func(id int64) string { return strconv.FormatInt(id, 10) }
	for _, p := range m.Geometry.Assembly.Products {
		model.Products = append(model.Products, ColoredAssemblyPart{ID: key(p.DefinitionID), Name: p.Name, Bodies: m.Geometry.ProductBodies[p.DefinitionID]})
	}
	for _, o := range m.Geometry.Assembly.Occurrences {
		model.Occurrences = append(model.Occurrences, AssemblyInstance{ID: o.Identifier, Name: o.Name, Parent: key(o.Parent), Child: key(o.Child), Transform: o.Transform})
	}
	var generated bytes.Buffer
	if e = WriteColoredAssembly(&generated, model); e != nil {
		t.Fatal(e)
	}
	view, e := mappedPlanarSourceBytes(m.Geometry.Assembly.Source)
	if e != nil {
		t.Fatal(e)
	}
	bin := os.Getenv("PLAINSOLID_OCCT_AP242_NURBS_PLANAR")
	if bin == "" {
		t.Skip("set PLAINSOLID_OCCT_AP242_NURBS_PLANAR")
	}
	for _, b := range [][]byte{raw, []byte(m.OriginalSTEP()), view, generated.Bytes()} {
		path := filepath.Join(t.TempDir(), "assembly.step")
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		out, e := exec.CommandContext(ctx, bin, "read", path).CombinedOutput()
		cancel()
		if e != nil {
			t.Fatalf("%v: %s", e, out)
		}
		var q struct {
			Volume, Area float64
			Colors       [][3]float64
			Centroid     [3]float64
		}
		at := bytes.IndexByte(out, '{')
		if at < 0 || json.Unmarshal(out[at:], &q) != nil {
			t.Fatal(string(out))
		}
		if math.Abs(q.Volume-54) > 1e-9 || math.Abs(q.Area-126) > 1e-9 || len(q.Colors) != 2 || math.Abs(q.Centroid[0]-85.0/18) > 1e-9 || math.Abs(q.Centroid[1]-86.0/9) > 1e-9 || math.Abs(q.Centroid[2]-2.5) > 1e-9 {
			t.Fatal(q)
		}
	}
}
func FuzzNURBSPlanarAssembly(f *testing.F) {
	f.Add(nurbsPlanarFixture(f))
	f.Add([]byte("ISO-10303-21;"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadNURBSPlanarAssembly(context.Background(), bytes.NewReader(b), Limits{MaxEntities: 1024})
		if e != nil {
			if m.original != "" || len(m.Geometry.ProductBodies) != 0 {
				t.Fatal("partial failure")
			}
			return
		}
		var out bytes.Buffer
		if e = WriteNURBSPlanarAssemblySource(context.Background(), &out, m, Limits{}); e != nil || !bytes.Equal(b, out.Bytes()) {
			t.Fatal("source stability", e)
		}
	})
}
