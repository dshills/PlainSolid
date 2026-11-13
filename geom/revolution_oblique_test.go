package geom

import (
	"math"
	"testing"
)

func TestRevolutionSurfaceObliqueMeridian(t *testing.T) {
	poles := []Vec3{{X: 6, Y: 8}, {X: 12, Y: 16, Z: 1}, {X: 9, Y: 12, Z: 3}}
	c, _ := NewBSpline(2, poles, []float64{0, 0, 0, 1, 1, 1}, nil)
	s, e := NewRevolutionSurface(c, Vec3{}, Vec3{Z: 1}, .25, 1.25)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.Eval(.5, .5)
	want := Vec3{X: 3.25 * (3*math.Cos(.5) - 4*math.Sin(.5)), Y: 3.25 * (3*math.Sin(.5) + 4*math.Cos(.5)), Z: 1.25}
	if e != nil || p.Distance(want) > 1e-13 {
		t.Fatal(p, want, e)
	}
	for _, bad := range []Vec3{{X: -12, Y: -16, Z: 1}, {X: 12, Y: math.Nextafter(16, 17), Z: 1}} {
		q := append([]Vec3(nil), poles...)
		q[1] = bad
		c, _ = NewBSpline(2, q, []float64{0, 0, 0, 1, 1, 1}, nil)
		if _, e = NewRevolutionSurface(c, Vec3{}, Vec3{Z: 1}, .25, 1.25); e == nil {
			t.Fatal("accepted nonpositive or noncoplanar radial vector")
		}
	}
}
