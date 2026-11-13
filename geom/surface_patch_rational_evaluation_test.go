package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestSurfacePatchRationalBilinearEvaluation(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	u, v := big.NewRat(1, 3), big.NewRat(2, 5)
	r, e := p.EvaluateRational(context.Background(), u, v, SurfacePatchEvaluationOptions{})
	if e != nil || r.Point[2].Cmp(big.NewRat(2, 15)) != 0 || r.Du[2].Cmp(v) != 0 || r.Dv[2].Cmp(u) != 0 {
		t.Fatal(r, e)
	}
	for axis, x := range r.Point {
		lo := new(big.Rat).SetFloat64(coordinate(r.PointBounds.Min, axis))
		hi := new(big.Rat).SetFloat64(coordinate(r.PointBounds.Max, axis))
		if lo.Cmp(x) > 0 || hi.Cmp(x) < 0 {
			t.Fatal(r)
		}
	}
	r.Point[0].SetInt64(9)
	again, e := p.EvaluateRational(context.Background(), u, v, SurfacePatchEvaluationOptions{})
	if e != nil || again.Point[0].Cmp(u) != 0 || u.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(again, e)
	}
	// The exact nonbinary point lies on this rational segment and its plane;
	// its float representative is deliberately not used for either check.
	start := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(2, 5), big.NewRat(2, 15)}
	end := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 5), big.NewRat(2, 15)}
	for axis := 0; axis < 3; axis++ {
		expected := new(big.Rat).Add(start[axis], new(big.Rat).Mul(u, new(big.Rat).Sub(end[axis], start[axis])))
		if expected.Cmp(again.Point[axis]) != 0 {
			t.Fatal(expected, again.Point)
		}
	}
}
func TestSurfacePatchRationalWeightsAndAffineInverse(t *testing.T) {
	s, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{}, {Y: 1, Z: 2}, {X: 1, Z: 1}, {X: 1, Y: 1, Z: 3}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, 2, 2})
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	r, e := p.EvaluateRational(context.Background(), big.NewRat(1, 3), big.NewRat(2, 5), SurfacePatchEvaluationOptions{})
	if e != nil || r.Point[0].Cmp(big.NewRat(1, 2)) != 0 || r.Point[2].Cmp(big.NewRat(13, 10)) != 0 || r.Du[0].Cmp(big.NewRat(9, 8)) != 0 {
		t.Fatal(r, e)
	}
	flat := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u + 2*v} })
	chart, _ := NewAffineSurfaceChart(context.Background(), flat, AffineSurfaceChartOptions{})
	inv, e := chart.InvertExact(r.Point)
	if e != nil || !inv.OnPlane || !inv.Inside || inv.U.Cmp(big.NewRat(1, 2)) != 0 || inv.V.Cmp(big.NewRat(2, 5)) != 0 {
		t.Fatal(inv, e)
	}
}
func TestSurfacePatchRationalKnotSideAndLimits(t *testing.T) {
	var poles []Vec3
	for i, u := range []float64{0, .5, 1} {
		for _, v := range []float64{0, 1} {
			z := 0.
			if i == 2 {
				z = 1
			}
			poles = append(poles, Vec3{u, v, z})
		}
	}
	s, _ := NewBSplineSurface(1, 1, 3, 2, poles, []float64{0, 0, .5, 1, 1}, []float64{0, 0, 1, 1}, nil)
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	below := new(big.Rat).Sub(big.NewRat(1, 2), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(3), 80)))
	f, _ := below.Float64()
	if f != .5 {
		t.Fatal("fixture")
	}
	for _, tc := range []struct {
		u  *big.Rat
		dz int64
	}{{below, 0}, {big.NewRat(1, 2), 2}, {big.NewRat(1, 1), 2}} {
		r, e := p.EvaluateRational(context.Background(), tc.u, big.NewRat(1, 3), SurfacePatchEvaluationOptions{})
		if e != nil || r.Du[2].Cmp(big.NewRat(tc.dz, 1)) != 0 {
			t.Fatal(r, e)
		}
	}
	_, e := p.EvaluateRational(context.Background(), big.NewRat(-1, 3), big.NewRat(1, 3), SurfacePatchEvaluationOptions{})
	if !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
	_, e = p.EvaluateRational(context.Background(), big.NewRat(1, 3), big.NewRat(1, 3), SurfacePatchEvaluationOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = p.EvaluateRational(canceled, big.NewRat(1, 3), big.NewRat(1, 3), SurfacePatchEvaluationOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
