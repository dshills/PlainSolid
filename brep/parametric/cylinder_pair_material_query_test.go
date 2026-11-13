package parametric

import (
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestFiniteTransverseCylinderMaterialQuery(t *testing.T) {
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 2}, Max: 4, Radius: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -2}, Min: -6, Max: -4, Radius: 3}
	u, e := NewFiniteTransverseCylinderSet(context.Background(), a, b, Union, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p                   geom.Vec3
		l                   geom.BorePointLocation
		boundary, incidence geom.BoreBoundaryMask
	}{
		{geom.Vec3{X: 3, Z: 8}, geom.BoreInside, 0, geom.RawCylinderFirstMax | geom.RawCylinderSecondMax},
		{geom.Vec3{X: 4, Y: 3, Z: 8}, geom.BoreBoundary, geom.RawCylinderFirstSide | geom.RawCylinderFirstMax | geom.RawCylinderSecondSide | geom.RawCylinderSecondMax, geom.RawCylinderFirstSide | geom.RawCylinderFirstMax | geom.RawCylinderSecondSide | geom.RawCylinderSecondMax},
		{geom.Vec3{X: 1, Z: 8}, geom.BoreBoundary, geom.RawCylinderSecondSide | geom.RawCylinderFirstMax, geom.RawCylinderFirstMax | geom.RawCylinderSecondSide | geom.RawCylinderSecondMax},
		{geom.Vec3{X: 5, Z: 8}, geom.BoreBoundary, geom.RawCylinderFirstSide | geom.RawCylinderSecondMax, geom.RawCylinderFirstSide | geom.RawCylinderFirstMax | geom.RawCylinderSecondMax},
		{geom.Vec3{X: 3, Z: math.Nextafter(8, 0)}, geom.BoreInside, 0, 0},
		{geom.Vec3{X: 3, Z: math.Nextafter(8, 9)}, geom.BoreInside, 0, 0},
		{geom.Vec3{X: 8, Z: 8}, geom.BoreOutside, 0, 0},
	} {
		r, e := u.Classify(tc.p)
		if e != nil || r.Location != tc.l || r.Boundary != tc.boundary || r.Incidence != tc.incidence {
			t.Fatal(tc, r, e)
		}
	}
	a.Radius = 100
	b.Origin.X = 200 // caller mutation cannot change the bound recipe
	r, e := u.Classify(geom.Vec3{X: 8, Z: 8})
	if e != nil || r.Location != geom.BoreOutside {
		t.Fatal("mutable input", r, e)
	}
	if _, e := (FiniteTransverseCylinderSet{}).Classify(geom.Vec3{}); e == nil {
		t.Fatal("zero query")
	}
}
func TestFiniteTransverseDifferenceQuery(t *testing.T) {
	for _, sign := range []float64{-1, 1} {
		a := geom.RawCylinder{Axis: geom.Vec3{Z: 2 * sign}, Max: 4, Radius: 5}
		b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -2 * sign}, Min: -4, Max: 0, Radius: 3}
		d, e := NewFiniteTransverseCylinderSet(context.Background(), a, b, Difference, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		for _, tc := range []struct {
			x, z float64
			l    geom.BorePointLocation
		}{{3, 4, geom.BoreOutside}, {0, 4, geom.BoreInside}, {1, 4, geom.BoreBoundary}, {3, 8, geom.BoreOutside}, {1, 8, geom.BoreBoundary}, {math.Nextafter(1, 0), 4, geom.BoreInside}, {math.Nextafter(1, 2), 4, geom.BoreOutside}} {
			r, e := d.Classify(geom.Vec3{X: tc.x, Z: tc.z * sign})
			if e != nil || r.Location != tc.l {
				t.Fatal(sign, tc, r, e)
			}
		}
		b.Min, b.Max = -6, -4
		d, e = NewFiniteTransverseCylinderSet(context.Background(), a, b, Difference, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		r, e := d.Classify(geom.Vec3{X: 3, Z: 8 * sign})
		if e != nil || r.Location != geom.BoreBoundary || r.Boundary != geom.RawCylinderFirstMax || r.Incidence != geom.RawCylinderFirstMax|geom.RawCylinderSecondMax {
			t.Fatal("regularized cap contact", r, e)
		}
	}
}
func TestFiniteTransverseQueryRejects(t *testing.T) {
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Max: 8, Radius: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 8}, Axis: geom.Vec3{Z: 1}, Min: 8, Max: 12, Radius: 3}
	if _, e := NewFiniteTransverseCylinderSet(context.Background(), a, b, Union, 1e-8); e == nil {
		t.Fatal("radial tangent")
	}
	b.Origin.X = 4
	b.Min = math.Nextafter(8, 0)
	if _, e := NewFiniteTransverseCylinderSet(context.Background(), a, b, Union, 1e-8); e == nil {
		t.Fatal("uncertified native tiny overlap")
	}
}

func TestFiniteTransverseBoundLineQuery(t *testing.T) {
	a := geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Max: 8, Radius: 5}
	b := geom.RawCylinder{Origin: geom.Vec3{X: 4}, Axis: geom.Vec3{Z: -1}, Min: -8, Max: 0, Radius: 3}
	s, e := NewFiniteTransverseCylinderSet(context.Background(), a, b, Difference, 1e-8)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.ClipLine(context.Background(), geom.Vec3{Z: 8}, geom.Vec3{X: 1}, -10, 10, geom.ToroidLineOptions{ParameterTolerance: 1e-10})
	if e != nil || len(r.Material.Intervals) != 1 || r.Material.Intervals[0].Boundary != geom.RawCylinderFirstMax || len(r.Sources.Intervals) != 3 {
		t.Fatal(r, e)
	}
	if r, e := (FiniteTransverseCylinderSet{}).ClipLine(context.Background(), geom.Vec3{}, geom.Vec3{X: 1}, -1, 1, geom.ToroidLineOptions{ParameterTolerance: 1e-9}); e == nil || len(r.Material.Intervals) != 0 {
		t.Fatal("zero companion", r, e)
	}
}
