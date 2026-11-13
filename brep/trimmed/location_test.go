package trimmed

import (
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBoreRayAndLocation(t *testing.T) {
	b, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	s, h, e := BoreCylinder(b, 0, 1, geom.Vec3{X: 2, Y: 2.5}, .5, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p geom.Vec3
		w parametric.Location
	}{
		{geom.Vec3{X: 2, Y: 2.5, Z: 1}, parametric.Outside},
		{geom.Vec3{X: 2, Y: 2.5}, parametric.Outside},
		{geom.Vec3{X: 1.5, Y: 2.5, Z: 3}, parametric.Boundary},
		{geom.Vec3{X: 2, Y: 3}, parametric.Boundary},
		{geom.Vec3{X: 1, Y: 2.5, Z: 3}, parametric.Inside},
		{geom.Vec3{X: 1, Y: 2.5, Z: 1e-11}, parametric.Boundary},
		{geom.Vec3{X: 1.5 + 1e-7, Y: 2.5, Z: 3}, parametric.Outside},
		{geom.Vec3{X: 1.5 - 1e-7, Y: 2.5, Z: 3}, parametric.Inside},
	} {
		got, e := s.Classify(tc.p, 1e-9)
		if e != nil || got != tc.w {
			t.Errorf("location of %v: got %v (%v), want %v", tc.p, got, e, tc.w)
		}
	}
	l, e := s.Ray(geom.Vec3{X: -1, Y: 2.5, Z: 3}, geom.Vec3{X: 1}, 20, 1e-9)
	if e != nil || len(l) != 4 {
		t.Fatal(l, e)
	}
	for i, d := range []float64{1, 2.5, 3.5, 5} {
		if math.Abs(l[i].Distance-d) > 1e-9 {
			t.Fatal(l)
		}
	}
	if !l[1].OnTrim || len(l[1].Faces) != 2 ||
		l[1].Faces[0] != h.WallFaces[1] || l[1].Faces[1] != h.WallFaces[2] {
		t.Fatal(l[1], h)
	}
	l, e = s.Ray(geom.Vec3{X: 2, Y: 2.5, Z: -1}, geom.Vec3{Z: 1}, math.Inf(1), 1e-9)
	if e != nil || len(l) != 0 {
		t.Fatal(l, e)
	}
	l, e = s.Ray(geom.Vec3{X: -1, Y: 3, Z: 3}, geom.Vec3{X: 1}, 20, 1e-9)
	if e != nil || len(l) != 3 || !l[1].Tangent {
		t.Fatal(l, e)
	}
}

func TestObliqueBoreRay(t *testing.T) {
	b, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	s, _, e := BoreCylinderAlong(b, 0, 3, geom.Vec3{X: 1, Y: 2}, geom.Vec3{X: 2, Z: 1}, .35, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p geom.Vec3
		w parametric.Location
	}{
		{geom.Vec3{X: 3, Y: 2, Z: 1}, parametric.Outside},
		{geom.Vec3{X: 3, Y: 2.5, Z: 1}, parametric.Inside},
		{geom.Vec3{X: 3, Y: 2.35, Z: 1}, parametric.Boundary},
		{geom.Vec3{X: 4, Y: 2, Z: 1.5}, parametric.Outside},
		{geom.Vec3{X: 4, Y: 2.35, Z: 1.5}, parametric.Boundary},
	} {
		got, e := s.Classify(tc.p, 1e-9)
		if e != nil || got != tc.w {
			t.Errorf("location of %v: got %v (%v), want %v", tc.p, got, e, tc.w)
		}
	}
	l, e := s.Ray(geom.Vec3{X: 3, Y: -1, Z: 1}, geom.Vec3{Y: 1}, 20, 1e-9)
	if e != nil || len(l) != 4 || math.Abs(l[1].Distance-2.65) > 1e-9 || math.Abs(l[2].Distance-3.35) > 1e-9 {
		t.Fatal(l, e)
	}
}

func TestAffineBoreQueries(t *testing.T) {
	b, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	s, _, e := BoreCylinder(b, 0, 1, geom.Vec3{X: 2, Y: 2.5}, .5, 1e-10)
	if e != nil {
		t.Fatal(e)
	}
	m := geom.Affine{{-2, .25, 0, 7}, {0, 3, .125, 2}, {0, 0, 1, -2}}
	r, e := s.Transformed(m)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		p geom.Vec3
		w parametric.Location
	}{
		{geom.Vec3{X: 2, Y: 2.5, Z: 1}, parametric.Outside},
		{geom.Vec3{X: 1, Y: 2.5, Z: 1}, parametric.Inside},
		{geom.Vec3{X: 1.5, Y: 2.5, Z: 1}, parametric.Boundary},
	} {
		loc, e := r.Classify(m.Apply(tc.p), 1e-9)
		if e != nil || loc != tc.w {
			t.Fatal(loc, tc, e)
		}
	}
	origin := geom.Vec3{X: -1, Y: 2.5, Z: 3}
	dir := geom.Vec3{X: 1}
	h, e := r.Ray(m.Apply(origin), m.ApplyVector(dir), 30, 1e-9)
	if e != nil || len(h) != 4 {
		t.Fatal(h, e)
	}
	for i, d := range []float64{1, 2.5, 3.5, 5} {
		if math.Abs(h[i].Distance-2*d) > 1e-9 || h[i].Point.Distance(m.Apply(origin.Add(dir.Scale(d)))) > 1e-9 {
			t.Fatal(h)
		}
	}
}

func TestTrimmedTorusAndConeQueries(t *testing.T) {
	frame, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	const tol = 1e-8
	ring, e := parametric.Torus(frame, 3, 1)
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromParametric(ring)
	if e != nil {
		t.Fatal(e)
	}
	hits, e := s.Ray(geom.Vec3{X: -5}, geom.Vec3{X: 1}, 10, tol)
	if e != nil || len(hits) != 4 {
		t.Fatal(hits, e)
	}
	for i, want := range []float64{1, 3, 7, 9} {
		if math.Abs(hits[i].Distance-want) > tol || hits[i].Tangent {
			t.Fatal(hits)
		}
	}
	for _, tc := range []struct {
		p    geom.Vec3
		want parametric.Location
	}{
		{geom.Vec3{}, parametric.Outside},
		{geom.Vec3{X: 3}, parametric.Inside},
		{geom.Vec3{X: 3, Z: -.5}, parametric.Inside},
		{geom.Vec3{X: 3, Z: -1}, parametric.Boundary},
		{geom.Vec3{X: 4}, parametric.Boundary},
	} {
		got, e := s.Classify(tc.p, tol)
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	cone, e := parametric.Cone(frame, math.Atan(.5), 4)
	if e != nil {
		t.Fatal(e)
	}
	s, e = FromParametric(cone)
	if e != nil {
		t.Fatal(e)
	}
	hits, e = s.Ray(geom.Vec3{X: .5, Z: -1}, geom.Vec3{Z: 1}, 6, tol)
	if e != nil || len(hits) != 2 || math.Abs(hits[0].Distance-2) > tol || math.Abs(hits[1].Distance-5) > tol {
		t.Fatal(hits, e)
	}
	for _, tc := range []struct {
		p    geom.Vec3
		want parametric.Location
	}{
		{geom.Vec3{Z: 2}, parametric.Inside},
		{geom.Vec3{X: 1.1, Z: 2}, parametric.Outside},
		{geom.Vec3{X: 1, Z: 2}, parametric.Boundary},
		{geom.Vec3{Z: 4}, parametric.Boundary},
		{geom.Vec3{}, parametric.Boundary},
	} {
		got, e := s.Classify(tc.p, tol)
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
}
