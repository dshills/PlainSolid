package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestTransformProjectiveUVKeepsOriginalCurveKnotsAndPhysicalDerivatives(t *testing.T) {
	ctx := context.Background()
	s, err := NewBSplineWithExactWeights(2, []Vec3{{0, 0, 0}, {1, 2, 0}, {2, .5, 0}, {3, 2, 0}, {4, 1, 0}},
		[]float64{-2, -2, -2, 0, 2, 5, 5, 5}, []float64{1, 2, 3, 2, 1})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewExactSplineCurve(s, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ints := func(a, b int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(a, 1), big.NewRat(b, 1)} }
	m := ProjectiveUVMap{{1, ints(3, 1), ints(1, 2)}, {0, ints(-1, 2), ints(1, 3)}}
	mapped, err := c.TransformProjectiveUV(ctx, m, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := mapped.Domain()
	if lo != -2 || hi != 5 || mapped.patch.degreeU != 4 || mapped.patch.countU != 13 ||
		mapped.patch.knotsU[5] != 0 || mapped.patch.knotsU[9] != 2 {
		t.Fatal("mapping lost the original clock or knot ownership", mapped.patch)
	}
	zero := new(big.Rat)
	for _, station := range []*big.Rat{big.NewRat(-2, 1), big.NewRat(-1, 3), zero, big.NewRat(8, 7), big.NewRat(2, 1), big.NewRat(5, 1)} {
		x, err := c.patch.EvaluateRational(ctx, station, zero, SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		y, err := mapped.patch.EvaluateRational(ctx, station, zero, SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// Independent coordinate and chain-rule calculations in the
		// original physical curve clock, including both original knots.
		denX := new(big.Rat).Add(x.Point[1], big.NewRat(2, 1))
		denY := new(big.Rat).Add(x.Point[0], big.NewRat(3, 1))
		wantX := new(big.Rat).Add(new(big.Rat).Mul(x.Point[1], big.NewRat(3, 1)), big.NewRat(1, 1))
		wantY := new(big.Rat).Sub(big.NewRat(2, 1), x.Point[0])
		wantDx := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(5, 1), x.Du[1]), new(big.Rat).Mul(denX, denX))
		wantDy := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(-5, 1), x.Du[0]), new(big.Rat).Mul(denY, denY))
		if y.Point[0].Cmp(wantX.Quo(wantX, denX)) != 0 || y.Point[1].Cmp(wantY.Quo(wantY, denY)) != 0 ||
			y.Du[0].Cmp(wantDx) != 0 || y.Du[1].Cmp(wantDy) != 0 || y.Point[2].Sign() != 0 {
			t.Fatal("original UV curve or its physical derivative changed under mapping", station, y)
		}
	}
	options := SurfacePatchOptions{MaxPoles: 25}
	if out, err := c.TransformProjectiveUV(ctx, m, options); !errors.Is(err, ErrConvergence) || out != nil {
		t.Fatal("incomplete mapped net must not publish a curve", out, err)
	}
	pole := m
	pole[1].Denominator = ints(1, -1)
	if out, err := c.TransformProjectiveUV(ctx, pole, SurfacePatchOptions{}); !errors.Is(err, ErrDomain) || out != nil {
		t.Fatal("mapping cannot admit a denominator with a pole in the original chart", out, err)
	}
	affine := ProjectiveUVMap{{1, ints(3, 1), ints(0, 2)}, {0, ints(-1, 2), ints(0, 3)}}
	result, err := c.TransformProjectiveUV(ctx, affine, SurfacePatchOptions{})
	if err != nil || result.patch.degreeU != 2 || result.patch.countU != 5 {
		t.Fatal("affine map should preserve the original degree and knot representation", result, err)
	}
}

func TestProjectiveSurfaceGraphOverlapMapsAnOriginalUVCurve(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	b := projectiveOverlapTestChart(t, false, false)
	proof, err := CertifyProjectiveSurfaceGraphOverlap(ctx, a, b, [2]int{0, 1}, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := proof.OriginalUVMap(SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	c, err := NewExactHomogeneousBezier(ctx, [][4]*big.Rat{
		{big.NewRat(1, 7), big.NewRat(4, 7), zero, one},
		{big.NewRat(6, 7), big.NewRat(13, 14), zero, one},
	}, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := c.TransformProjectiveUV(ctx, m, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, station := range [4]*big.Rat{zero, big.NewRat(3, 11), big.NewRat(10, 13), one} {
		var model [2][3]*big.Rat
		for i, p := range [2]*SurfacePatch{a, b} {
			original := [2]*SurfaceIsoCurve{c, mapped}[i]
			uv, err := original.patch.EvaluateRational(ctx, station, zero, SurfacePatchEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			x, err := p.EvaluateRational(ctx, uv.Point[0], uv.Point[1], SurfacePatchEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			model[i] = x.Point
		}
		for axis := range 3 {
			if model[0][axis].Cmp(model[1][axis]) != 0 {
				t.Fatal("mapped original curves differ on the proved common graph", station, axis, model)
			}
		}
	}
}
