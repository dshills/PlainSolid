package trimmed

import (
	"context"
	"testing"

	"plainsolid/geom"
)

func TestCircularAnnularSeedProvesOriginalCurves(t *testing.T) {
	ctx := context.Background()
	cap := func(center geom.Vec3, radius float64) Face {
		t.Helper()
		patch, err := geom.NewCanonicalWrappedCylinder(ctx, center, radius, 4, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		solid, _, err := NewWrappedCylinder(ctx, patch, 1e-9, geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return solid.Faces()[1]
	}
	ring := func(inner Face) []Use {
		loop := make([]Use, 4)
		for i, u := range inner.Outer {
			loop[3-i] = Use{UVStart: u.UVEnd, UVEnd: u.UVStart, Curve: reversePCurve(u.Curve)}
		}
		return loop
	}
	f := cap(geom.Vec3{}, 5)
	for _, center := range []geom.Vec3{{X: 3}, {X: 3.125}} {
		f.Holes = [][]Use{ring(cap(center, 2))}
		if ok, err := certifyCircularAnnularSeedCap(ctx, f); err != nil || ok {
			t.Fatalf("tangent or intersecting original circles qualified: %v %v", ok, err)
		}
	}
	f.Holes = [][]Use{ring(cap(geom.Vec3{X: 1.5, Y: 2}, 2))}
	var coarse [][]geom.Vec3
	for _, loop := range [2][]Use{f.Outer, f.Holes[0]} {
		var points []geom.Vec3
		for _, u := range loop {
			points = append(points, geom.Vec3{X: u.UVStart.X, Y: u.UVStart.Y})
		}
		coarse = append(coarse, points)
	}
	if _, err := triangulateUV(coarse); err == nil {
		t.Fatal("test must reproduce the rejected original chord rings")
	}
	if ok, err := certifyCircularAnnularSeedCap(ctx, f); err != nil || !ok {
		t.Fatalf("strict original circular containment: %v %v", ok, err)
	}
	if level, err := circularAnnularSeedLevel(ctx, f); err != nil || level < 1 || level > maxCircularAnnularSeedLevel {
		t.Fatalf("bounded original sample search: %d %v", level, err)
	}

	u := &f.Holes[0][0]
	arc := u.Curve.(SplineUV)
	poles := arc.Curve.Poles()
	poles[1].X += .125
	curve, err := geom.NewBSpline(2, poles, arc.Curve.Knots(), arc.Curve.Weights())
	if err != nil {
		t.Fatal(err)
	}
	arc.Curve = curve
	u.Curve = arc
	if ok, err := certifyCircularAnnularSeedCap(ctx, f); err != nil || ok {
		t.Fatalf("changed rational curve with identical endpoints qualified: %v %v", ok, err)
	}
}
