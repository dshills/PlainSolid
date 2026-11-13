package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestShearedGraphTubeRationalSectionAndTrim(t *testing.T) {
	ctx := context.Background()
	knots := []float64{0, 0, 0, 1, 1, 1}
	poles := make([]Vec3, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z++
			}
			poles[3*i+j] = Vec3{float64(i) / 2, float64(j) / 2, z}
		}
	}
	spline, e := NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
	if e != nil {
		t.Fatal(e)
	}
	original, e := NewSurfacePatch(spline, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := original.Trim(ParameterRectangle{7.0 / 32, .25, 5.0 / 8, 11.0 / 16})
	if e != nil {
		t.Fatal(e)
	}
	for _, swapped := range []bool{false, true} {
		coordinates := []float64{-1, 1}
		planePoles := make([]Vec3, 4)
		for i, u := range coordinates {
			for j, v := range coordinates {
				x, y := 2*u+v, u+2*v
				if swapped {
					x, y = u+2*v, 2*u+v
				}
				planePoles[2*i+j] = Vec3{x, y, .5}
			}
		}
		k := []float64{-1, -1, 1, 1}
		surface, e := NewBSplineSurface(1, 1, 2, 2, planePoles, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		target, e := NewSurfacePatch(surface, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		domain := ParameterRectangle{-.125, 0, 5.0 / 16, 7.0 / 16}
		if swapped {
			domain = ParameterRectangle{5.0 / 16, 7.0 / 16, -.125, 0}
		}
		local, e := target.Trim(domain)
		if e != nil {
			t.Fatal(e)
		}
		tube, e := NewVerifiedSurfaceTube(a, local, SurfacePairTubeOptions{})
		if e != nil {
			t.Fatal(e)
		}
		section, e := tube.EvaluateRationalGraph(ctx, big.NewRat(7, 30), TubeAlgebraicGraphEndpointOptions{})
		if e != nil || !section.Proven {
			t.Fatal(section, e)
		}
		v := importTestRoot(t, big.NewRat(-401, 900), true, "5/8", "11/16")
		first, _, e := v.AffineMap(ctx, big.NewRat(7, 45), big.NewRat(-1, 3), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		second, _, e := v.AffineMap(ctx, big.NewRat(-7, 90), big.NewRat(2, 3), ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		expected := []*ExactAlgebraicRoot{first, second}
		if swapped {
			expected[0], expected[1] = expected[1], expected[0]
		}
		for axis := 0; axis < 2; axis++ {
			order, e := section.Parameters[axis+2].Compare(ctx, expected[axis], ExactCurvePlaneEventOptions{})
			if e != nil || order.Order != 0 {
				t.Fatal(order, e)
			}
		}
		if section.PointBounds.Min.X > 7.0/30 || section.PointBounds.Max.X < 7.0/30 || section.PointBounds.Min.Z > .5 || section.PointBounds.Max.Z < .5 {
			t.Fatal(section)
		}
		trim, e := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{big.NewRat(15, 64), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(15, 64), new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(15, 64), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)}}, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		hits, e := tube.IntersectQuadraticTrim(ctx, original, target, 0, trim, TubeTrimIntersectionOptions{})
		if e != nil || !hits.Complete || len(hits.Hits) != 1 || !hits.Hits[0].Transverse {
			t.Fatal(hits, e)
		}
	}
}

func TestAffineXYGraphRejectsNonaffineAndSingular(t *testing.T) {
	p, _ := graphContactFixture(t, false, false)
	nonaffine := *p
	nonaffine.net = append([]exactHomogeneous(nil), p.net...)
	nonaffine.net[4] = p.net[4]
	nonaffine.net[4][0] = new(big.Rat).Add(p.net[4][0], big.NewRat(1, 1024))
	budget := &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
	if _, _, e := normalizeAffineXYGraph(context.Background(), &nonaffine, budget); !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	singular := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u + v, 2 * (u + v), u * v} })
	budget = &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
	if _, _, e := normalizeAffineXYGraph(context.Background(), singular, budget); !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
}

func TestCurvedAffineXYGraphNormalizationAcrossKnots(t *testing.T) {
	ctx := context.Background()
	original, _ := graphContactFixture(t, false, false)
	original, e := original.InsertUKnot(.5, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	original, e = original.InsertVKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	sheared := *original
	sheared.net = make([]exactHomogeneous, len(original.net))
	for i, h := range original.net {
		for axis := 0; axis < 4; axis++ {
			sheared.net[i][axis] = new(big.Rat).Set(h[axis])
		}
		sheared.net[i][0] = new(big.Rat).Add(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(2, 1), h[0]), h[1]), new(big.Rat).Mul(big.NewRat(1, 4), h[3]))
		sheared.net[i][1] = new(big.Rat).Sub(new(big.Rat).Sub(h[0], new(big.Rat).Mul(big.NewRat(2, 1), h[1])), new(big.Rat).Mul(big.NewRat(1, 2), h[3]))
	}
	b := &polynomialBudget{maxOperations: 1000000, maxBits: 16384}
	if e := sheared.computeBounds(b); e != nil {
		t.Fatal(e)
	}
	normalized, _, e := normalizeAffineXYGraph(ctx, &sheared, b)
	if e != nil {
		t.Fatal(e)
	}
	identity, e := CompareExactSurfaces(ctx, original, normalized, ExactSurfaceComparisonOptions{})
	if e != nil || !identity.Complete || !identity.Equal {
		t.Fatal(identity, e)
	}
}
