package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestHelicalConstantSliceIntegral(t *testing.T) {
	r := big.NewRat
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, 0}, {3, .25}, {2, .25}}, 1, RightHanded, 0, .5)
	s, _ := NewSeparatedHelicalStarts(p, 1)
	radial := [2]*big.Rat{r(2, 1), r(3, 1)}
	stock := [2]*big.Rat{r(0, 1), r(1, 1)}
	q, e := IntegrateHelicalConstantSliceCell(context.Background(), s, s, r(0, 1), radial, [2]*big.Rat{r(0, 1), r(1, 4)}, stock, stock, HelicalStartsLineOptions{})
	if e != nil || !q.Proven || q.PiCoefficient.Cmp(r(15, 16)) != 0 || q.LengthSlope.Sign() != 0 || q.LengthIntercept.Cmp(r(3, 4)) != 0 {
		t.Fatal(q, e)
	}
	// Cutter exists at phase 1/2 only, then is absent. Integrating endpoint
	// values would be wrong; open cell length is exactly one throughout.
	q, e = IntegrateHelicalConstantSliceCell(context.Background(), s, s, r(0, 1), radial, [2]*big.Rat{r(1, 2), r(3, 4)}, stock, stock, HelicalStartsLineOptions{})
	if e != nil || q.PiCoefficient.Cmp(r(5, 4)) != 0 || q.LengthIntercept.Cmp(r(1, 1)) != 0 {
		t.Fatal(q, e)
	}
	narrow := [2]*big.Rat{r(0, 1), r(1, 8)}
	q, e = IntegrateHelicalConstantSliceCell(context.Background(), s, s, r(0, 1), radial, [2]*big.Rat{r(0, 1), r(1, 8)}, narrow, narrow, HelicalStartsLineOptions{})
	if e != nil || q.PiCoefficient.Cmp(r(5, 128)) != 0 || q.LengthSlope.Cmp(r(1, 1)) != 0 || q.LengthIntercept.Sign() != 0 {
		t.Fatal(q, e)
	}
	q, e = IntegrateHelicalConstantSliceCell(context.Background(), s, s, r(0, 1), radial, [2]*big.Rat{r(0, 1), r(3, 4)}, stock, stock, HelicalStartsLineOptions{})
	if e != ErrConvergence || q.Proven {
		t.Fatal(q, e)
	}
	triangle, _ := NewHelicalPolygon(f, []Vec2{{2, 0}, {3, .125}, {2, .25}}, 1, RightHanded, 0, .5)
	v, _ := NewSeparatedHelicalStarts(triangle, 1)
	q, e = IntegrateHelicalConstantSliceCell(context.Background(), v, v, r(0, 1), radial, [2]*big.Rat{r(0, 1), r(1, 8)}, stock, stock, HelicalStartsLineOptions{})
	if e != ErrConvergence || q.Proven {
		t.Fatal(q, e)
	}
}
