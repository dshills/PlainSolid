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
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"reflect"
	"testing"
	"time"
)

func anisotropicSTEPFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-anisotropic-weighted-station.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestWrappedAnisotropicStationSTEP(t *testing.T) {
	raw := anisotropicSTEPFixture(t)
	m, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Walls) != 2 || m.Walls[0].SideEdges[2] != m.Walls[1].SideEdges[0] {
		t.Fatal("rim ownership")
	}
	proof, e := m.Solid.ValidateWrappedAnisotropicStationLoft(context.Background(), geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	want := [][2]int64{{3, 1}, {1, 2}, {2, 1}}
	for i, s := range proof.Scales {
		for j, q := range s {
			if !q.IsInt() || q.Num().Int64() != want[i][j] {
				t.Fatal("scales", proof.Scales)
			}
		}
	}
	if _, e = ReadWrappedRationalStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("homothetic widened")
	}
	if _, e = ReadWrappedStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("circular widened")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.ReadJSONContext(context.Background(), &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	recovered, e := WrappedAnisotropicStationLoftFromSolid(context.Background(), s, m.Name, m.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, w := range m.Walls {
		a, _ := w.Geometry.ExportRecipe()
		b, _ := recovered.Walls[i].Geometry.ExportRecipe()
		if !reflect.DeepEqual(a.Surface.Data(), b.Surface.Data()) {
			t.Fatal("original support")
		}
	}
	var out bytes.Buffer
	if e = WriteWrappedAnisotropicStationLoft(&out, m); e != nil {
		t.Fatal(e)
	}
	again, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, w := range m.Walls {
		a, _ := w.Geometry.ExportRecipe()
		b, _ := again.Walls[i].Geometry.ExportRecipe()
		if !reflect.DeepEqual(a.Surface.Data(), b.Surface.Data()) {
			t.Fatal("STEP support")
		}
	}
	var stable bytes.Buffer
	if e = WriteWrappedAnisotropicStationLoft(&stable, again); e != nil || !bytes.Equal(out.Bytes(), stable.Bytes()) {
		t.Fatal("unstable", e)
	}
}

func TestWrappedAnisotropicStationSTEPReject(t *testing.T) {
	raw := anisotropicSTEPFixture(t)
	for index, edit := range []func(*File){
		func(f *File) { sideDiskEditRecord(f, "PCURVE", func(r *Record) { r.Args[1] = r.Args[2] }) },
		func(f *File) {
			sideDiskEditRecord(f, "ADVANCED_FACE", func(r *Record) { r.Args[3] = Value{Kind: Enumeration, Text: "F"} })
		},
		func(f *File) {
			sideDiskEditRecord(f, "RATIONAL_B_SPLINE_SURFACE", func(r *Record) { r.Args[0].Items[1].Items[0] = Value{Kind: Number, Text: "0.50000000000000001"} })
		},
		func(f *File) {
			sideDiskEditRecord(f, "B_SPLINE_SURFACE", func(r *Record) {
				id := r.Args[2].Items[1].Items[0].Ref
				p := f.Entities[id].Records[0]
				p.Args[1].Items[0] = Value{Kind: Number, Text: "99."}
				ovalSet(f, id, p)
			})
		},
	} {
		bad := sideDiskMutateSource(t, raw, edit)
		m, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(bad), Limits{}, WrappedSurfaceOptions{})
		if e == nil || len(m.Walls) != 0 {
			t.Fatal("unsupported geometry accepted", index)
		}
	}
	for _, l := range []Limits{{MaxBytes: 20}, {MaxFaces: 2}, {MaxVertices: 4}} {
		if _, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(raw), l, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("limits")
		}
	}
	if _, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{MaxOperations: 1}); e == nil {
		t.Fatal("proof work")
	}
	m, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Walls[1] = m.Walls[0]
	var out bytes.Buffer
	if e = WriteWrappedAnisotropicStationLoft(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("mismatched bands")
	}
}
func TestWrappedAnisotropicStationSTEPLive(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_ANISOTROPIC_STATION")
	if bin == "" {
		t.Skip("optional independent OCCT author")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dir := t.TempDir()
	source, report := filepath.Join(dir, "original.step"), filepath.Join(dir, "original.json")
	if b, e := exec.CommandContext(ctx, bin, source, report, "filled").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	type mass struct {
		Volume, Area   float64
		Centroid       [3]float64
		Solids, Shells int
	}
	data, e := os.ReadFile(report)
	if e != nil {
		t.Fatal(e)
	}
	var original mass
	if e = json.Unmarshal(data, &original); e != nil {
		t.Fatal(e)
	}
	bounds, e := m.Solid.BoundWrappedAnisotropicStationMass(ctx, trimmed.WrappedRationalStationMassOptions{VolumeTolerance: big.NewRat(1, 1000)})
	if e != nil {
		t.Fatal(e)
	}
	if bounds.AxialVolumeFactor.Cmp(big.NewRat(43, 6)) != 0 || bounds.AxialFirstZFactor.Cmp(big.NewRat(10, 1)) != 0 {
		t.Fatal("independent station integral")
	}
	lo, _ := bounds.Volume.Lower.Float64()
	hi, _ := bounds.Volume.Upper.Float64()
	if original.Volume < lo-1e-10 || original.Volume > hi+1e-10 {
		t.Fatal("volume", original, bounds.Volume)
	}
	for i, c := range bounds.Centroid {
		lo, _ = c.Lower.Float64()
		hi, _ = c.Upper.Float64()
		if original.Centroid[i] < lo-1e-9 || original.Centroid[i] > hi+1e-9 {
			t.Fatal("centroid", i, original)
		}
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	s, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	rebuilt, e := WrappedAnisotropicStationLoftFromSolid(ctx, s, m.Name, m.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, model := range []WrappedAnisotropicStationLoftBody{m, rebuilt} {
		var output bytes.Buffer
		if e = WriteWrappedAnisotropicStationLoft(&output, model); e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(dir, string(rune('a'+i))+".step")
		if e = os.WriteFile(p, output.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		if b, e := exec.CommandContext(ctx, bin, p, report, "read").CombinedOutput(); e != nil {
			t.Fatal(e, string(b))
		}
		data, e = os.ReadFile(report)
		if e != nil {
			t.Fatal(e)
		}
		var q mass
		if e = json.Unmarshal(data, &q); e != nil {
			t.Fatal(e)
		}
		if q.Solids != 1 || q.Shells != 1 || math.Abs(q.Volume-original.Volume) > 1e-8 || math.Abs(q.Area-original.Area) > 1e-8 {
			t.Fatal("OCCT", q, original)
		}
		for j := range q.Centroid {
			if math.Abs(q.Centroid[j]-original.Centroid[j]) > 1e-8 {
				t.Fatal("OCCT centroid", q, original)
			}
		}
	}
}

func FuzzWrappedAnisotropicStationSTEP(f *testing.F) {
	f.Add(anisotropicSTEPFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadWrappedAnisotropicStationLoft(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 4096, MaxValues: 65536, MaxDepth: 16}, WrappedSurfaceOptions{MaxSurfaces: 16, MaxOperations: 1000000})
		if e != nil && len(m.Walls) != 0 {
			t.Fatal("partial material")
		}
	})
}
