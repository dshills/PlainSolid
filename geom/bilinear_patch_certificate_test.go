package geom

import "testing"

func TestExactBilinearCertificateWeightsAndOwnership(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	poles := []Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 2}}
	for _, weights := range [][]float64{{.5, .5, .5, .5}, {1, 1, 1, 2}} {
		s, e := NewBSplineSurface(1, 1, 2, 2, poles, k, k, weights)
		if e != nil {
			t.Fatal(e)
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		c, ok := p.BilinearCoefficients()
		if weights[3] == 2 {
			if ok {
				t.Fatal("rational weights falsely certified polynomial")
			}
			continue
		}
		if !ok || c[3][2].RatString() != "2" {
			t.Fatal(c, ok)
		}
		c[3][2].SetInt64(999)
		d, _ := p.BilinearCoefficients()
		if d[3][2].RatString() != "2" {
			t.Fatal("aliased exact coefficients")
		}
	}
}
