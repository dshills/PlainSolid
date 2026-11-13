package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestQuadraticTrimCorrespondenceIndependentParabola(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	curve, e := geom.NewBSpline(2, []geom.Vec3{{}, {X: .5, Y: .5}, {X: 1}}, []float64{2, 2, 2, 6, 6, 6}, nil)
	if e != nil {
		t.Fatal(e)
	}
	trim := SplineUV{Curve: curve, Range: UVInterval{Min: 2, Max: 6}}
	bound, e := BoundQuadraticSplineTrimCorrespondence(context.Background(), patch, trim, geom.Vec3{}, geom.Vec3{X: 1})
	// Exact curve (t,t-t²,0), with maximum sag 1/4. Degree-nine
	// Bernstein controls have largest sag 5/18, giving square 25/324.
	if e != nil || bound.Cmp(big.NewRat(25, 324)) != 0 {
		t.Fatal(bound, e)
	}
	trim.Range.Max = 4
	half, e := BoundQuadraticSplineTrimCorrespondence(context.Background(), patch, trim, geom.Vec3{}, geom.Vec3{X: .5, Y: .25})
	if e != nil || half.Cmp(new(big.Rat).Quo(bound, big.NewRat(16, 1))) != 0 {
		t.Fatal("quadratic convergence", half, e)
	}
	trim.Reversed = true
	reverse, e := BoundQuadraticSplineTrimCorrespondence(context.Background(), patch, trim, geom.Vec3{X: .5, Y: .25}, geom.Vec3{})
	if e != nil || reverse.Cmp(half) != 0 {
		t.Fatal("nonunit reversed range", reverse, e)
	}
	shifted, e := BoundQuadraticSplineTrimCorrespondence(context.Background(), patch, trim, geom.Vec3{X: .5, Y: .25, Z: .125}, geom.Vec3{Z: .125})
	if e != nil || shifted.Cmp(new(big.Rat).Add(half, big.NewRat(1, 64))) != 0 {
		t.Fatal("endpoint displacement not included", shifted, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	failed, e := BoundQuadraticSplineTrimCorrespondence(ctx, patch, trim, geom.Vec3{}, geom.Vec3{})
	if !errors.Is(e, context.Canceled) || failed != nil {
		t.Fatal("cancellation", e)
	}
}

func TestQuadraticTrimCorrespondenceRationalComposition(t *testing.T) {
	k := []float64{0, 0, 1, 1}
	poles := []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}
	weights := []float64{1, .75, .5, 1}
	surface, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, k, k, weights)
	if e != nil {
		t.Fatal(e)
	}
	patch, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	uvp := []geom.Vec3{{X: .125, Y: .25}, {X: .5, Y: .75}, {X: .875, Y: .25}}
	curve, e := geom.NewBSpline(2, uvp, []float64{0, 0, 0, 1, 1, 1}, []float64{1, .75, 1})
	if e != nil {
		t.Fatal(e)
	}
	independent := func(t float64) geom.Vec3 {
		c := []float64{(1 - t) * (1 - t), 1.5 * t * (1 - t), t * t}
		u, v, w := 0., 0., 0.
		for i, a := range c {
			u += a * uvp[i].X
			v += a * uvp[i].Y
			w += a
		}
		u /= w
		v /= w
		b := []float64{(1 - u) * (1 - v), (1 - u) * v, u * (1 - v), u * v}
		p := geom.Vec3{}
		w = 0
		for i, a := range b {
			p = p.Add(poles[i].Scale(a * weights[i]))
			w += a * weights[i]
		}
		return p.Scale(1 / w)
	}
	trim := SplineUV{Curve: curve, Range: UVInterval{Min: .25, Max: .75}, Reversed: true}
	a, b := independent(.75), independent(.25)
	bound, e := BoundQuadraticSplineTrimCorrespondence(context.Background(), patch, trim, a, b)
	if e != nil {
		t.Fatal(e)
	}
	upper, _ := bound.Float64()
	for i := 0; i <= 128; i++ {
		t0 := float64(i) / 128
		point := independent(.75 - .5*t0)
		linear := a.Scale(1 - t0).Add(b.Scale(t0))
		delta := point.Sub(linear)
		if delta.Dot(delta) > upper {
			t.Fatal("independent composed residual", i, delta.Dot(delta), upper)
		}
	}
}
