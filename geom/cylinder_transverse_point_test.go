package geom

import (
	"math"
	"testing"
)

func TestTransverseCylinderRegularizedPointMasks(t *testing.T) {
	a := RawCylinder{Axis: Vec3{Z: 1}, Max: 8, Radius: 5}
	b := RawCylinder{Origin: Vec3{X: 4}, Axis: Vec3{Z: -1}, Min: -8, Max: 0, Radius: 3}
	u, e := NewTransverseCylinderSetQuery(a, b, false)
	if e != nil {
		t.Fatal(e)
	}
	r, e := u.Classify(Vec3{X: 1, Z: 8})
	if e != nil || r.Location != BoreBoundary || r.Boundary != RawCylinderFirstMax|RawCylinderSecondMin || r.Incidence != RawCylinderFirstMax|RawCylinderSecondSide|RawCylinderSecondMin {
		t.Fatal("hidden side on shared outward cap", r, e)
	}
	d, e := NewTransverseCylinderSetQuery(a, b, true)
	if e != nil {
		t.Fatal(e)
	}
	r, e = d.Classify(Vec3{X: 1, Z: 8})
	if e != nil || r.Location != BoreBoundary || r.Boundary != RawCylinderFirstMax|RawCylinderSecondSide {
		t.Fatal("canceled cutter cap", r, e)
	}
	r, e = d.Classify(Vec3{X: 3, Z: 8})
	if e != nil || r.Location != BoreOutside || r.Boundary != 0 || r.Incidence != RawCylinderFirstMax|RawCylinderSecondMin {
		t.Fatal("regularized versus closed", r, e)
	}
	// Exact oblique raw axes, using a represented similarity of the same point.
	a = RawCylinder{Axis: Vec3{X: 3, Z: 4}, Max: 8, Radius: 25}
	b = RawCylinder{Origin: Vec3{Y: 20}, Axis: Vec3{X: -3, Z: -4}, Min: -12, Max: -8, Radius: 15}
	u, e = NewTransverseCylinderSetQuery(a, b, false)
	if e != nil {
		t.Fatal(e)
	}
	r, e = u.Classify(Vec3{X: 24, Y: 15, Z: 32})
	if e != nil || r.Location != BoreInside || r.Boundary != 0 || r.Incidence != RawCylinderFirstMax|RawCylinderSecondMax {
		t.Fatal("oblique cap cancellation", r, e)
	}
	if _, e = u.Classify(Vec3{X: math.NaN()}); e == nil {
		t.Fatal("nonfinite query")
	}
	if _, e = (TransverseCylinderSetQuery{}).Classify(Vec3{}); e == nil {
		t.Fatal("zero query")
	}
	b.Axis = Vec3{Z: 1}
	if _, e = NewTransverseCylinderSetQuery(a, b, false); e == nil {
		t.Fatal("nonparallel")
	}
}
