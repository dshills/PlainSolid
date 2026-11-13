package geom

import (
	"math"
	"math/rand"
	"sync"
	"testing"
)

// cardinalBasis evaluates a uniform B-spline using its independent truncated
// power representation, not the production de Boor recurrence.
func cardinalBasis(degree int, x float64) float64 {
	if x <= 0 || x >= float64(degree+1) {
		return 0
	}
	factorial := 1.0
	for i := 2; i <= degree; i++ {
		factorial *= float64(i)
	}
	sum, choose := 0.0, 1.0
	for k := 0; k <= degree+1; k++ {
		term := 0.0
		if x > float64(k) {
			term = choose * math.Pow(x-float64(k), float64(degree))
		}
		if k%2 == 1 {
			term = -term
		}
		sum += term
		choose *= float64(degree+1-k) / float64(k+1)
	}
	return sum / factorial
}
func periodicOracle(degree int, poles []Vec3, weights []float64, u float64) Vec3 {
	sum := Vec3{}
	denominator := 0.0
	for i := 0; i < len(poles)+degree; i++ {
		index := i % len(poles)
		basis := cardinalBasis(degree, u-float64(i)+float64(degree)) * weights[index]
		sum = sum.Add(poles[index].Scale(basis))
		denominator += basis
	}
	return sum.Scale(1 / denominator)
}
func TestPeriodicUniformOracleAndSeams(t *testing.T) {
	random := rand.New(rand.NewSource(22229))
	for trial := 0; trial < 50; trial++ {
		degree := 1 + random.Intn(5)
		count := degree + 1 + random.Intn(5)
		poles := make([]Vec3, count)
		weights := make([]float64, count)
		for i := range poles {
			poles[i] = Vec3{random.Float64()*4 - 2, random.Float64()*4 - 2, random.Float64()*4 - 2}
			weights[i] = .5 + 2*random.Float64()
		}
		start, period := -2.0, 8.0
		curve, e := NewPeriodicBSpline(degree, poles, weights, start, period)
		if e != nil {
			t.Fatal(e)
		}
		for sample := 0; sample < 30; sample++ {
			fraction := float64(sample) / 30
			t0 := start + period*fraction
			point, e := curve.Eval(t0)
			if e != nil {
				t.Fatal(e)
			}
			near(t, point, periodicOracle(degree, poles, weights, fraction*float64(count)), 2e-12)
			for _, cycle := range []float64{-7, -1, 1, 9} {
				shifted, e := curve.Eval(t0 + cycle*period)
				if e != nil {
					t.Fatal(e)
				}
				near(t, shifted, point, 2e-13)
				d, _ := curve.Derivative(t0)
				shiftedD, e := curve.Derivative(t0 + cycle*period)
				if e != nil {
					t.Fatal(e)
				}
				if degree > 1 || math.Abs(fraction*float64(count)-math.Round(fraction*float64(count))) > 1e-10 {
					near(t, shiftedD, d, 2e-12)
				}
			}
		}
		a, b := curve.Domain()
		p, _ := curve.Eval(a)
		q, _ := curve.Eval(b)
		near(t, p, q, 0)
		d, _ := curve.Derivative(a)
		de, _ := curve.Derivative(b)
		near(t, d, de, 0)
		if degree >= 2 {
			left, _ := curve.Derivative(math.Nextafter(b, a))
			near(t, left, d, 2e-13)
		}
		reversed, e := curve.Reversed()
		if e != nil {
			t.Fatal(e)
		}
		for _, fraction := range []float64{.13, .36, .73} {
			p, _ := curve.Eval(start + period*(1-fraction))
			q, _ := reversed.Eval(start + period*fraction)
			near(t, p, q, 2e-14)
			d, _ := curve.Derivative(start + period*(1-fraction))
			dr, _ := reversed.Derivative(start + period*fraction)
			near(t, dr, d.Scale(-1), 5e-14)
		}
	}
}
func TestPeriodicDerivativeAndCyclicShift(t *testing.T) {
	poles := []Vec3{{0, 0, 0}, {2, 0, 1}, {3, 2, 0}, {0, 3, -1}, {-1, 1, 0}}
	weights := []float64{1, 2, .5, 3, 1}
	curve, _ := NewPeriodicBSpline(3, poles, weights, 1, 5)
	shiftedPoles := append(append([]Vec3(nil), poles[1:]...), poles[0])
	shiftedWeights := append(append([]float64(nil), weights[1:]...), weights[0])
	shifted, _ := NewPeriodicBSpline(3, shiftedPoles, shiftedWeights, 1, 5)
	for _, u := range []float64{1.23, 2.1, 4.7, 5.9} {
		p, _ := curve.Eval(u + 1)
		q, _ := shifted.Eval(u)
		near(t, p, q, 1e-14)
		left, _ := curve.Eval(u - 1e-6)
		right, _ := curve.Eval(u + 1e-6)
		derivative, _ := curve.Derivative(u)
		near(t, derivative, right.Sub(left).Scale(5e5), 2e-8)
	}
	samples, e := curve.Sample(31)
	if e != nil || len(samples) != 31 {
		t.Fatal(e)
	}
	near(t, samples[0], samples[30], 0)
}
func TestPeriodicOwnershipAndConcurrentRead(t *testing.T) {
	poles := []Vec3{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}, {0, 1, 0}}
	weights := []float64{1, 2, 1, 2}
	curve, e := NewPeriodicBSpline(2, poles, weights, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	want, _ := curve.Eval(.2)
	poles[0] = Vec3{99, 99, 99}
	weights[0] = 99
	copyPoles, copyWeights := curve.Poles(), curve.Weights()
	copyPoles[0] = Vec3{}
	copyWeights[0] = 99
	var wait sync.WaitGroup
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for j := 0; j < 30; j++ {
				got, e := curve.Eval(.2)
				if e != nil || got.Distance(want) != 0 {
					t.Error(got, e)
				}
			}
		}()
	}
	wait.Wait()
}
func TestPeriodicScaleAndInvalid(t *testing.T) {
	poles := []Vec3{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}, {0, 1, 0}}
	for _, scale := range []float64{1e-300, 1e300} {
		scaled := make([]Vec3, len(poles))
		for i, p := range poles {
			scaled[i] = p.Scale(scale)
		}
		curve, e := NewPeriodicBSpline(2, scaled, nil, 0, scale)
		if e != nil {
			t.Fatal(e)
		}
		derivative, e := curve.Derivative(scale * .2)
		if e != nil || !derivative.IsFinite() || derivative.Length() == 0 {
			t.Fatal(derivative, e)
		}
	}
	curve, e := NewPeriodicBSpline(2, poles, nil, -1e308, 1e308)
	if e != nil {
		t.Fatal(e)
	}
	p, e := curve.Eval(1e308)
	q, _ := curve.Eval(-1e308)
	if e != nil {
		t.Fatal(e)
	}
	near(t, p, q, 0)
	for _, parameters := range [][2]float64{{0, 0}, {0, -1}, {1e16, 3}, {math.Inf(1), 1}, {0, math.SmallestNonzeroFloat64}} {
		if _, e := NewPeriodicBSpline(2, poles, nil, parameters[0], parameters[1]); e == nil {
			t.Fatal(parameters)
		}
	}
	if _, e := NewPeriodicBSpline(4, poles, nil, 0, 1); e == nil {
		t.Fatal("degree")
	}
	var zero PeriodicBSpline
	if _, e := zero.Eval(0); e == nil {
		t.Fatal("zero")
	}
	a, b := zero.Domain()
	if !math.IsNaN(a) || !math.IsNaN(b) {
		t.Fatal(a, b)
	}
}

func TestPeriodicIntegrationAndTessellation(t *testing.T) {
	curve, e := NewPeriodicBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, nil, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	length, e := ArcLength(curve, -1, 2, LengthOptions{})
	if e != nil || math.Abs(length.Length-12) > 1e-12 {
		t.Fatal(length, e)
	}
	segments, e := TessellateCurve(curve, 0, 1, CurveTessellationOptions{InitialSegments: 1})
	if e != nil || len(segments) != 4 {
		t.Fatal(segments, e)
	}
	if _, e = ArcLength(curve, 0, 1000000, LengthOptions{}); e == nil {
		t.Fatal("periodic work limit")
	}
	if _, e = TessellateCurve(curve, 0, 1000000, CurveTessellationOptions{}); e == nil {
		t.Fatal("periodic work limit")
	}
}
