package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSurfacePatchSecondJetRationalPreservesOriginalKnotSide(t *testing.T) {
	ctx := context.Background()
	// Two quadratic U pieces, sharing only their end control. The original
	// chart is S(u,v)=(u,v,z(u)+u*v), with physical U domains [-2,1] and
	// [1,5]. The left height is (u-1)^2 and the right height is
	// 2(u-1)+3(u-1)^2.
	var poles []Vec3
	for i, u := range []float64{-2, -.5, 1, 3, 5} {
		for _, v := range []float64{-1, 3} {
			z := []float64{9, 0, 0, 4, 56}[i] + u*v
			poles = append(poles, Vec3{u, v, z})
		}
	}
	s, err := NewBSplineSurface(2, 1, 5, 2, poles, []float64{-2, -2, -2, 1, 1, 5, 5, 5}, []float64{-1, -1, 3, 3}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewSurfacePatch(s, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	v := big.NewRat(1, 1)
	for _, tc := range []struct {
		u, duz, duuz int64
		domain       ParameterRectangle
	}{{0, -1, 2, ParameterRectangle{-2, 1, -1, 3}},
		{1, 3, 6, ParameterRectangle{1, 5, -1, 3}},
		{5, 27, 6, ParameterRectangle{1, 5, -1, 3}}} {
		u := big.NewRat(tc.u, 1)
		got, err := p.EvaluateSecondJetRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if err != nil || got.KnotCell != tc.domain {
			t.Fatal("wrong original knot cell", tc.u, got.KnotCell, err)
		}
		if got.Du[2].Cmp(big.NewRat(tc.duz, 1)) != 0 || got.Duu[2].Cmp(big.NewRat(tc.duuz, 1)) != 0 ||
			got.Duv[2].Cmp(big.NewRat(1, 1)) != 0 || got.Dvv[2].Sign() != 0 || got.Dv[2].Cmp(u) != 0 {
			t.Fatalf("incorrect exact physical derivatives at %d: %+v", tc.u, got)
		}
		original, err := p.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for axis := range 3 {
			if got.Point[axis].Cmp(original.Point[axis]) != 0 || got.Du[axis].Cmp(original.Du[axis]) != 0 ||
				got.Dv[axis].Cmp(original.Dv[axis]) != 0 {
				t.Fatal("second and first jet disagree on original chart", tc.u, axis)
			}
		}
		short, err := p.EvaluateSecondJetRational(ctx, u, v, SurfacePatchEvaluationOptions{MaxOperations: got.ExactOperations - 1})
		if !errors.Is(err, ErrConvergence) || short.Duv[2] != nil || short.KnotCell.Valid() {
			t.Fatal("exhausted evaluation published a cell or partial jet", short, err)
		}
	}
}

func TestSurfacePatchSecondJetRationalWeightsAffectEverySecondPartial(t *testing.T) {
	ctx := context.Background()
	var controls [][4]*big.Rat
	for _, x := range []int64{0, 1} {
		for _, y := range []int64{0, 1} {
			controls = append(controls, [4]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1),
				big.NewRat(x*y, 1), big.NewRat(1+x+y, 1)})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.EvaluateSecondJetRational(ctx, big.NewRat(1, 4), big.NewRat(1, 3), SurfacePatchEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Even with linear basis functions in each direction, the independent
	// quotient x*y/(1+x+y) has nonzero pure and mixed second derivatives.
	if got.Duu[2].Cmp(big.NewRat(-1536, 6859)) != 0 || got.Duv[2].Cmp(big.NewRat(3024, 6859)) != 0 ||
		got.Dvv[2].Cmp(big.NewRat(-1080, 6859)) != 0 {
		t.Fatal("rational normal jet lost weight curvature or mixed degree", got.Duu, got.Duv, got.Dvv)
	}
}
