package geom

import (
	"math"
	"testing"
)

func TestRevolutionSurfaceRational(t *testing.T) {
	c, e := NewBSpline(2, []Vec3{{X: 2}, {X: 4, Z: 1}, {X: 3, Z: 3}}, []float64{0, 0, 0, 1, 1, 1}, []float64{1, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewRevolutionSurface(c, Vec3{}, Vec3{Z: 1}, .25, 1.25)
	if e != nil {
		t.Fatal(e)
	}
	p, _ := s.Eval(.5, .5)
	du, dv, _ := s.Partials(.5, .5)
	if p.Distance(Vec3{X: 3 * math.Cos(.5), Y: 3 * math.Sin(.5), Z: 4. / 3}) > 1e-14 || du.Distance(Vec3{X: -3 * math.Sin(.5), Y: 3 * math.Cos(.5)}) > 1e-14 || dv.Distance(Vec3{X: 4. / 3 * math.Cos(.5), Y: 4. / 3 * math.Sin(.5), Z: 4}) > 1e-14 {
		t.Fatal(p, du, dv)
	}
	for _, sign := range []float64{1, -1} {
		c, e = NewBSpline(2, []Vec3{{X: 2}, {X: 3, Z: sign}, {X: 4, Z: 2 * sign}, {X: 5, Z: 3 * sign}}, []float64{0, 0, 0, .5, 1, 1, 1}, []float64{1, .5, .75, 1})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = NewRevolutionSurface(c, Vec3{}, Vec3{Z: 1}, 0, 1); e != nil {
			t.Fatal("multispan", sign, e)
		}
	}
}
func TestRevolutionRationalProofRejects(t *testing.T) {
	for _, h := range [][3]float64{{0, 2, 1}, {0, 0, 0}} {
		c, _ := NewBSpline(2, []Vec3{{X: 2, Z: h[0]}, {X: 3, Z: h[1]}, {X: 4, Z: h[2]}}, []float64{0, 0, 0, 1, 1, 1}, []float64{1, .5, 1})
		if e := certifyRevolutionRationalHeight(c, 2, 1); e == nil {
			t.Fatal("unproved derivative")
		}
	}
	p := make([]Vec3, 67)
	w := make([]float64, 67)
	k := []float64{0, 0}
	for i := range p {
		p[i] = Vec3{X: 2, Z: float64(i)}
		w[i] = 1
		if i%2 == 1 {
			w[i] = .5
		}
		if i > 0 {
			k = append(k, float64(i))
		}
	}
	k = append(k, 66)
	c, e := NewBSpline(1, p, k, w)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = NewRevolutionSurface(c, Vec3{}, Vec3{Z: 1}, 0, 1); e == nil {
		t.Fatal("span limit")
	}
}
