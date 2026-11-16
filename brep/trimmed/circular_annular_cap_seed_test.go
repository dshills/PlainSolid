package trimmed

import (
	"context"
	"errors"
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
		if _, ok, err := certifyCircularAnnularSeedCap(ctx, f); err != nil || ok {
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
	if _, ok, err := certifyCircularAnnularSeedCap(ctx, f); err != nil || !ok {
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
	if _, ok, err := certifyCircularAnnularSeedCap(ctx, f); err != nil || ok {
		t.Fatalf("changed rational curve with identical endpoints qualified: %v %v", ok, err)
	}
}

// Bypass the nested-loft author to exercise the generic constructor directly
// with a closed topological shell and two individually valid end annuli.
func TestCircularAnnularSeedRejectsInteriorWallContact(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	makeLoft := func(center geom.Vec2, stations []WrappedLoftStation) (Solid, WrappedStationLoftProof) {
		t.Helper()
		s, _, err := LoftWrappedCircularStations(ctx, center, stations, 1e-9, o)
		if err != nil {
			t.Fatal(err)
		}
		proof, err := s.ValidateWrappedCircularStationLoft(ctx, o)
		if err != nil || !proof.Complete {
			t.Fatalf("individual loft proof: %v", err)
		}
		return s, proof
	}
	for _, tc := range []struct {
		name             string
		outerWaist, tool float64
	}{
		{"outer-crosses", 3.75, 2},
		{"outer-tangent", 4.5, 2},
		{"inner-crosses", 5, 3.125},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outer, a := makeLoft(geom.Vec2{}, []WrappedLoftStation{
				{Z: 0, Radius: 5}, {Z: 3, Radius: tc.outerWaist}, {Z: 8, Radius: 5},
			})
			inner, b := makeLoft(geom.Vec2{X: 1.5, Y: 2}, []WrappedLoftStation{
				{Z: 0, Radius: 2}, {Z: 2, Radius: tc.tool}, {Z: 8, Radius: 2},
			})
			result, _, err := joinProvedWrappedStationLoftBore(ctx, outer, inner, a.History, b.History, a.Heights, b.Heights, o)
			if !errors.Is(err, geom.ErrDomain) || len(result.Faces()) != 0 {
				t.Fatalf("cap seed returned a shell with interior ring contact: %d faces, %v", len(result.Faces()), err)
			}
		})
	}
}
