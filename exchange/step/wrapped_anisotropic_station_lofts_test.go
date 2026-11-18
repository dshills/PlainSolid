package step

import (
	"bytes"
	"context"
	"os"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
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
	slowtest.Skip(t)
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
	t.Parallel()
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
