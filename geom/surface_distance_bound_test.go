package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestSurfaceDistanceBoundsPlaneAndBoundary(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	for _, tc := range []struct {
		point    Vec3
		distance float64
	}{{Vec3{.5, .5, 2}, 4}, {Vec3{2, .3, 0}, 1}, {Vec3{.3, .4, 0}, 0}} {
		r, e := BoundPointSurfaceDistance(tc.point, p, SurfaceDistanceOptions{SquaredTolerance: 1e-5})
		if e != nil || !r.Complete || r.SquaredDistanceMin > tc.distance || r.SquaredDistanceMax < tc.distance || r.SquaredDistanceMax-r.SquaredDistanceMin > 1.00001e-5 {
			t.Fatal(tc, r, e)
		}
	}
}
func TestSurfaceDistanceBoundsRationalCylinder(t *testing.T) {
	p, _ := NewSurfacePatch(quarterCylinder(t), SurfacePatchOptions{})
	q := Vec3{3, 3, 1}
	distance := (math.Sqrt(18) - 2) * (math.Sqrt(18) - 2)
	r, e := BoundPointSurfaceDistance(q, p, SurfaceDistanceOptions{SquaredTolerance: 5e-3, MaxOperations: 8000000})
	if e != nil || !r.Complete || r.SquaredDistanceMin > distance+1e-12 || r.SquaredDistanceMax < distance-1e-12 {
		t.Fatal(r, e)
	}
}
func TestSurfaceDistanceBoundsPreservedAtLimit(t *testing.T) {
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	r, e := BoundPointSurfaceDistance(Vec3{2, .3, 0}, p, SurfaceDistanceOptions{MaxCells: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || r.SquaredDistanceMin > 1 || r.SquaredDistanceMax < 1 {
		t.Fatal(r, e)
	}
}

func TestExactRationalSurfaceDistanceUnrounded(t *testing.T) {
	ctx := context.Background()
	p := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	eps := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 120))
	query := [3]*big.Rat{new(big.Rat).Add(big.NewRat(1, 1), eps), big.NewRat(1, 1), new(big.Rat)}
	r, e := BoundPointSurfaceDistanceRational(ctx, query, p, SurfaceDistanceOptions{SquaredTolerance: 1e-90})
	want := new(big.Rat).Mul(eps, eps)
	if e != nil || !r.Complete || !r.HasUpperPoint || r.Squared.Lower.Cmp(want) != 0 || r.Squared.Upper.Cmp(want) != 0 || r.Witness[0].Cmp(big.NewRat(1, 1)) != 0 || r.Witness[1].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("the exact displacement from the trim corner must not round to zero", r, e, want)
	}
	query[1] = big.NewRat(1, 3)
	r, e = BoundPointSurfaceDistanceRational(ctx, query, p, SurfaceDistanceOptions{MaxCells: 1, SquaredTolerance: 1e-30})
	if !errors.Is(e, ErrConvergence) || r.Complete || !r.HasUpperPoint || r.Squared.Lower.Cmp(want) > 0 || r.Squared.Upper.Cmp(want) < 0 {
		t.Fatal("resource limit must retain a truthful exact enclosure", r, e)
	}
}
