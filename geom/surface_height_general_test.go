package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestGeneralSurfaceHeightDifferentDegreesAndKnots(t *testing.T) {
	as, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 1}, {0, 1, 1}, {1, 0, 1}, {1, 1, 1}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	as, _ = as.InsertUKnot(.5, 1)
	poles := make([]Vec3, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.0
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles[3*i+j] = Vec3{float64(i) / 2, float64(j) / 2, z}
		}
	}
	bs, _ := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	bs, _ = bs.InsertVKnot(.25, 1)
	a, _ := NewSurfacePatch(as, SurfacePatchOptions{})
	b, _ := NewSurfacePatch(bs, SurfacePatchOptions{})
	difference, e := NewGeneralSurfaceHeightDifferencePatch(context.Background(), a, b, GeneralSurfaceHeightOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if difference.degreeU != 3 || difference.degreeV != 3 || difference.countU != 7 || difference.countV != 7 {
		t.Fatal(difference.degreeU, difference.degreeV, difference.countU, difference.countV)
	}
	for _, uv := range []Vec2{{0, 0}, {1, 1}, {.5, .25}, {.125, .75}, {.75, .875}} {
		budget := &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
		pa, e := exactPatchPointAt(a, uv.X, uv.Y, budget)
		if e != nil {
			t.Fatal(e)
		}
		pb, e := exactPatchPointAt(b, uv.X, uv.Y, budget)
		if e != nil {
			t.Fatal(e)
		}
		pd, e := exactPatchPointAt(difference, uv.X, uv.Y, budget)
		if e != nil {
			t.Fatal(e)
		}
		if pd[0].Cmp(pa[0]) != 0 || pd[1].Cmp(pa[1]) != 0 || pd[2].Cmp(new(big.Rat).Sub(pa[2], pb[2])) != 0 {
			t.Fatal(uv, pa, pb, pd)
		}
	}
}
func TestGeneralSurfaceHeightNonproportionalWeights(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 1} })
	source := &BSplineSurface{degreeU: 2, degreeV: 1, countU: 3, countV: 2, knotsU: []float64{0, 0, 0, 1, 1, 1}, knotsV: []float64{0, 0, 1, 1}, weights: []float64{1, 1, 2, 2, 3, 3}, poles: []Vec3{{0, 0, 0}, {0, 1, 0}, {.25, 0, .25}, {.25, 1, .25}, {1, 0, 1}, {1, 1, 1}}}
	b, _ := NewSurfacePatch(source, SurfacePatchOptions{})
	difference, e := NewGeneralSurfaceHeightDifferencePatch(context.Background(), a, b, GeneralSurfaceHeightOptions{})
	if e != nil {
		t.Fatal(e)
	}
	expected := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 1 - u} })
	proof, e := CompareExactSurfaces(context.Background(), difference, expected, ExactSurfaceComparisonOptions{})
	if e != nil || !proof.Equal {
		t.Fatal(proof, e)
	}
	for _, h := range difference.net {
		if h[3].Sign() <= 0 {
			t.Fatal("nonpositive product weight")
		}
	}
}
func TestGeneralSurfaceHeightExactSubtractionAndErrors(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 1} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, math.Ldexp(1, -54)} })
	p, e := NewGeneralSurfaceHeightDifferencePatch(context.Background(), a, b, GeneralSurfaceHeightOptions{})
	if e != nil {
		t.Fatal(e)
	}
	exact := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).SetFloat64(math.Ldexp(1, -54)))
	for _, h := range p.net {
		if new(big.Rat).Quo(h[2], h[3]).Cmp(exact) != 0 {
			t.Fatal(h)
		}
	}
	_, e = p.ToBSplineSurface()
	if !errors.Is(e, ErrUnrepresentable) {
		t.Fatal(e)
	}
	mismatch := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u + 1, v, 0} })
	_, e = NewGeneralSurfaceHeightDifferencePatch(context.Background(), a, mismatch, GeneralSurfaceHeightOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	_, e = NewGeneralSurfaceHeightDifferencePatch(context.Background(), a, b, GeneralSurfaceHeightOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = NewGeneralSurfaceHeightDifferencePatch(ctx, a, b, GeneralSurfaceHeightOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
