package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSphereCylinderRimHeightBounds(t *testing.T) {
	for _, axis := range []Vec3{{Z: 1}, {Z: -3}, {X: 1}, {Y: -2}} {
		center := Vec3{X: .5, Y: .25}
		if axis.X != 0 {
			center = Vec3{Y: .5, Z: .25}
		} else if axis.Y != 0 {
			center = Vec3{Z: .5, X: .25}
		}
		p, e := BoundSphereCylinderRimHeight(context.Background(), center, 3, Vec3{}, axis, 1, SurfacePatchEvaluationOptions{})
		if e != nil || !p.Proven {
			t.Fatal(p, e)
		}
		min, _ := p.HeightBounds[0].Float64()
		max, _ := p.HeightBounds[1].Float64()
		d1, _ := p.FirstDerivativeAbsolute.Float64()
		d2, _ := p.SecondDerivativeAbsolute.Float64()
		for i := 0; i < 64; i++ {
			u := float64(i) * 2 * math.Pi / 64
			h, a, b, e := p.Evaluate(u)
			if e != nil || h < min-1e-14 || h > max+1e-14 || math.Abs(a) > d1+1e-14 || math.Abs(b) > d2+1e-14 {
				t.Fatal(u, h, a, b, p, e)
			}
			eps := 1e-5
			hm, am, _, _ := p.Evaluate(u - eps)
			hp, ap, _, _ := p.Evaluate(u + eps)
			if math.Abs((hp-hm)/(2*eps)-a) > 1e-8 || math.Abs((ap-am)/(2*eps)-b) > 1e-8 {
				t.Fatal(u, h, a, b)
			}
		}
	}
	q, e := BoundSphereCylinderRimHeight(context.Background(), Vec3{}, 5, Vec3{}, Vec3{Z: 1}, 3, SurfacePatchEvaluationOptions{})
	if e != nil || q.HeightBounds[0].Cmp(big.NewRat(4, 1)) != 0 || q.HeightBounds[1].Cmp(big.NewRat(4, 1)) != 0 || q.FirstDerivativeAbsolute.Sign() != 0 || q.SecondDerivativeAbsolute.Sign() != 0 || q.RimSecondDerivativeNormSquared.Cmp(big.NewRat(9, 1)) != 0 {
		t.Fatal(q, e)
	}
	q, e = BoundSphereCylinderRimHeight(context.Background(), Vec3{X: 2}, 3, Vec3{}, Vec3{Z: 1}, 1, SurfacePatchEvaluationOptions{})
	if e != ErrDomain || q.Proven {
		t.Fatal(q, e)
	}
	q, e = BoundSphereCylinderRimHeight(context.Background(), Vec3{X: 1}, 3, Vec3{}, Vec3{Z: 1}, 1, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Proven {
		t.Fatal(q, e)
	}
}
