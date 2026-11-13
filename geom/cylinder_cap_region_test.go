package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestCylinderCapRegion(t *testing.T) {
	a := RawCylinder{Axis: Vec3{Z: 1}, Radius: 3, Min: -1, Max: 1}
	b := RawCylinder{Axis: Vec3{X: 1, Z: 1}, Radius: 2, Min: -5, Max: 5}
	p, e := DescribeCylinderCapRegion(context.Background(), a, b, true, SurfacePatchEvaluationOptions{})
	if e != nil || !p.Complete || p.Kind != CylinderCapEllipse {
		t.Fatal(p, e)
	}
	// Chart U=+Y,V=-X; ellipse center is model (1,0,1), hence (0,-1).
	if p.EllipseCenter[0].Sign() != 0 || p.EllipseCenter[1].Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal(p.EllipseCenter)
	}
	eval := func(f [6]*big.Rat, x, y *big.Rat) *big.Rat {
		m := []*big.Rat{new(big.Rat).Mul(x, x), new(big.Rat).Mul(x, y), new(big.Rat).Mul(y, y), x, y, big.NewRat(1, 1)}
		r := new(big.Rat)
		for i := range f {
			r.Add(r, new(big.Rat).Mul(f[i], m[i]))
		}
		return r
	}
	s, _ := NewCylinderPairCommon(a, b)
	for x := -4; x <= 4; x++ {
		for y := -4; y <= 4; y++ {
			rx, ry := big.NewRat(int64(x), 1), big.NewRat(int64(y), 1)
			inside := eval(p.Disk, rx, ry).Sign() <= 0 && eval(p.OppositeRadial, rx, ry).Sign() <= 0
			for _, c := range p.OppositeCaps {
				v := new(big.Rat).Add(new(big.Rat).Add(new(big.Rat).Mul(c[0], rx), new(big.Rat).Mul(c[1], ry)), c[2])
				inside = inside && v.Sign() <= 0
			}
			loc, _, e := s.Classify(Vec3{X: float64(-y), Y: float64(x), Z: 1})
			if e != nil || inside != (loc != BoreOutside) {
				t.Fatal(x, y, inside, loc, e)
			}
		}
	}
	b.Axis = Vec3{X: 2}
	b.Radius = 1
	for _, v := range []struct {
		z    float64
		kind CylinderCapSectionKind
	}{{.5, CylinderCapStrip}, {1, CylinderCapTangentLine}, {2, CylinderCapEmpty}} {
		a.Max = v.z
		p, e = DescribeCylinderCapRegion(context.Background(), a, b, true, SurfacePatchEvaluationOptions{})
		if e != nil || p.Kind != v.kind {
			t.Fatal(p, e)
		}
	}
	if p, e = DescribeCylinderCapRegion(context.Background(), a, b, true, SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil || p.Complete {
		t.Fatal("budget", p, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = DescribeCylinderCapRegion(ctx, a, b, false, SurfacePatchEvaluationOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}
