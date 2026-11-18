package step

import (
	"bytes"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func cylinderSupportFixture() CylinderSupportModel {
	return CylinderSupportModel{Name: "supports", SetName: "set", LengthUnit: .001, Surfaces: []CylinderSupport{{Name: "cylinder", PlacementName: "frame", LocationName: "center", AxisName: "axis", ReferenceName: "radial", Location: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: -1}, Reference: geom.Vec3{Z: -1}, Radius: 2.5}}}
}
func TestCylinderSupportsRoundtrip(t *testing.T) {
	t.Parallel()
	for _, unit := range []float64{.001, .3048, 1} {
		m := cylinderSupportFixture()
		m.LengthUnit = unit
		var b bytes.Buffer
		if e := WriteCylinderSupports(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadCylinderSupports(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatal(got, m)
		}
		var c bytes.Buffer
		if e = WriteCylinderSupports(&c, got); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
			t.Fatal(e)
		}
		if _, e = Read(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
			t.Fatal("solid accepted")
		}
		g, e := got.Surfaces[0].Geometry()
		if e != nil {
			t.Fatal(e)
		}
		p, _ := g.Eval(0, -1.25)
		if p != (geom.Vec3{X: 3.25, Y: 3, Z: 1.5}) {
			t.Fatal(p)
		}
	}
}
func TestCylinderSupportsReject(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	_ = WriteCylinderSupports(&b, cylinderSupportFixture())
	s := b.String()
	for _, change := range [][2]string{{"CYLINDRICAL_SURFACE('cylinder',#8,2.5)", "CYLINDRICAL_SURFACE('cylinder',#8,0.)"}, {"(-1.,0.,0.)", "(-2.,0.,0.)"}, {"REPRESENTATION('supports'", "SHAPE_REPRESENTATION('supports'"}, {"DATA;", "DATA;\n#99=CARTESIAN_POINT('',(0.,0.,0.));"}, {"(2.,3.,4.)", "(2.1,3.,4.)"}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change, s)
		}
		m, e := ReadCylinderSupports(strings.NewReader(strings.Replace(s, change[0], change[1], 1)), Limits{})
		if e == nil || len(m.Surfaces) != 0 {
			t.Fatal("accepted", change)
		}
	}
	for _, l := range []Limits{{MaxEntities: 4}, {MaxBytes: 80}} {
		if _, e := ReadCylinderSupports(strings.NewReader(s), l); e == nil {
			t.Fatal("limit")
		}
	}
}

func TestCylinderSupportsOwnershipAndAtomicity(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	_ = WriteCylinderSupports(&b, cylinderSupportFixture())
	s := b.String()
	for _, change := range [][2]string{{"#8=AXIS2_PLACEMENT_3D('frame',#5,#6,#7)", "#8=AXIS2_PLACEMENT_3D('frame',#5,#6,#6)"}, {"GEOMETRIC_SET('set',(#9))", "GEOMETRIC_SET('set',(#9,#9))"}, {"#8=AXIS2_PLACEMENT_3D('frame',#5,#6,#7)", "#8=AXIS2_PLACEMENT_3D('frame',#5,$,#7)"}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change)
		}
		if _, e := ReadCylinderSupports(strings.NewReader(strings.Replace(s, change[0], change[1], 1)), Limits{}); e == nil {
			t.Fatal("ownership")
		}
	}
	m := cylinderSupportFixture()
	m.Surfaces = append(m.Surfaces, m.Surfaces[0])
	m.Surfaces[1].Axis = geom.Vec3{}
	b.Reset()
	if e := WriteCylinderSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("partial output")
	}
}
func FuzzCylinderSupports(f *testing.F) {
	var b bytes.Buffer
	_ = WriteCylinderSupports(&b, cylinderSupportFixture())
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadCylinderSupports(bytes.NewReader(b), Limits{MaxBytes: 8192, MaxEntities: 64, MaxValues: 256, MaxDepth: 8})
	})
}
