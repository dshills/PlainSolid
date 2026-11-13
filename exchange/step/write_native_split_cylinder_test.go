package step

import (
	"bytes"
	"os"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestNativeSplitCylinderProductChain(t *testing.T) {
	d := splitWriterFixture(t)
	s, e := parametric.Cylinder(d.Frame, d.Radius, d.Height)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteNativeSplitCylinderProduct(&b, s, d.Name, d.LengthUnit); e != nil {
		t.Fatal(e)
	}
	first := append([]byte(nil), b.Bytes()...)
	m, e := ReadSplitCylinderProduct(&b, Limits{})
	if e != nil || m.Model.Name != d.Name || m.Model.LengthUnit != d.LengthUnit || m.Model.Frame != d.Frame {
		t.Fatal(m, e)
	}
	if !reflect.DeepEqual(s, m.Model.Solid) {
		t.Fatal("native geometry changed")
	}
	got, e := NativeSplitCylinderDescriptor(m.Model.Solid, m.Model.Name, m.Model.LengthUnit)
	if e != nil || got != d {
		t.Fatal(got, e)
	}
	if e = WriteNativeSplitCylinderProduct(&b, m.Model.Solid, m.Model.Name, m.Model.LengthUnit); e != nil || !bytes.Equal(first, b.Bytes()) {
		t.Fatal("unstable native chain", e)
	}
	rounded, e := os.ReadFile("testdata/split-cylinder-occt.step")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadSplitCylinderProduct(bytes.NewReader(rounded), Limits{}); e == nil {
		t.Fatal("rounded source accepted")
	}
}

// A valid sampled native shell may deliberately supply a foreign evaluator.
// Even an exactly delegating wrapper is not a canonical descriptor proof.
type splitForeignSurface struct{ geom.Surface }

func TestNativeSplitCylinderRejectForeignGeometry(t *testing.T) {
	d := splitWriterFixture(t)
	s, e := parametric.Cylinder(d.Frame, d.Radius, d.Height)
	if e != nil {
		t.Fatal(e)
	}
	for _, which := range []int{0, 1, 2} {
		faces := s.Faces()
		faces[which].Surface = splitForeignSurface{faces[which].Surface}
		foreign, e := parametric.NewSolid(s.Vertices(), s.Edges(), faces, s.Tolerance())
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		if e = WriteNativeSplitCylinderProduct(&b, foreign, d.Name, d.LengthUnit); e == nil || b.Len() != 0 {
			t.Fatal("accepted foreign evaluator", which)
		}
	}
	changed, e := parametric.NewSolid(s.Vertices(), s.Edges(), s.Faces(), s.Tolerance()*2)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = NativeSplitCylinderDescriptor(changed, d.Name, d.LengthUnit); e == nil {
		t.Fatal("accepted modified canonical contract")
	}
	if _, e = NativeSplitCylinderDescriptor(parametric.Solid{}, d.Name, d.LengthUnit); e == nil {
		t.Fatal("accepted empty")
	}
}
