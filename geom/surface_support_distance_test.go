package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestPointSurfaceSupportImprovesDiagonalHull(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{(1 - u) * (1 - v), v, u * (1 - v)} })
	budget := &polynomialBudget{maxOperations: 1000, maxBits: 16384}
	box, e := exactPointBoxDistance(Vec3{}, p.bounds, budget)
	if e != nil || box.Sign() != 0 {
		t.Fatal(box, e)
	}
	r, e := BoundPointSurfaceSupport(context.Background(), Vec3{}, p, PointSurfaceSupportOptions{})
	if e != nil || !r.HasBound || !r.Complete || r.SquaredLower.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(r, e)
	}
	// The attainable centroid independently proves this lower bound is sharp.
	point, e := p.EvaluateRational(context.Background(), big.NewRat(1, 2), big.NewRat(1, 3), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range point.Point {
		if x.Cmp(big.NewRat(1, 3)) != 0 {
			t.Fatal(point)
		}
	}
	for _, scale := range []float64{1, -2, math.Ldexp(1, -600)} {
		r, e := BoundPointSurfaceSupport(context.Background(), Vec3{}, p, PointSurfaceSupportOptions{Directions: []Vec3{{scale, scale, scale}}, MaxIterations: 1})
		if e != nil || r.SquaredLower.Cmp(big.NewRat(1, 3)) != 0 {
			t.Fatal(r, e)
		}
	}
}
func TestPointSurfaceSupportRationalCircleCap(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	p, e := p.Trim(ParameterRectangle{.4, .6, .25, .75})
	if e != nil {
		t.Fatal(e)
	}
	query := Vec3{Z: 1}
	b := &polynomialBudget{maxOperations: 1000, maxBits: 16384}
	box, e := exactPointBoxDistance(query, p.bounds, b)
	if e != nil {
		t.Fatal(e)
	}
	r, e := BoundPointSurfaceSupport(context.Background(), query, p, PointSurfaceSupportOptions{})
	if e != nil || r.SquaredLower.Cmp(box) <= 0 || r.SquaredLowerBound < 3.8 || r.SquaredLower.Cmp(big.NewRat(4, 1)) > 0 {
		t.Fatal(box, r, e)
	}
	// An independently evaluated exact surface point is an upper witness for
	// the represented rational arc (whose circle weight is a rounded float).
	jet, e := p.EvaluateRational(context.Background(), big.NewRat(1, 2), big.NewRat(1, 2), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	witnessDistance := new(big.Rat)
	for axis, x := range jet.Point {
		delta := new(big.Rat).Sub(x, new(big.Rat).SetFloat64(coordinate(query, axis)))
		witnessDistance.Add(witnessDistance, new(big.Rat).Mul(delta, delta))
	}
	if r.SquaredLower.Cmp(witnessDistance) > 0 {
		t.Fatal(r, witnessDistance)
	}
	r.SquaredLower.SetInt64(100)
	again, e := BoundPointSurfaceSupport(context.Background(), query, p, PointSurfaceSupportOptions{})
	if e != nil || again.SquaredLower.Cmp(big.NewRat(4, 1)) > 0 {
		t.Fatal(again, e)
	}
}
func TestPointSurfaceSupportLimitsAndContact(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 1} })
	r, e := BoundPointSurfaceSupport(context.Background(), Vec3{.5, .5, 1}, p, PointSurfaceSupportOptions{})
	if e != nil || r.SquaredLower.Sign() != 0 {
		t.Fatal(r, e)
	}
	r, e = BoundPointSurfaceSupport(context.Background(), Vec3{}, p, PointSurfaceSupportOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || !r.HasBound || r.Complete || r.SquaredLower.Sign() < 0 {
		t.Fatal(r, e)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = BoundPointSurfaceSupport(canceled, Vec3{}, p, PointSurfaceSupportOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = BoundPointSurfaceSupport(context.Background(), Vec3{}, p, PointSurfaceSupportOptions{Directions: []Vec3{{}}})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 1100))
	bounds, e := RationalOutwardRange(tiny)
	if e != nil || bounds.Min != -math.SmallestNonzeroFloat64 || bounds.Max != 0 {
		t.Fatal(bounds, e)
	}
}

func TestPointSurfaceSupportNonbinaryRationalCovector(t *testing.T) {
	ctx := context.Background()
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{(1 - u) * (1 - v), v, u * (1 - v)} })
	// This point is just behind the plane X+Y+Z=1. Its normal
	// projection is (1/3,1/3,1/3), which this exact patch attains.
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	component := new(big.Rat).Sub(big.NewRat(1, 3), eps)
	point := [3]*big.Rat{component, component, component}
	d := big.NewRat(-3, 7)
	r, e := BoundPointSurfaceSupportRational(ctx, point, p, [][3]*big.Rat{{d, d, d}}, PointSurfaceSupportOptions{})
	want := new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Mul(eps, eps))
	if e != nil || !r.HasBound || !r.Complete || r.SquaredLower.Cmp(want) != 0 || r.Direction[0].Cmp(big.NewRat(3, 7)) != 0 {
		t.Fatal("nonbinary control-hull separation must attain the independently known bound", r, e, want)
	}
}
