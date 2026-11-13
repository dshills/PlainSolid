package geom

import (
	"errors"
	"math"
	"testing"
)

func TestOpenConicImplicitAndConversion(t *testing.T) {
	frame, _ := NewFrame(Vec3{2, -3, 5}, Vec3{1, 2, 0}, Vec3{1, -1, 3})
	p, _ := NewParabola(frame, 2)
	h, _ := NewHyperbola(frame, 3, 2)
	negative, _ := NewHyperbolaBranch(frame, 3, 2, -1)
	for _, curve := range []interface {
		Curve
		RationalBSpline(float64, float64) (*BSpline, error)
	}{p, h, negative} {
		s, e := curve.RationalBSpline(-.7, .8)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i <= 40; i++ {
			parameter := -.7 + 1.5*float64(i)/40
			if i == 40 {
				parameter = .8
			}
			point, e := curve.Eval(parameter)
			if e != nil {
				t.Fatal(e)
			}
			q, e := s.Eval(parameter)
			if e != nil || point.Distance(q) > 1e-12 {
				t.Fatal(point, q, e)
			}
			d, _ := curve.Derivative(parameter)
			sd, e := s.Derivative(parameter)
			if e != nil || d.Distance(sd) > 1e-10 {
				t.Fatal(d, sd, e)
			}
			local := frame.ToLocal(point)
			residual := local.X*local.X/9 - local.Y*local.Y/4 - 1
			if _, ok := curve.(Parabola); ok {
				residual = local.Y*local.Y - 8*local.X
			}
			if math.Abs(residual) > 1e-11 || math.Abs(local.Z) > 1e-12 {
				t.Fatal(local, residual)
			}
		}
	}
}
func TestOpenConicSecondDerivativeAndJoins(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	p, _ := NewParabola(f, 1)
	h, _ := NewHyperbola(f, 2, 1)
	for _, c := range []interface {
		Curve
		SecondDerivative(float64) (Vec3, error)
		RationalBSpline(float64, float64) (*BSpline, error)
	}{p, h} {
		for _, u := range []float64{-.6, 0, .6} {
			delta := 1e-5
			a, _ := c.Derivative(u + delta)
			b, _ := c.Derivative(u - delta)
			second, e := c.SecondDerivative(u)
			if e != nil || second.Distance(a.Sub(b).Scale(1/(2*delta))) > 1e-6 {
				t.Fatal(second, e)
			}
		}
		a, _ := c.RationalBSpline(-.7, .2)
		b, _ := c.RationalBSpline(.2, .8)
		ap, _ := a.Eval(.2)
		bp, _ := b.Eval(.2)
		ad, _ := a.Derivative(.2)
		bd, _ := b.Derivative(.2)
		if ap.Distance(bp) > 1e-13 || ad.Distance(bd) > 1e-12 {
			t.Fatal(ap, bp, ad, bd)
		}
	}
}
func TestOpenConicValidation(t *testing.T) {
	f, _ := NewFrame(Vec3{}, Vec3{X: 1}, Vec3{Z: 1})
	if _, e := NewParabola(f, 0); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := NewHyperbolaBranch(f, 1, 1, 0); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := NewHyperbola(Frame{}, 1, 1); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	h, _ := NewHyperbola(f, 1, 1)
	for _, u := range []float64{-1, 1, 2} {
		if _, e := h.Eval(u); !errors.Is(e, ErrDomain) {
			t.Fatal(e)
		}
	}
	if _, e := h.RationalBSpline(-1, .5); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	p, _ := NewParabola(f, 1)
	if _, e := p.RationalBSpline(1, 1); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	if _, e := p.Eval(math.MaxFloat64); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
