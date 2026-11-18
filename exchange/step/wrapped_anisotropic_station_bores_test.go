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

func anisotropicBoreSTEPFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-anisotropic-weighted-station-bore.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestWrappedAnisotropicStationBoreSTEP(t *testing.T) {
	slowtest.Skip(t)
	raw := anisotropicBoreSTEPFixture(t)
	m, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Outer.Walls) != 2 || len(m.Inner.Walls) != 2 {
		t.Fatal("original bands")
	}
	proof, e := m.Solid.ValidateWrappedAnisotropicStationBore(context.Background(), geom.SurfacePatchOptions{})
	if e != nil || !proof.Complete {
		t.Fatal(e)
	}
	if m.Solid.Topology().EulerCharacteristic != 0 || m.Solid.Topology().InnerWires != 2 {
		t.Fatal("annular cap topology")
	}
	if _, e = ReadWrappedRationalStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("homothetic widened")
	}
	var native bytes.Buffer
	if e = trimmed.WriteJSON(&native, m.Solid); e != nil {
		t.Fatal(e)
	}
	solid, e := trimmed.ReadJSONContext(context.Background(), &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	recovered, e := WrappedAnisotropicStationBoreFromSolid(context.Background(), solid, m.Name, m.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var out bytes.Buffer
	if e = WriteWrappedAnisotropicStationBore(&out, recovered); e != nil {
		t.Fatal(e)
	}
	again, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for k, a := range []WrappedAnisotropicStationLoftBody{m.Outer, m.Inner} {
		b := []WrappedAnisotropicStationLoftBody{again.Outer, again.Inner}[k]
		for i, w := range a.Walls {
			x, _ := w.Geometry.ExportRecipe()
			y, _ := b.Walls[i].Geometry.ExportRecipe()
			if !reflect.DeepEqual(x.Surface.Data(), y.Surface.Data()) {
				t.Fatal("original support")
			}
			if i > 0 && b.Walls[i-1].SideEdges[2] != b.Walls[i].SideEdges[0] {
				t.Fatal("shared rim")
			}
		}
	}
	var stable bytes.Buffer
	if e = WriteWrappedAnisotropicStationBore(&stable, again); e != nil || !bytes.Equal(out.Bytes(), stable.Bytes()) {
		t.Fatal("stable", e)
	}
}

func TestWrappedAnisotropicStationBoreSTEPReject(t *testing.T) {
	slowtest.Skip(t)
	raw := anisotropicBoreSTEPFixture(t)
	for index, edit := range []func(*File){
		func(f *File) { sideDiskEditRecord(f, "PCURVE", func(r *Record) { r.Args[1] = r.Args[2] }) },
		func(f *File) {
			sideDiskEditRecord(f, "ADVANCED_FACE", func(r *Record) {
				if r.Args[3].Text == "T" {
					r.Args[3].Text = "F"
				} else {
					r.Args[3].Text = "T"
				}
			})
		},
		func(f *File) {
			sideDiskEditRecord(f, "RATIONAL_B_SPLINE_SURFACE", func(r *Record) { r.Args[0].Items[1].Items[0] = Value{Kind: Number, Text: "0.50000000000000001"} })
		},
	} {
		bad := sideDiskMutateSource(t, raw, edit)
		m, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(bad), Limits{}, WrappedSurfaceOptions{})
		if e == nil || len(m.Outer.Walls) != 0 {
			t.Fatal("bad original", index)
		}
	}
	for _, l := range []Limits{{MaxBytes: 20}, {MaxFaces: 2}, {MaxVertices: 4}} {
		if _, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), l, WrappedSurfaceOptions{}); e == nil {
			t.Fatal("limits")
		}
	}
	if _, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{MaxOperations: 1}); e == nil {
		t.Fatal("work")
	}
	m, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m.Inner = m.Outer
	var out bytes.Buffer
	if e = WriteWrappedAnisotropicStationBore(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("touching walls")
	}
}

func FuzzWrappedAnisotropicStationBoreSTEP(f *testing.F) {
	f.Add(anisotropicBoreSTEPFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		m, e := ReadWrappedAnisotropicStationBore(bytes.NewReader(b), Limits{MaxBytes: 1 << 20, MaxEntities: 4096, MaxValues: 65536, MaxDepth: 16}, WrappedSurfaceOptions{MaxSurfaces: 32, MaxOperations: 4000000})
		if e != nil && len(m.Outer.Walls) != 0 {
			t.Fatal("partial material")
		}
	})
}
