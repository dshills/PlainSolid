package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestExactSurfaceComparisonDegreeKnotsAndDomain(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	// Bilinear surface represented as biquadratic Bernstein coefficients.
	poles := make([]Vec3, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := float64(i)/2, float64(j)/2
			poles[i*3+j] = Vec3{u, v, u * v}
		}
	}
	s, _ := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	s, _ = s.InsertUKnot(.25, 1)
	s, _ = s.InsertVKnot(.5, 1)
	b, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	r, e := CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{})
	if e != nil || !r.Equal || r.ComparedCells != 4 {
		t.Fatal(r, e)
	}
	b, e = b.Reparameterize(ParameterRectangle{UMin: 2, UMax: 6, VMin: -4, VMax: 2}, false, false, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e = CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true})
	if e != nil || !r.Equal {
		t.Fatal(r, e)
	}
	_, e = CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
func TestExactSurfaceComparisonChartSymmetries(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, 2 * v, u * v} })
	for _, swap := range []bool{false, true} {
		for _, ru := range []bool{false, true} {
			for _, rv := range []bool{false, true} {
				b := arcTestPatch(t, func(u, v float64) Vec3 {
					// B's original coordinates map to aligned U,V after swapping and reversal.
					if swap {
						u, v = v, u
					}
					if ru {
						u = 1 - u
					}
					if rv {
						v = 1 - v
					}
					return Vec3{u, 2 * v, u * v}
				})
				r, e := CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{SwapB: swap, ReverseBU: ru, ReverseBV: rv})
				if e != nil || !r.Equal {
					t.Fatal(swap, ru, rv, r, e)
				}
			}
		}
	}
}
func TestExactSurfaceComparisonNegativeAndLimits(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, math.Ldexp(1, -100) * u * v} })
	r, e := CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{})
	if e != nil || !r.Complete || r.Equal || r.DifferentAxis != 2 {
		t.Fatal(r, e)
	}
	r, e = CompareExactSurfaces(context.Background(), a, a, ExactSurfaceComparisonOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = CompareExactSurfaces(ctx, a, a, ExactSurfaceComparisonOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}

func TestExactSurfaceComparisonRationalMultiplierAndNonbinaryGrid(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	s := &BSplineSurface{degreeU: 2, degreeV: 1, countU: 3, countV: 2, knotsU: []float64{0, 0, 0, 1, 1, 1}, knotsV: []float64{0, 0, 1, 1}, weights: []float64{1, 1, 2, 2, 3, 3}, poles: []Vec3{{0, 0, 0}, {0, 1, 0}, {.25, 0, 0}, {.25, 1, .25}, {1, 0, 0}, {1, 1, 1}}}
	b, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{})
	if e != nil || !r.Equal {
		t.Fatal(r, e)
	}
	a = arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{3 * u, v, 3 * u * v} })
	s, e = NewBSplineSurface(1, 1, 3, 2, []Vec3{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}, {1, 1, 1}, {3, 0, 0}, {3, 1, 3}}, []float64{2, 2, 3, 5, 5}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = NewSurfacePatch(s, SurfacePatchOptions{})
	r, e = CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true})
	if e != nil || !r.Equal || r.ComparedCells != 2 {
		t.Fatal(r, e)
	}
	r, e = CompareExactSurfaces(context.Background(), a, b, ExactSurfaceComparisonOptions{NormalizeDomains: true, MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
}

func TestExactSurfaceComparisonFractionalOriginalKnotMaps(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, 2 * v, u * v} })
	makeB := func(perturb bool) *SurfacePatch {
		t.Helper()
		// On B, the original U is A's second station, and B's decreasing
		// V is A's first. Evaluate the analytically known homogeneous
		// bilinear polynomial at the original knot grid. Degree-one
		// controls interpolate this polynomial exactly at those knots.
		p := &SurfacePatch{degreeU: 1, degreeV: 1, countU: 3, countV: 3,
			knotsU: []float64{-2, -2, 0, 2, 2}, knotsV: []float64{4, 4, 7, 8, 8}}
		one, two := big.NewRat(1, 1), big.NewRat(2, 1)
		for i, v := range [3]*big.Rat{new(big.Rat), big.NewRat(1, 2), one} {
			for j, u := range [3]*big.Rat{one, big.NewRat(1, 4), new(big.Rat)} {
				wu := new(big.Rat).Sub(big.NewRat(3, 2), new(big.Rat).Quo(u, two))
				wv := new(big.Rat).Sub(big.NewRat(4, 3), new(big.Rat).Quo(v, big.NewRat(3, 1)))
				h := exactHomogeneous{new(big.Rat).Mul(u, wv), new(big.Rat).Mul(two, new(big.Rat).Mul(v, wu)),
					new(big.Rat).Mul(u, v), new(big.Rat).Mul(wu, wv)}
				if perturb && i == 1 && j == 1 {
					h[2].Add(h[2], new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120)))
				}
				p.net = append(p.net, h)
			}
		}
		p.options, _ = patchOptions(SurfacePatchOptions{})
		if err := p.computeBounds(&polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}); err != nil {
			t.Fatal(err)
		}
		return p
	}
	b := makeB(false)
	// B's original (U,V) knot intersection pulls back to A (2/11,3/7).
	jet, err := b.EvaluateRational(ctx, new(big.Rat), big.NewRat(7, 1), SurfacePatchEvaluationOptions{})
	if err != nil || jet.Point[0].Cmp(big.NewRat(2, 11)) != 0 || jet.Point[1].Cmp(big.NewRat(6, 7)) != 0 || jet.Point[2].Cmp(big.NewRat(6, 77)) != 0 {
		t.Fatal("independent original fractional knot control is inconsistent", jet, err)
	}
	o := ExactSurfaceComparisonOptions{NormalizeDomains: true, SwapB: true, ReverseBU: true,
		BStationRatios: [2]*big.Rat{big.NewRat(3, 2), big.NewRat(4, 3)}}
	r, err := CompareExactSurfaces(ctx, a, b, o)
	if err != nil || !r.Equal || !r.Complete || r.ComparedCells != 4 {
		t.Fatal("failed to prove exact coincidence on the original pulled knot partition", r, err)
	}
	partial := [2]ExactSurfaceRectangle{
		{ExactParameterInterval{big.NewRat(1, 7), big.NewRat(5, 6)}, ExactParameterInterval{big.NewRat(1, 10), big.NewRat(9, 10)}},
		{ExactParameterInterval{big.NewRat(-46, 31), big.NewRat(22, 13)}, ExactParameterInterval{big.NewRat(76, 17), big.NewRat(36, 5)}},
	}
	local := o
	local.NormalizeDomains = false
	local.BStationRatios = [2]*big.Rat{big.NewRat(119, 90), big.NewRat(39, 31)}
	r, err = CompareExactSurfaceRectangles(ctx, a, b, partial, local)
	if err != nil || !r.Equal || !r.Complete || r.ComparedCells != 4 {
		t.Fatal("nonbinary exact clipping must retain the projective knot partition", r, err)
	}
	outside := partial
	outside[1].U.Lower = big.NewRat(-3, 1)
	if r, err := CompareExactSurfaceRectangles(ctx, a, b, outside, local); !errors.Is(err, ErrDomain) || r.Complete {
		t.Fatal("requested rectangle crossed the original domain", r, err)
	}
	wrong := o
	wrong.BStationRatios[1] = big.NewRat(5, 4)
	r, err = CompareExactSurfaces(ctx, a, b, wrong)
	if err != nil || !r.Complete || r.Equal || r.DifferentAxis != 1 {
		t.Fatal("coincident corners must not authenticate a wrong interior station map", r, err)
	}
	r, err = CompareExactSurfaces(ctx, a, makeB(true), o)
	if err != nil || !r.Complete || r.Equal || r.DifferentAxis != 2 {
		t.Fatal("tiny original interior deviation must survive the exact mapped comparison", r, err)
	}
	short := o
	short.MaxCells = 3
	r, err = CompareExactSurfaces(ctx, a, b, short)
	if !errors.Is(err, ErrConvergence) || r.Complete {
		t.Fatal("incomplete mapped knot cover must not claim equality", r, err)
	}
	wrong.BStationRatios[0] = new(big.Rat)
	if r, err := CompareExactSurfaces(ctx, a, b, wrong); !errors.Is(err, ErrInvalid) || r.Complete {
		t.Fatal("station map is not bijective with a zero ratio", r, err)
	}
}

func TestExactSurfaceComparisonPartialLeavesOtherRegionsUnclassified(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	s, err := NewBSplineSurface(1, 1, 3, 2,
		[]Vec3{{0, 0, 0}, {0, 1, 0}, {.5, 0, 0}, {.5, 1, 0}, {1, 0, 1}, {1, 1, 1}},
		[]float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSurfacePatch(s, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	area := ExactSurfaceRectangle{ExactParameterInterval{big.NewRat(1, 7), big.NewRat(3, 7)}, ExactParameterInterval{big.NewRat(1, 9), big.NewRat(8, 9)}}
	r, err := CompareExactSurfaceRectangles(ctx, a, b, [2]ExactSurfaceRectangle{area, area}, ExactSurfaceComparisonOptions{})
	if err != nil || !r.Equal || !r.Complete || r.ComparedCells != 1 {
		t.Fatal("failed to prove the selected interior area", r, err)
	}
	r, err = CompareExactSurfaces(ctx, a, b, ExactSurfaceComparisonOptions{})
	if err != nil || r.Equal || !r.Complete || r.DifferentCell != 1 || r.DifferentAxis != 2 {
		t.Fatal("selected partial equality must not imply whole-surface equality", r, err)
	}
}
