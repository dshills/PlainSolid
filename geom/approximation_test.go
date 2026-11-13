package geom

import (
	"errors"
	"math"
	"math/rand"
	"testing"
)

func TestApproximationCubicOracle(t *testing.T) {
	parameters := make([]float64, 31)
	points := make([]Vec3, len(parameters))
	weights := make([]float64, len(parameters))
	for i := range parameters {
		parameters[i] = -1 + 3*float64(i)/float64(len(parameters)-1)
		points[i] = cubicPoint(parameters[i])
		weights[i] = float64(i%5 + 1)
	}
	curve, diagnostics, e := ApproximateBSpline(points, ApproximationOptions{PoleCount: 4, Degree: 3, Parameters: parameters, SampleWeights: weights, MaxResidual: 1e-11})
	if e != nil || diagnostics.Rank != 4 || diagnostics.MaxResidual > 1e-11 || diagnostics.RConditionEstimate < 1 {
		t.Fatal(diagnostics, e)
	}
	for i := 0; i <= 100; i++ {
		u := -1 + 3*float64(i)/100
		point, e := curve.Eval(u)
		if e != nil {
			t.Fatal(e)
		}
		near(t, point, cubicPoint(u), 3e-12)
	}
}
func TestApproximationFixedRationalWeights(t *testing.T) {
	weights := []float64{1, math.Sqrt(.5), 1}
	poles := []Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}
	parameters := make([]float64, 41)
	points := make([]Vec3, len(parameters))
	for i := range parameters {
		parameters[i] = float64(i) / float64(len(parameters)-1)
		points[i] = bernsteinPoint(poles, weights, parameters[i])
	}
	curve, diagnostics, e := ApproximateBSpline(points, ApproximationOptions{Degree: 2, PoleCount: 3, Parameters: parameters, Weights: weights, MaxResidual: 1e-12})
	if e != nil {
		t.Fatal(diagnostics, e)
	}
	for i, p := range curve.Poles() {
		near(t, p, poles[i], 2e-14)
	}
	for i := 0; i <= 100; i++ {
		p, _ := curve.Eval(float64(i) / 100)
		if math.Abs(p.Length()-1) > 2e-14 {
			t.Fatal(p)
		}
	}
}
func TestApproximationWeightedRegressionOracle(t *testing.T) {
	parameters := []float64{0, .07, .2, .39, .6, .72, .91, 1}
	points := make([]Vec3, len(parameters))
	weights := []float64{1, 2, 7, 1, 4, 2, 1, 9}
	for i, u := range parameters {
		points[i] = Vec3{u, 2 + 3*u + .2*math.Sin(float64(i)*2), -1 + 4*u + .1*math.Cos(float64(i)*3)}
	}
	curve, diagnostics, e := ApproximateBSpline(points, ApproximationOptions{Degree: 1, PoleCount: 2, Parameters: parameters, SampleWeights: weights})
	if e != nil {
		t.Fatal(diagnostics, e)
	}
	sumWeights, meanParameter := 0.0, 0.0
	mean := Vec3{}
	for i, w := range weights {
		sumWeights += w
		meanParameter += w * parameters[i]
		mean = mean.Add(points[i].Scale(w))
	}
	meanParameter /= sumWeights
	mean = mean.Scale(1 / sumWeights)
	numerator := Vec3{}
	denominator := 0.0
	for i, w := range weights {
		delta := parameters[i] - meanParameter
		numerator = numerator.Add(points[i].Sub(mean).Scale(w * delta))
		denominator += w * delta * delta
	}
	slope := numerator.Scale(1 / denominator)
	intercept := mean.Sub(slope.Scale(meanParameter))
	for _, u := range []float64{0, .3, .8, 1} {
		point, _ := curve.Eval(u)
		near(t, point, intercept.Add(slope.Scale(u)), 1e-13)
	}
	if diagnostics.MaxResidual <= 0 || diagnostics.RMSResidual <= 0 || diagnostics.WeightedRMSResidual <= 0 {
		t.Fatal(diagnostics)
	}
}
func TestApproximationResidualOrthogonality(t *testing.T) {
	parameters := make([]float64, 35)
	points := make([]Vec3, len(parameters))
	for i := range parameters {
		u := float64(i) / float64(len(parameters)-1)
		parameters[i] = u
		points[i] = Vec3{u, math.Sin(5 * u), math.Cos(3 * u)}
	}
	curve, diagnostics, e := ApproximateBSpline(points, ApproximationOptions{Degree: 2, PoleCount: 3, Parameters: parameters})
	if e != nil {
		t.Fatal(diagnostics, e)
	}
	sums := [3]Vec3{}
	for i, u := range parameters {
		point, _ := curve.Eval(u)
		residual := point.Sub(points[i])
		basis := []float64{(1 - u) * (1 - u), 2 * u * (1 - u), u * u}
		for j, b := range basis {
			sums[j] = sums[j].Add(residual.Scale(b))
		}
	}
	for _, sum := range sums {
		if sum.Length() > 2e-13 {
			t.Fatal(sum)
		}
	}
	if curve, diagnostics, e = ApproximateBSpline(points, ApproximationOptions{Degree: 2, PoleCount: 3, Parameters: parameters, MaxResidual: 1e-8}); curve != nil || !errors.Is(e, ErrConvergence) || diagnostics.MaxResidual <= 1e-8 {
		t.Fatal(diagnostics, e)
	}
}
func TestApproximationAffineInvariance(t *testing.T) {
	random := rand.New(rand.NewSource(27139))
	transform := Affine{{2, .5, 0, 7}, {0, -1, .3, -4}, {.1, 0, 3, 2}}
	for trial := 0; trial < 20; trial++ {
		parameters := make([]float64, 30)
		points, transformed := make([]Vec3, 30), make([]Vec3, 30)
		for i := range parameters {
			parameters[i] = float64(i) / 29
			points[i] = Vec3{random.Float64() * 10, random.Float64() * 10, random.Float64() * 10}
			transformed[i] = transform.Apply(points[i])
		}
		a, _, e := ApproximateBSpline(points, ApproximationOptions{PoleCount: 5, Parameters: parameters})
		if e != nil {
			t.Fatal(e)
		}
		b, _, e := ApproximateBSpline(transformed, ApproximationOptions{PoleCount: 5, Parameters: parameters})
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i <= 20; i++ {
			u := float64(i) / 20
			p, _ := a.Eval(u)
			q, _ := b.Eval(u)
			near(t, q, transform.Apply(p), 2e-12)
		}
	}
}
func TestApproximationParametersAndFailures(t *testing.T) {
	points := []Vec3{{0, 0, 0}, {0, 0, 0}, {1, 1, 0}, {2, 0, 0}, {2, 0, 0}}
	curve, diagnostics, e := ApproximateBSpline(points, ApproximationOptions{Degree: 2, PoleCount: 3})
	if e != nil || curve == nil || diagnostics.Parameters[0] != diagnostics.Parameters[1] {
		t.Fatal(diagnostics, e)
	}
	rankDeficient := []float64{0, 0, 0, 1, 1}
	if _, _, e := ApproximateBSpline(points, ApproximationOptions{Degree: 2, PoleCount: 3, Parameters: rankDeficient}); !errors.Is(e, ErrIllConditioned) {
		t.Fatal(e)
	}
	for _, options := range []ApproximationOptions{{PoleCount: 3, MaxWork: 1}, {PoleCount: 3, MaxSamples: 4}, {PoleCount: 3, MaxPoles: 2}} {
		if _, _, e := ApproximateBSpline(points, options); !errors.Is(e, ErrConvergence) {
			t.Fatal(options, e)
		}
	}
	for _, options := range []ApproximationOptions{{PoleCount: 1}, {PoleCount: 3, Weights: []float64{1, -1, 1}}, {PoleCount: 3, Parameters: []float64{0, .5, .3, .8, 1}}, {PoleCount: 3, SampleWeights: []float64{1, 1, 0, 1, 1}}} {
		if _, _, e := ApproximateBSpline(points, options); e == nil {
			t.Fatal(options)
		}
	}
	if _, _, e := ApproximateBSpline([]Vec3{{}, {}, {}}, ApproximationOptions{PoleCount: 2}); !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
}

func TestApproximationCustomRationalSpace(t *testing.T) {
	knots := []float64{0, 0, 0, 0, .3, .7, 1, 1, 1, 1}
	weights := []float64{1, 2, .5, 3, 2, 1}
	poles := []Vec3{{0, 0, 0}, {1, 2, 1}, {2, -1, 3}, {3, 2, 2}, {4, -2, 0}, {5, 0, 1}}
	source, e := NewBSpline(3, poles, knots, weights)
	if e != nil {
		t.Fatal(e)
	}
	parameters := make([]float64, 80)
	points := make([]Vec3, len(parameters))
	for i := range points {
		parameters[i] = float64(i) / float64(len(points)-1)
		points[i], _ = source.Eval(parameters[i])
	}
	curve, diagnostics, e := ApproximateBSpline(points, ApproximationOptions{Degree: 3, PoleCount: 6, Knots: knots, Weights: weights, Parameters: parameters, MaxResidual: 1e-11})
	if e != nil {
		t.Fatal(diagnostics, e)
	}
	for i, p := range curve.Poles() {
		near(t, p, poles[i], 1e-12)
	}
	if _, diagnostics, e = ApproximateBSpline(points, ApproximationOptions{Degree: 3, PoleCount: 6, Knots: knots, Weights: weights, Parameters: parameters, RConditionLimit: 1.01}); !errors.Is(e, ErrIllConditioned) || diagnostics.RConditionEstimate <= 1.01 {
		t.Fatal(diagnostics, e)
	}
}
