package step

import (
	"bytes"
	"plainsolid/geom"
	"reflect"
	"strings"
	"testing"
)

func sphereSupportFixture() SphereSupportModel {
	return SphereSupportModel{Name: "supports", SetName: "set", LengthUnit: .001, Surfaces: []SphereSupport{{Name: "sphere", PlacementName: "frame", LocationName: "center", AxisName: "axis", ReferenceName: "radial", Location: geom.Vec3{X: 2, Y: 3, Z: 4}, Axis: geom.Vec3{X: -1}, Reference: geom.Vec3{Z: -1}, Radius: 2.5}}}
}
func TestSphereSupportsRoundtrip(t *testing.T) {
	t.Parallel()
	for _, unit := range []float64{.001, .3048, 1} {
		m := sphereSupportFixture()
		m.LengthUnit = unit
		var b bytes.Buffer
		if e := WriteSphereSupports(&b, m); e != nil {
			t.Fatal(e)
		}
		got, e := ReadSphereSupports(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatal(got, m)
		}
		var c bytes.Buffer
		if e = WriteSphereSupports(&c, got); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
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
		if p != (geom.Vec3{X: 2, Y: 3, Z: 1.5}) {
			t.Fatal(p)
		}
	}
}
func TestSphereSupportsReject(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	_ = WriteSphereSupports(&b, sphereSupportFixture())
	s := b.String()
	for _, change := range [][2]string{{"SPHERICAL_SURFACE('sphere',#8,2.5)", "SPHERICAL_SURFACE('sphere',#8,0.)"}, {"(-1.,0.,0.)", "(-2.,0.,0.)"}, {"REPRESENTATION('supports'", "SHAPE_REPRESENTATION('supports'"}, {"DATA;", "DATA;\n#99=CARTESIAN_POINT('',(0.,0.,0.));"}, {"(2.,3.,4.)", "(2.1,3.,4.)"}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change, s)
		}
		m, e := ReadSphereSupports(strings.NewReader(strings.Replace(s, change[0], change[1], 1)), Limits{})
		if e == nil || len(m.Surfaces) != 0 {
			t.Fatal("accepted", change)
		}
	}
	for _, l := range []Limits{{MaxEntities: 4}, {MaxBytes: 80}} {
		if _, e := ReadSphereSupports(strings.NewReader(s), l); e == nil {
			t.Fatal("limit")
		}
	}
}

func TestSphereSupportsOwnershipAndAtomicity(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	_ = WriteSphereSupports(&b, sphereSupportFixture())
	s := b.String()
	for _, change := range [][2]string{{"#8=AXIS2_PLACEMENT_3D('frame',#5,#6,#7)", "#8=AXIS2_PLACEMENT_3D('frame',#5,#6,#6)"}, {"GEOMETRIC_SET('set',(#9))", "GEOMETRIC_SET('set',(#9,#9))"}, {"#8=AXIS2_PLACEMENT_3D('frame',#5,#6,#7)", "#8=AXIS2_PLACEMENT_3D('frame',#5,$,#7)"}} {
		if !strings.Contains(s, change[0]) {
			t.Fatal("stale mutation", change)
		}
		if _, e := ReadSphereSupports(strings.NewReader(strings.Replace(s, change[0], change[1], 1)), Limits{}); e == nil {
			t.Fatal("ownership")
		}
	}
	m := sphereSupportFixture()
	m.Surfaces = append(m.Surfaces, m.Surfaces[0])
	m.Surfaces[1].Axis = geom.Vec3{}
	b.Reset()
	if e := WriteSphereSupports(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("partial output")
	}
}
func FuzzSphereSupports(f *testing.F) {
	var b bytes.Buffer
	_ = WriteSphereSupports(&b, sphereSupportFixture())
	f.Add(b.Bytes())
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadSphereSupports(bytes.NewReader(b), Limits{MaxBytes: 8192, MaxEntities: 64, MaxValues: 256, MaxDepth: 8})
	})
}
