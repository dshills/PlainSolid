package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func triangularPointPatch(t *testing.T) *SurfacePatch {
	p := triangularInversePatch(t, false)
	for i, h := range p.net {
		p.net[i][2] = new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Sub(h[1], h[0]))
	}
	if e := p.computeBounds(&polynomialBudget{maxOperations: 100000, maxBits: 16384}); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestTriangularSurfaceAlgebraicModelPoint(t *testing.T) {
	ctx := context.Background()
	p := triangularPointPatch(t)
	target := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(5, 6), big.NewRat(1, 1)}
	for _, axes := range [][2]int{{0, 1}, {0, 2}} {
		r, e := InvertTriangularSurfacePointRational(ctx, p, target, TriangularSurfaceInverseOptions{RationalSurfaceInverseOptions: RationalSurfaceInverseOptions{ProjectionAxes: axes}})
		if e != nil || !r.Found || !r.Complete || !r.FullModelQuery || !r.ModelPointValidated || r.Parameters[1] != nil || r.ModelBounds.Min.Z != 1 || r.ModelBounds.Max.Z != 1 {
			t.Fatal(r, e)
		}
		r.TargetPoint[0].SetInt64(9)
		if target[0].Cmp(big.NewRat(1, 3)) != 0 {
			t.Fatal("aliased query")
		}
	}
}
func TestTriangularSurfaceProjectedButOffModel(t *testing.T) {
	ctx := context.Background()
	p := triangularPointPatch(t)
	target := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(5, 6), new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80)))}
	projection, e := InvertTriangularSurfaceProjectionRational(ctx, p, [2]*big.Rat{target[0], target[1]}, TriangularSurfaceInverseOptions{})
	if e != nil || !projection.Found || projection.FullModelQuery {
		t.Fatal(projection, e)
	}
	point, e := InvertTriangularSurfacePointRational(ctx, p, target, TriangularSurfaceInverseOptions{})
	if e != nil || !point.Complete || !point.Excluded || point.Found || point.ModelPointValidated || !point.FullModelQuery {
		t.Fatal(point, e)
	}
}
func TestTriangularSurfaceRationalModelPointAndLimits(t *testing.T) {
	ctx := context.Background()
	p := triangularPointPatch(t)
	var e error
	p, e = p.InsertVKnot(.5, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	target := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(13, 12), big.NewRat(3, 2)}
	r, e := InvertTriangularSurfacePointRational(ctx, p, target, TriangularSurfaceInverseOptions{})
	if e != nil || !r.ModelPointValidated || r.Parameters[1].Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal(r, e)
	}
	_, e = InvertTriangularSurfacePointRational(ctx, p, target, TriangularSurfaceInverseOptions{RationalSurfaceInverseOptions: RationalSurfaceInverseOptions{MaxOperations: 1}})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = InvertTriangularSurfacePointRational(canceled, p, target, TriangularSurfaceInverseOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
