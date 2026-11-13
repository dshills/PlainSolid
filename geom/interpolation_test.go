package geom

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

func cubicPoint(t float64) Vec3 { return Vec3{t, t * t, t * t * t} }
func TestInterpolationPolynomialOracle(t *testing.T) {
	parameters := []float64{-2, -1.9, -1, -.2, .7, 1.3, 3}
	points := make([]Vec3, len(parameters))
	for i, t := range parameters {
		points[i] = cubicPoint(t)
	}
	curve, diagnostics, e := Interpolate(points, InterpolationOptions{Parameters: parameters})
	if e != nil || diagnostics.MaxResidual > 1e-12 || diagnostics.ConditionEstimate < 1 || diagnostics.PivotRatio <= 0 {
		t.Fatal(diagnostics, e)
	}
	a, b := curve.Domain()
	if a != -2 || b != 3 {
		t.Fatal(a, b)
	}
	for i := 0; i <= 100; i++ {
		u := -2 + 5*float64(i)/100
		p, e := curve.Eval(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, p, cubicPoint(u), 2e-12)
	}
	parameters[0] = 999
	points[0] = Vec3{}
	if diagnostics.Parameters[0] != -2 {
		t.Fatal("parameter alias")
	}
}
func TestInterpolationRandomAndAffineInvariant(t *testing.T) {
	random := rand.New(rand.NewSource(110917))
	transform := Affine{{2, .5, 0, 3}, {0, -1, .25, -2}, {.2, 0, 3, 5}}
	for trial := 0; trial < 40; trial++ {
		count := 4 + random.Intn(8)
		parameters := make([]float64, count)
		points := make([]Vec3, count)
		transformed := make([]Vec3, count)
		for i := range parameters {
			parameters[i] = float64(i) / float64(count-1)
			points[i] = Vec3{random.Float64() * 10, random.Float64() * 10, random.Float64() * 10}
			transformed[i] = transform.Apply(points[i])
		}
		curve, _, e := Interpolate(points, InterpolationOptions{Parameters: parameters})
		if e != nil {
			t.Fatal(e)
		}
		other, _, e := Interpolate(transformed, InterpolationOptions{Parameters: parameters})
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i <= 40; i++ {
			u := float64(i) / 40
			p, _ := curve.Eval(u)
			q, e := other.Eval(u)
			if e != nil {
				t.Fatal(e)
			}
			near(t, q, transform.Apply(p), 1e-12)
		}
	}
}
func TestChordInterpolationAndLinear(t *testing.T) {
	points := []Vec3{{0, 0, 0}, {1, 2, 0}, {2, 1, 1}, {4, 3, 0}, {5, 0, 1}}
	curve, d, e := Interpolate(points, InterpolationOptions{})
	if e != nil {
		t.Fatal(d, e)
	}
	for i, u := range d.Parameters {
		p, e := curve.Eval(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, p, points[i], 1e-12)
	}
	line, d, e := Interpolate(points[:2], InterpolationOptions{})
	if e != nil || line.Degree() != 1 || math.Abs(d.ConditionEstimate-1) > 1e-15 {
		t.Fatal(d, e)
	}
	constant := []Vec3{{2, 3, 4}, {2, 3, 4}, {2, 3, 4}, {2, 3, 4}}
	curve, d, e = Interpolate(constant, InterpolationOptions{Parameters: []float64{0, .2, .7, 1}})
	if e != nil {
		t.Fatal(d, e)
	}
	p, _ := curve.Eval(.4)
	near(t, p, constant[0], 1e-15)
}
func TestInterpolationFailures(t *testing.T) {
	points := []Vec3{{0, 0, 0}, {1, 2, 0}, {2, 1, 0}, {3, 2, 0}, {4, 0, 0}}
	for _, options := range []InterpolationOptions{{Parameters: []float64{0, 0, 1, 2, 3}}, {Parameters: []float64{0, 1}}, {Degree: 5}, {ConditionLimit: -1}} {
		if _, _, e := Interpolate(points, options); e == nil {
			t.Fatal(options)
		}
	}
	if _, _, e := Interpolate(points, InterpolationOptions{MaxPoints: 4}); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	_, d, e := Interpolate(points, InterpolationOptions{Parameters: []float64{0, 1e-14, 2e-14, .5, 1}})
	if !errors.Is(e, ErrIllConditioned) || d.ConditionEstimate <= 1e12 {
		t.Fatal(d, e)
	}
	if _, _, e := Interpolate([]Vec3{{}, {}, {}}, InterpolationOptions{}); !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	if _, _, e := Interpolate([]Vec3{{math.NaN(), 0, 0}, {}}, InterpolationOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
