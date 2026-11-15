package geom

import (
	"errors"
	"math"
	"testing"
)

func TestOuterTorusExcludesSingularAndReflectedSheets(t *testing.T) {
	f, err := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, radius := range []float64{2, 7} {
		s, err := NewOuterTorusSurface(f, 2, radius)
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			origin, direction Vec3
			x                 []float64
			multiplicity      int
		}{
			{Vec3{}, Vec3{X: 1}, []float64{-2 - radius, 2 + radius}, 1},
			{Vec3{Z: radius}, Vec3{X: 1}, []float64{-2, 2}, 2},
			{Vec3{Y: 2, Z: radius}, Vec3{X: 1}, []float64{0}, 4},
			{Vec3{}, Vec3{Z: 1}, nil, 0},
		} {
			l, err := NewLine(test.origin, test.direction)
			if err != nil {
				t.Fatal(err)
			}
			hits, err := IntersectLineOuterTorus(l, s, LineTorusOptions{})
			if err != nil || len(hits.Hits) != len(test.x) {
				t.Fatalf("radius %v, line %+v: %+v, %v", radius, test, hits, err)
			}
			for i, hit := range hits.Hits {
				if math.Abs(hit.Parameter-test.x[i]) > 1e-9 || hit.Multiplicity != test.multiplicity || hit.Crossing != (test.multiplicity%2 == 1) {
					t.Fatalf("radius %v, line %+v: hit %+v", radius, test, hit)
				}
			}
		}
		for _, v := range []float64{-math.Pi / 2, 0, math.Pi / 2} {
			u := .25
			du, dv, err := s.Partials(u, v)
			if err != nil || du.Cross(dv).Length() < 2*radius-1e-12 {
				t.Fatalf("regular meridian at %v: %v %v %v", v, du, dv, err)
			}
		}
		if _, err := s.Eval(0, math.Nextafter(math.Pi/2, math.Inf(1))); !errors.Is(err, ErrDomain) {
			t.Fatalf("out-of-domain meridian admitted: %v", err)
		}
	}
}
