package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestIsoRawPlaneRationalRoots(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		z     []float64
		roots []string
		mult  int
	}{
		{[]float64{2, -2.5, 2}, []string{"1/3", "2/3"}, 1},
		{[]float64{1, -2, 4}, []string{"1/3"}, 2},
		{[]float64{-1, 2}, []string{"1/3"}, 1},
	} {
		poles := make([]Vec3, len(tc.z))
		for i, z := range tc.z {
			poles[i] = Vec3{X: float64(i), Y: float64(i), Z: 3*float64(i) + z}
		}
		spline, _ := NewBezier(poles, nil)
		curve := isoIntersectionFixture(t, spline)
		r, e := curve.IntersectRawPlane(ctx, Vec3{}, Vec3{1, 0, 1}, Vec3{0, 1, 2}, SplinePlaneOptions{})
		if e != nil || !r.Complete || len(r.Hits) != len(tc.roots) {
			t.Fatal(r, e)
		}
		for i, h := range r.Hits {
			if h.Parameter == nil || h.Parameter.RatString() != tc.roots[i] || h.Multiplicity != tc.mult {
				t.Fatal(h)
			}
			res := new(big.Rat).Sub(h.Point[2], new(big.Rat).Add(h.Point[0], new(big.Rat).Mul(big.NewRat(2, 1), h.Point[1])))
			if res.Sign() != 0 {
				t.Fatal("raw residual", res)
			}
			if h.Lower.Cmp(h.Upper) != 0 {
				t.Fatal(h)
			}
		}
	}
}
func TestIsoRawPlaneAlgebraicCoincidentAndLimits(t *testing.T) {
	ctx := context.Background()
	s, _ := NewBezier([]Vec3{{Z: -.5}, {X: .5, Z: -.5}, {X: 1, Z: .5}}, nil)
	c := isoIntersectionFixture(t, s)
	a, b, d := Vec3{}, Vec3{X: 1}, Vec3{Y: 1}
	r, e := c.IntersectRawPlane(ctx, a, b, d, SplinePlaneOptions{ParameterTolerance: 1e-8})
	if e != nil || !r.Complete || len(r.Hits) != 1 || r.Hits[0].Parameter != nil {
		t.Fatal(r, e)
	}
	h := r.Hits[0]
	if new(big.Rat).Mul(h.Lower, h.Lower).Cmp(big.NewRat(1, 2)) > 0 || new(big.Rat).Mul(h.Upper, h.Upper).Cmp(big.NewRat(1, 2)) < 0 {
		t.Fatal(h)
	}
	flat, _ := NewBezier([]Vec3{{}, {X: 1}}, nil)
	r, e = isoIntersectionFixture(t, flat).IntersectRawPlane(ctx, a, b, d, SplinePlaneOptions{})
	if e != nil || !r.Complete || len(r.Coincident) != 1 {
		t.Fatal(r, e)
	}
	_, e = c.IntersectRawPlane(ctx, a, b, b, SplinePlaneOptions{})
	if !errors.Is(e, ErrDegenerate) {
		t.Fatal(e)
	}
	r, e = c.IntersectRawPlane(ctx, a, b, d, SplinePlaneOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = c.IntersectRawPlane(canceled, a, b, d, SplinePlaneOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestIsoRawPlaneRationalWeights(t *testing.T) {
	s, _ := NewBezier([]Vec3{{Z: -1}, {X: 1, Z: 1}}, []float64{1, 2})
	c := isoIntersectionFixture(t, s)
	r, e := c.IntersectRawPlane(context.Background(), Vec3{}, Vec3{X: 1}, Vec3{Y: 1}, SplinePlaneOptions{})
	if e != nil || len(r.Hits) != 1 || r.Hits[0].Parameter.RatString() != "1/3" || r.Hits[0].Point[0].RatString() != "1/2" {
		t.Fatal(r, e)
	}
	r.Hits[0].Point[0].SetInt64(99)
	r, e = c.IntersectRawPlane(context.Background(), Vec3{}, Vec3{X: 1}, Vec3{Y: 1}, SplinePlaneOptions{})
	if e != nil || r.Hits[0].Point[0].RatString() != "1/2" {
		t.Fatal(r, e)
	}
}
