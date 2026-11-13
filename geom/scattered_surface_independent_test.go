package geom

import (
	"context"
	"math"
	"testing"
)

// The oracle is a directly authored rational biquadratic Bernstein sum,
// independent of the B-spline evaluator and fitter basis implementation.
func TestIndependentScatteredNonseparableQuadratic(t *testing.T) {
	weights := []float64{1, .25, 2, .5, 3, 1.25, 2, .75, 1}
	poles := []Vec3{{X: 100, Y: -20}, {X: 100, Y: -19, Z: 1}, {X: 100, Y: -18, Z: -1}, {X: 101, Y: -20, Z: 2}, {X: 101, Y: -19, Z: -2}, {X: 101, Y: -18, Z: 1}, {X: 102, Y: -20, Z: 1}, {X: 102, Y: -19, Z: 3}, {X: 102, Y: -18}}
	value := func(u, v float64) Vec3 {
		a, b := [3]float64{(1 - u) * (1 - u), 2 * u * (1 - u), u * u}, [3]float64{(1 - v) * (1 - v), 2 * v * (1 - v), v * v}
		sum := Vec3{}
		den := 0.
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				w := a[i] * b[j] * weights[3*i+j]
				sum = sum.Add(poles[3*i+j].Scale(w))
				den += w
			}
		}
		return sum.Scale(1 / den)
	}
	var samples []SurfaceSample
	for i := 0; i < 53; i++ {
		u := float64((i*17)%53) / 52
		v := float64((i*i*7+3*i)%59) / 58
		samples = append(samples, SurfaceSample{UV: Vec2{X: u, Y: v}, Point: value(u, v), Weight: float64(1 + i%7)})
	}
	k := []float64{0, 0, 0, 1, 1, 1}
	for _, factor := range []float64{1, math.Ldexp(1, 400), math.Ldexp(1, -400)} {
		scaled := append([]SurfaceSample{}, samples...)
		for i := range scaled {
			scaled[i].Weight *= factor
		}
		fit, d, e := ApproximateScatteredBSplineSurface(context.Background(), scaled, ScatteredSurfaceOptions{DegreeU: 2, DegreeV: 2, CountU: 3, CountV: 3, KnotsU: k, KnotsV: k, Weights: weights, MaxResidual: 1e-10})
		if e != nil || d.Rank != 9 || d.WeightedSamples != len(samples) {
			t.Fatal(d, e)
		}
		for i := 0; i < 13; i++ {
			for j := 0; j < 11; j++ {
				u, v := float64(i)/12, float64(j)/10
				p, e := fit.Eval(u, v)
				if e != nil || p.Distance(value(u, v)) > 2e-10 {
					t.Fatal("independent rational fit", factor, u, v, p, value(u, v), e)
				}
			}
		}
	}
}
