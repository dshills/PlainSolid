package trimmed

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestFiniteBoreRadialSeparation(t *testing.T) {
	base, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 30, Y: 30, Z: 30})
	cases := []struct {
		cuts   []CylinderCut
		depths []float64
	}{
		{[]CylinderCut{{5, 3, geom.Vec3{Y: 10, Z: 10}, geom.Vec3{X: 1}, 2}, {0, 1, geom.Vec3{X: 10, Y: 13.5}, geom.Vec3{Z: 1}, 2}}, []float64{9, 9}},
		{[]CylinderCut{{0, 1, geom.Vec3{X: 10, Y: 10}, geom.Vec3{Z: 1}, 2}, {0, 1, geom.Vec3{X: 13, Y: 13}, geom.Vec3{Z: 1}, 2}}, []float64{8, 10}},
		{[]CylinderCut{{0, 1, geom.Vec3{X: 10, Y: 10}, geom.Vec3{Z: 1}, 2}, {1, 0, geom.Vec3{X: 13, Y: 13, Z: 30}, geom.Vec3{Z: -1}, 2}}, []float64{20, 20}},
	}
	for _, c := range cases {
		if _, _, e := BoreCylinders(base, c.cuts, 1e-9); e == nil {
			t.Fatal("full AABB proof unexpectedly passes")
		}
		s, _, _, e := BoreFiniteCylinders(base, c.cuts, c.depths, 1e-9)
		if e != nil {
			t.Fatal(c, e)
		}
		mass, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := 27000 - 4*math.Pi*(c.depths[0]+c.depths[1])
		if math.Abs(mass.Volume-want) > 1e-7 {
			t.Fatal(mass, want)
		}
		mesh, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
	cuts := cases[0].cuts
	cuts[1].AxisPoint.Y = 13
	if _, _, _, e := BoreFiniteCylinders(base, cuts, []float64{9, 9}, 1e-9); e == nil {
		t.Fatal("overlap accepted")
	}
}
func TestFiniteBoreInputsFailSafely(t *testing.T) {
	base, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 20, Y: 20, Z: 20})
	good := CylinderCut{0, 1, geom.Vec3{X: 5, Y: 5}, geom.Vec3{Z: 1}, 1}
	for _, cut := range []CylinderCut{{-1, 1, good.AxisPoint, good.Direction, 1}, {0, 99, good.AxisPoint, good.Direction, 1}, {0, 0, good.AxisPoint, good.Direction, 1}, {2, 1, good.AxisPoint, good.Direction, 1}, {0, 1, good.AxisPoint, geom.Vec3{}, 1}, {0, 1, geom.Vec3{X: math.NaN()}, good.Direction, 1}, {0, 1, good.AxisPoint, good.Direction, -1}, {0, 1, good.AxisPoint, good.Direction, math.Inf(1)}} {
		if _, _, _, e := BoreFiniteCylinders(base, []CylinderCut{cut}, []float64{5}, 1e-9); e == nil {
			t.Fatal("forged tool accepted", cut)
		}
	}
	if _, _, _, e := BoreFiniteCylinders(base, []CylinderCut{good}, nil, 1e-9); e == nil {
		t.Fatal("missing depths")
	}
	if _, _, _, e := BoreFiniteCylinders(base, []CylinderCut{good}, []float64{5}, math.NaN()); e == nil {
		t.Fatal("nan tolerance")
	}
}
func TestFiniteBoreExactRadicalContact(t *testing.T) {
	// U=25−4²=9,V=25−3²=16, so exact contact occurs at D=7.
	a := finiteBoreEnvelope{axis: 0, center: geom.Vec3{}, radius: 5, box: geom.Box{Min: geom.Vec3{X: -2}, Max: geom.Vec3{X: 2}}, valid: true}
	b := finiteBoreEnvelope{axis: 2, center: geom.Vec3{X: 5, Y: 7}, radius: 5, box: geom.Box{Min: geom.Vec3{Z: 4}, Max: geom.Vec3{Z: 10}}, valid: true}
	if finiteBoresSeparated(a, b) {
		t.Fatal("contact separated")
	}
	b.center.Y = math.Nextafter(7, math.Inf(1))
	if !finiteBoresSeparated(a, b) {
		t.Fatal("exact represented positive gap lost")
	}
	a.padding = 1e-9
	if finiteBoresSeparated(a, b) {
		t.Fatal("padding ignored")
	}
}
