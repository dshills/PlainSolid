package step

import (
	"bytes"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func torusSupportFixture() TorusSupportModel {
	return TorusSupportModel{Name: "supports", SetName: "set", LengthUnit: .001, Surfaces: []TorusSupport{{Name: "torus", PlacementName: "frame", LocationName: "center", AxisName: "axis", ReferenceName: "radial", Location: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: -1}, Reference: geom.Vec3{Z: -1}, MajorRadius: 4, MinorRadius: 1.25}}}
}
func TestTorusSupportsRoundtrip(t *testing.T) {
	t.Parallel()
	for _, unit := range []float64{.001, .3048, 1} {
		m := torusSupportFixture()
		m.LengthUnit = unit
		var b bytes.Buffer
		if e := WriteTorusSupports(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadTorusSupports(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatal(got, m)
		}
		var c bytes.Buffer
		if e = WriteTorusSupports(&c, got); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
			t.Fatal(e)
		}
		if _, e = Read(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
			t.Fatal("solid accepted")
		}
		g, e := got.Surfaces[0].Geometry()
		if e != nil {
			t.Fatal(e)
		}
		p, _ := g.Eval(0, 0)
		if p != (geom.Vec3{X: 2, Y: 3, Z: -1.25}) {
			t.Fatal(p)
		}
	}
}
func TestTorusSupportsReject(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	_ = WriteTorusSupports(&b, torusSupportFixture())
	s := b.String()
	for _, change := range [][2]string{{"TOROIDAL_SURFACE('torus',#8,4.,1.25)", "TOROIDAL_SURFACE('torus',#8,4.,4.)"}, {"(-1.,0.,0.)", "(-2.,0.,0.)"}, {"REPRESENTATION('supports'", "SHAPE_REPRESENTATION('supports'"}, {"DATA;", "DATA;\n#99=CARTESIAN_POINT('',(0.,0.,0.));"}, {"(2.,3.,4.)", "(2.1,3.,4.)"}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change, s)
		}
		m, e := ReadTorusSupports(strings.NewReader(strings.Replace(s, change[0], change[1], 1)), Limits{})
		if e == nil || len(m.Surfaces) != 0 {
			t.Fatal("accepted", change)
		}
	}
	for _, l := range []Limits{{MaxEntities: 4}, {MaxBytes: 80}} {
		if _, e := ReadTorusSupports(strings.NewReader(s), l); e == nil {
			t.Fatal("limit")
		}
	}
}

func TestTorusSupportsOwnershipAndAtomicity(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	_ = WriteTorusSupports(&b, torusSupportFixture())
	s := b.String()
	for _, change := range [][2]string{{"#8=AXIS2_PLACEMENT_3D('frame',#5,#6,#7)", "#8=AXIS2_PLACEMENT_3D('frame',#5,#6,#6)"}, {"GEOMETRIC_SET('set',(#9))", "GEOMETRIC_SET('set',(#9,#9))"}, {"#8=AXIS2_PLACEMENT_3D('frame',#5,#6,#7)", "#8=AXIS2_PLACEMENT_3D('frame',#5,$,#7)"}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change)
		}
		if _, e := ReadTorusSupports(strings.NewReader(strings.Replace(s, change[0], change[1], 1)), Limits{}); e == nil {
			t.Fatal("ownership")
		}
	}
	m := torusSupportFixture()
	m.Surfaces = append(m.Surfaces, m.Surfaces[0])
	m.Surfaces[1].Axis = geom.Vec3{}
	b.Reset()
	if e := WriteTorusSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("partial output")
	}
}
func FuzzTorusSupports(f *testing.F) {
	var b bytes.Buffer
	_ = WriteTorusSupports(&b, torusSupportFixture())
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadTorusSupports(bytes.NewReader(b), Limits{MaxBytes: 8192, MaxEntities: 64, MaxValues: 256, MaxDepth: 8})
	})
}
