package trimmed

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestFiniteDisjointOrthogonalBores(t *testing.T) {
	base, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 20, Y: 20, Z: 20})
	cuts := []CylinderCut{{Entry: 5, Exit: 3, AxisPoint: geom.Vec3{Y: 10, Z: 10}, Direction: geom.Vec3{X: 1}, Radius: 1}, {Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 10, Y: 10}, Direction: geom.Vec3{Z: 1}, Radius: 1}}
	if _, _, e := BoreCylinders(base, cuts, 1e-9); e == nil {
		t.Fatal("projected proof unexpectedly passed")
	}
	for _, depths := range [][]float64{{4, 4}, {0, 7}, {7, 0}} {
		s, _, floors, e := BoreFiniteCylinders(base, cuts, depths, 1e-9)
		if e != nil {
			t.Fatal(depths, e)
		}
		mass, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		v, a := 8000., 2400.
		for i, d := range depths {
			if d == 0 {
				d = 20
				a -= 2 * math.Pi
				if floors[i] != -1 {
					t.Fatal(floors)
				}
			} else if floors[i] < 0 {
				t.Fatal(floors)
			}
			v -= math.Pi * d
			a += 2 * math.Pi * d
		}
		if math.Abs(mass.Volume-v) > 1e-8 || math.Abs(mass.Area-a) > 1e-8 {
			t.Fatal(mass, v, a)
		}
		m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 100000})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
	for _, depths := range [][]float64{{10, 10}, {9, 9}, {0, 10}, {-1, 4}, {math.NaN(), 4}, {21, 4}} {
		if _, _, _, e := BoreFiniteCylinders(base, cuts, depths, 1e-9); e == nil {
			t.Fatal("accepted overlap/contact/invalid", depths)
		}
	}
	cuts[1].AxisPoint.Y = 15
	original, h, e := BoreCylinders(base, cuts, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	retained, fh, floors, e := BoreFiniteCylinders(base, cuts, []float64{0, 0}, 1e-9)
	if e != nil || !reflect.DeepEqual(original, retained) || !reflect.DeepEqual(h, fh) || !reflect.DeepEqual(floors, []int{-1, -1}) {
		t.Fatal("changed through topology", e)
	}
}
