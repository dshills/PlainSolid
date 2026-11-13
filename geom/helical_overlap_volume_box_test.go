package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalOverlapVolumeBox(t *testing.T) {
	r := big.NewRat
	f, _ := NewFrame(Vec3{X: 1}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, 0}, {3, .25}, {2, .25}}, 2, LeftHanded, 0, 1)
	s, _ := NewSeparatedHelicalStarts(p, 1)
	radial := [2]*big.Rat{r(9, 4), r(11, 4)}
	phase := [2]*big.Rat{r(1, 8), r(1, 4)}
	axial := [2]*big.Rat{r(1, 1), r(3, 2)}
	stock := [2]*big.Rat{r(0, 1), r(4, 1)}
	// First cutter lies in [1/4,3/4], next turn is absent on this phase
	// window. The open box [9/4,11/4]×[1/8,1/4]×[1,3/2]
	// is untouched; pi coefficient=(121-81)/16 * 1/8 * 1/2=5/32.
	q, e := CertifyHelicalOverlapVolumeBox(context.Background(), s, s, r(0, 1), radial, phase, axial, stock, stock, HelicalStartsLineOptions{})
	if e != nil || !q.Proven || q.PiCoefficient.Cmp(r(5, 32)) != 0 || q.FrameAbsoluteDeterminant.Cmp(r(1, 1)) != 0 {
		t.Fatal(q, e)
	}
	q, e = CertifyHelicalOverlapVolumeBox(context.Background(), s, s, r(1, 3), radial, phase, axial, stock, stock, HelicalStartsLineOptions{})
	if e == nil || q.Proven {
		t.Fatal("shifted cutter enters box", q, e)
	}
	q, e = CertifyHelicalOverlapVolumeBox(context.Background(), s, s, r(0, 1), radial, [2]*big.Rat{r(0, 1), r(1, 1)}, axial, stock, stock, HelicalStartsLineOptions{})
	if e == nil || q.Proven {
		t.Fatal("full turn cuts box", q, e)
	}
	q, e = CertifyHelicalOverlapVolumeBox(context.Background(), s, s, r(0, 1), radial, phase, axial, stock, stock, HelicalStartsLineOptions{HelicalPolygonLineOptions: HelicalPolygonLineOptions{MaxOperations: 1}})
	if e == nil || q.Proven {
		t.Fatal(q, e)
	}
	// A represented nonorthonormal frame (possible internally after placement)
	// must multiply the chart coefficient by its exact determinant.
	scaled := s
	scaled.base.reference.frame.x = Vec3{X: 2}
	scaled.base.reference.frame.y = Vec3{Y: 3}
	q, e = CertifyHelicalOverlapVolumeBox(context.Background(), scaled, scaled, r(0, 1), radial, phase, axial, stock, stock, HelicalStartsLineOptions{})
	if e != nil || q.PiCoefficient.Cmp(r(15, 16)) != 0 || q.FrameAbsoluteDeterminant.Cmp(r(6, 1)) != 0 {
		t.Fatal(q, e)
	}
}
