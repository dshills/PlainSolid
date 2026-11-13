package geom

import (
	"context"
	"math"
	"math/big"
	"testing"
)

func TestSphereToolEquator(t *testing.T) {
	for _, d := range []float64{0, .5, 2, 3.875} {
		c, e := NewSphereToolEquator(context.Background(), 4, d, SurfacePatchEvaluationOptions{})
		if e != nil || !c.Proven {
			t.Fatal(c, e)
		}
		lo, _ := c.SphereAngleFirst[0].Float64()
		hi, _ := c.SphereAngleFirst[1].Float64()
		second, _ := c.SphereAngleSecondAbsolute.Float64()
		sum := 0.0
		for i := 0; i < 128; i++ {
			theta := 2 * math.Pi * (float64(i) + .5) / 128
			p, phi, a, b, e := c.Evaluate(theta)
			if e != nil || math.Abs(p.X*p.X+p.Y*p.Y-16) > 1e-12 || a < lo-1e-12 || a > hi+1e-12 || math.Abs(b) > second+1e-12 {
				t.Fatal(theta, p, a, b, e)
			}
			inv, e := c.Inverse(phi)
			if e != nil || math.Abs(math.Remainder(inv-theta, 2*math.Pi)) > 1e-12 {
				t.Fatal(inv, theta, e)
			}
			eps := 1e-6
			_, _, am, _, _ := c.Evaluate(theta - eps)
			_, _, ap, _, _ := c.Evaluate(theta + eps)
			if math.Abs((ap-am)/(2*eps)-b) > 1e-6 {
				t.Fatal(theta, b)
			}
			sum += a * 2 * math.Pi / 128
		}
		if math.Abs(sum-2*math.Pi) > 1e-6 {
			t.Fatal("degree", d, sum)
		}
	}
	c, e := NewSphereToolEquator(context.Background(), 4, 0, SurfacePatchEvaluationOptions{})
	if e != nil || c.SphereAngleFirst[0].Cmp(big.NewRat(1, 1)) != 0 || c.SphereAngleFirst[1].Cmp(big.NewRat(1, 1)) != 0 || c.SphereAngleSecondAbsolute.Sign() != 0 {
		t.Fatal(c, e)
	}
	c, e = NewSphereToolEquator(context.Background(), 4, 4, SurfacePatchEvaluationOptions{})
	if e != ErrDomain || c.Proven {
		t.Fatal(c, e)
	}
	c, e = NewSphereToolEquator(context.Background(), 4, 2, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || c.Proven {
		t.Fatal(c, e)
	}
}
