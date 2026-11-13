package geom

import "testing"

func TestBiquadraticExactCoefficientsAndOwnership(t *testing.T) {
	var poles []Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z += 1
			}
			if j == 2 {
				z += 2
			}
			if i == 2 && j == 2 {
				z += 3
			}
			poles = append(poles, Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
		}
	}
	k := []float64{2, 2, 2, 6, 6, 6}
	s, e := NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	c, ok := p.BiquadraticCoefficients()
	if !ok || c[2][0][2].RatString() != "1" || c[0][2][2].RatString() != "2" || c[2][2][2].RatString() != "3" {
		t.Fatal(c, ok)
	}
	c[2][2][2].SetInt64(999)
	again, _ := p.BiquadraticCoefficients()
	if again[2][2][2].RatString() != "3" {
		t.Fatal("aliased coefficient")
	}
}
