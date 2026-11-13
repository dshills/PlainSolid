package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestQuadraticTrimAdaptiveExactEndpoints(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	curve, e := geom.NewBSpline(2, []geom.Vec3{{}, {X: .5, Y: 1}, {X: 1}}, []float64{2, 2, 2, 6, 6, 6}, []float64{1, .5, 1})
	if e != nil {
		t.Fatal(e)
	}
	trim := SplineUV{Curve: curve, Range: UVInterval{Min: 2, Max: 4}, Reversed: true}
	o := QuadraticTrimTessellationOptions{Deflection: .02}
	r, e := TessellateQuadraticSplineTrimCertified(context.Background(), patch, trim, o)
	if e != nil {
		t.Fatal(e)
	}
	expected := [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 6)}
	for i, p := range expected {
		if r.ExactPoints[0][i].Cmp(p) != 0 {
			t.Fatal("lost exact nonbinary endpoint", r.ExactPoints[0])
		}
	}
	if r.Parameters[0] != 4 || r.Parameters[len(r.Parameters)-1] != 2 || len(r.ExactUV) != len(r.Points) {
		t.Fatal("original parameter orientation")
	}
	for i, b := range r.SquaredErrors {
		if b.Cmp(big.NewRat(1, 2500)) > 0 {
			t.Fatal("excess bound")
		}
		if r.Parameters[i] <= r.Parameters[i+1] {
			t.Fatal("unordered reversed intervals")
		}
		// Independent rational Bézier composition at each segment midpoint.
		parameter := (r.Parameters[i] + r.Parameters[i+1]) / 2
		t0 := (parameter - 2) / 4
		denominator := (1-t0)*(1-t0) + t0*(1-t0) + t0*t0
		u := (.5*t0*(1-t0) + t0*t0) / denominator
		v := t0 * (1 - t0) / denominator
		actual := geom.Vec3{X: u, Y: v, Z: u * v}
		linear := r.Points[i].Add(r.Points[i+1]).Scale(.5)
		delta := actual.Sub(linear)
		upper, _ := b.Float64()
		if delta.Dot(delta) > upper {
			t.Fatal("independent point correspondence")
		}
	}
	for _, opt := range []QuadraticTrimTessellationOptions{{Deflection: .00001, MaxSegments: 1}, {Deflection: .00001, MaxChecks: 1}, {Deflection: .00001, MaxDepth: 1}, {Deflection: .02, Evaluation: geom.SurfacePatchEvaluationOptions{MaxOperations: 1}}} {
		bad, e := TessellateQuadraticSplineTrimCertified(context.Background(), patch, trim, opt)
		if e == nil || len(bad.Points) != 0 || len(bad.ExactPoints) != 0 {
			t.Fatal("partial failed proof", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = TessellateQuadraticSplineTrimCertified(ctx, patch, trim, o)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	t.Logf("%d chords, %d checks", len(r.SquaredErrors), r.Checks)
}
