package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestAffineXYGraphHeightCertificate(t *testing.T) {
	ctx := context.Background()
	makeGraph := func(swap bool, z float64) *SurfacePatch {
		p := make([]Vec3, 4)
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				x, y := float64(2*i+j), float64(i+2*j)
				if swap {
					x, y = y, x
				}
				p[2*i+j] = Vec3{x, y, z}
			}
		}
		s, e := NewBSplineSurface(1, 1, 2, 2, p, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		p0, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p0
	}
	a, b := makeGraph(false, 1), makeGraph(true, 0)
	ga, e := NewAffineXYGraph(ctx, a, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	gb, e := NewAffineXYGraph(ctx, b, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if ga.Orientation() != 1 || gb.Orientation() != -1 {
		t.Fatal("orientation")
	}
	uv := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}
	r, e := ga.CompareHeightAtRational(ctx, gb, uv, SurfacePatchEvaluationOptions{})
	if e != nil || !r.Complete || r.Sign != 1 || r.Difference.Cmp(big.NewRat(1, 1)) != 0 || r.TargetUV[0].Cmp(uv[1]) != 0 || r.TargetUV[1].Cmp(uv[0]) != 0 {
		t.Fatalf("%+v %v", r, e)
	}
	copy, _ := ga.InverseCoefficients()
	copy[0][0].SetInt64(0)
	r, e = CompareAffineXYGraphHeight(ctx, b, a, r.TargetUV, SurfacePatchOptions{})
	if e != nil || r.Sign != -1 {
		t.Fatal(r, e)
	}
	r, e = ga.CompareHeightAtRational(ctx, ga, uv, SurfacePatchEvaluationOptions{})
	if e != nil || r.Sign != 0 {
		t.Fatal(r, e)
	}
	clipped, e := b.Trim(ParameterRectangle{0, .25, 0, .25})
	if e != nil {
		t.Fatal(e)
	}
	r, e = CompareAffineXYGraphHeight(ctx, a, clipped, uv, SurfacePatchOptions{})
	if e != nil || !r.TargetOutside || !r.Complete || r.Difference != nil {
		t.Fatal(r, e)
	}
	_, e = ga.CompareHeightAtRational(ctx, gb, uv, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, e = NewAffineXYGraph(cancelCtx, a, SurfacePatchOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	tiny := makeGraph(false, 0)
	for i := range tiny.net {
		tiny.net[i][2] = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 1100))
	}
	tiny.computeBounds(&polynomialBudget{maxOperations: 10000, maxBits: 16384})
	r, e = CompareAffineXYGraphHeight(ctx, tiny, makeGraph(false, 0), uv, SurfacePatchOptions{})
	if e != nil || r.Sign != 1 {
		t.Fatal(r, e)
	}
}

func TestAffineXYGraphCurvedHeight(t *testing.T) {
	ctx := context.Background()
	poles := make([]Vec3, 9)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			z := 0.
			if i == 2 {
				z++
			}
			if j == 2 {
				z += 2
			}
			poles[3*i+j] = Vec3{float64(i) / 2, float64(j) / 2, z}
		}
	}
	k := []float64{0, 0, 0, 1, 1, 1}
	s, e := NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	a, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range poles {
		poles[i].X, poles[i].Y = poles[i].Y, poles[i].X
		poles[i].Z = 0
	}
	s, e = NewBSplineSurface(2, 2, 3, 3, poles, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := CompareAffineXYGraphHeight(ctx, a, b, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)}, SurfacePatchOptions{})
	if e != nil || r.Difference.Cmp(big.NewRat(97, 225)) != 0 {
		t.Fatal(r, e)
	}
}

func TestExactHomogeneousSplineRawWeights(t *testing.T) {
	ctx := context.Background()
	r := func(n, d int64) *big.Rat { return big.NewRat(n, d) }
	net := [][4]*big.Rat{{r(0, 1), r(0, 1), r(0, 1), r(4, 1)}, {r(2, 1), r(2, 1), r(0, 1), r(4, 1)}, {r(9, 2), r(2, 1), r(0, 1), r(9, 2)}, {r(5, 1), r(0, 1), r(0, 1), r(5, 1)}}
	knots := []float64{0, 0, 0, .5, 1, 1, 1}
	c, e := NewExactHomogeneousSpline(ctx, 2, net, knots, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// At the inserted knot the value is the mean of the two middle homogeneous controls.
	jet, e := c.patch.EvaluateRational(ctx, r(1, 2), r(0, 1), SurfacePatchEvaluationOptions{})
	if e != nil || jet.Point[0].Cmp(r(13, 17)) != 0 || jet.Point[1].Cmp(r(8, 17)) != 0 {
		t.Fatal(jet, e)
	}
	net[1][0].SetInt64(999)
	knots[3] = .25
	again, e := c.patch.EvaluateRational(ctx, r(1, 2), r(0, 1), SurfacePatchEvaluationOptions{})
	if e != nil || again.Point[0].Cmp(jet.Point[0]) != 0 {
		t.Fatal("alias", e)
	}
	_, e = NewExactHomogeneousSpline(ctx, 2, net, []float64{0, 0, 0, .5, .5, .5, 1}, SurfacePatchOptions{})
	if e == nil {
		t.Fatal("accepted invalid knots")
	}
	_, e = NewExactHomogeneousSpline(ctx, 2, net, []float64{0, 0, 0, .5, 1, 1, 1}, SurfacePatchOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
