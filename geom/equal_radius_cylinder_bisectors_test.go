package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestEqualRadiusCylinderBisectors(t *testing.T) {
	center := Vec3{X: 1, Y: -2, Z: 3}
	u, v := Vec3{X: 3, Y: 4}, Vec3{X: 4, Y: 3}
	q, e := IntersectEqualRadiusCylinderBisectors(context.Background(), center, u, v, 2, SurfacePatchEvaluationOptions{})
	if e != nil || !q.Complete {
		t.Fatal(q, e)
	}
	dot := func(a, b [3]*big.Rat) *big.Rat {
		z := new(big.Rat)
		for i := 0; i < 3; i++ {
			z.Add(z, new(big.Rat).Mul(a[i], b[i]))
		}
		return z
	}
	for _, ellipse := range q.Ellipses {
		if dot(ellipse.PlaneNormal, ellipse.BasisB).Sign() != 0 || dot(ellipse.PlaneNormal, ellipse.BasisW).Sign() != 0 || dot(ellipse.BasisB, ellipse.BasisW).Sign() != 0 {
			t.Fatal(ellipse)
		}
		for _, t0 := range []float64{0, .3, 1.7, 3, 5} {
			p, e := ellipse.Eval(t0)
			d, _ := ellipse.Derivative(t0)
			x := p.Sub(center)
			for _, axis := range []Vec3{u, v} {
				normal := x.Sub(axis.Scale(x.Dot(axis) / axis.Dot(axis)))
				if e != nil || math.Abs(normal.Dot(normal)-4) > 1e-11 || math.Abs(normal.Dot(d)) > 1e-11 {
					t.Fatal(p, d, e)
				}
			}
		}
	}
	// Cross product=(0,0,-7), so shared nodes are exactly center±(0,0,2).
	if q.Nodes[0].Bounds[2][0].Cmp(big.NewRat(5, 1)) != 0 || q.Nodes[1].Bounds[2][1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(q.Nodes)
	}
	a, _ := q.Ellipses[0].Derivative(math.Pi / 2)
	b, _ := q.Ellipses[1].Derivative(math.Pi / 2)
	if a.Cross(b).Length() == 0 {
		t.Fatal("node curve tangents parallel")
	}
	before, _ := q.Ellipses[0].Eval(.3)
	q.Ellipses[0].BasisB[0].SetInt64(999)
	after, _ := q.Ellipses[0].Eval(.3)
	if before != after {
		t.Fatal("mutable evaluator")
	}
	q, e = IntersectEqualRadiusCylinderBisectors(context.Background(), Vec3{}, Vec3{X: 1}, Vec3{Z: 2}, 1, SurfacePatchEvaluationOptions{})
	if e != ErrDomain || q.Complete {
		t.Fatal(q, e)
	}
	q, e = IntersectEqualRadiusCylinderBisectors(context.Background(), Vec3{}, u, u.Scale(-1), 1, SurfacePatchEvaluationOptions{})
	if e != ErrDegenerate || q.Complete {
		t.Fatal(q, e)
	}
	q, e = IntersectEqualRadiusCylinderBisectors(context.Background(), center, u, v, 2, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.Complete {
		t.Fatal(q, e)
	}
}
