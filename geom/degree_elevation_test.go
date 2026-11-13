package geom

import (
	"errors"
	"math/rand"
	"testing"
)

func TestRationalBezierDegreeElevation(t *testing.T) {
	random := rand.New(rand.NewSource(7919))
	for trial := 0; trial < 70; trial++ {
		degree := 1 + random.Intn(6)
		poles := make([]Vec3, degree+1)
		weights := make([]float64, degree+1)
		for i := range poles {
			poles[i] = Vec3{10*random.Float64() - 5, 10*random.Float64() - 5, 10*random.Float64() - 5}
			weights[i] = .1 + 10*random.Float64()
		}
		curve, e := NewBezier(poles, weights)
		if e != nil {
			t.Fatal(e)
		}
		target := degree + 1 + random.Intn(4)
		elevated, e := curve.ElevateDegree(target)
		if e != nil || elevated.Degree() != target || len(elevated.Poles()) != target+1 {
			t.Fatal(e)
		}
		for sample := 0; sample <= 40; sample++ {
			u := float64(sample) / 40
			got, e := elevated.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			near(t, got, bernsteinPoint(poles, weights, u), 2e-12)
			d, e := curve.Derivative(u)
			if e != nil {
				t.Fatal(e)
			}
			de, e := elevated.Derivative(u)
			if e != nil {
				t.Fatal(e)
			}
			near(t, de, d, 2e-10)
		}
	}
}
func TestGeneralDegreeElevation(t *testing.T) {
	circle, _ := NewCircle(Vec3{1, 2, 3}, Vec3{0, 0, 1}, 2)
	curve, _ := circle.RationalBSpline()
	elevated, e := curve.ElevateDegree(5)
	if e != nil || len(elevated.Poles()) != 21 {
		t.Fatal(e)
	}
	for sample := 0; sample <= 200; sample++ {
		u := float64(sample) / 50
		p, _ := curve.Eval(u)
		q, e := elevated.Eval(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, q, p, 5e-15)
	}
	smooth, e := NewBSpline(3, []Vec3{{0, 0, 0}, {1, 2, 1}, {2, -1, 2}, {3, 2, 1}, {4, 0, 1}, {5, 1, 0}}, []float64{0, 0, 0, 0, .3, .7, 1, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	elevated, e = smooth.ElevateDegree(4)
	if e != nil {
		t.Fatal(e)
	}
	for sample := 0; sample <= 200; sample++ {
		u := float64(sample) / 200
		p, _ := smooth.Eval(u)
		q, e := elevated.Eval(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, q, p, 1e-13)
		d, _ := smooth.Derivative(u)
		de, e := elevated.Derivative(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, de, d, 2e-12)
	}
	for _, k := range []float64{.3, .7} {
		multiplicity := 0
		for _, v := range elevated.Knots() {
			if v == k {
				multiplicity++
			}
		}
		if multiplicity != 4 {
			t.Fatal(multiplicity)
		}
	}
	same, e := smooth.ElevateDegree(3)
	if e != nil || len(same.Poles()) != len(smooth.Poles()) {
		t.Fatal(e)
	}
}
func TestDegreeElevationLimits(t *testing.T) {
	curve, _ := NewBezier([]Vec3{{0, 0, 0}, {1, 1, 0}, {2, 0, 0}}, nil)
	if _, e := curve.ElevateDegree(1); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := curve.ElevateDegree(65); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := curve.ElevateDegreeWithOptions(5, DegreeElevationOptions{MaxPoles: 4}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	unclamped, _ := NewBSpline(2, curve.Poles(), []float64{-2, -1, 0, 1, 2, 3}, nil)
	if _, e := unclamped.ElevateDegree(3); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	jump, _ := NewBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {2, 1, 0}, {3, 1, 0}}, []float64{0, 0, 1, 1, 2, 2}, nil)
	if _, e := jump.ElevateDegree(2); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
