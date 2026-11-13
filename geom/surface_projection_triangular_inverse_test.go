package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func triangularInversePatch(t *testing.T, swap bool) *SurfacePatch {
	t.Helper()
	nu, nv, du, dv := 2, 3, 1, 2
	ku, kv := []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}
	if swap {
		nu, nv, du, dv = 3, 2, 2, 1
		ku, kv = kv, ku
	}
	var poles []Vec3
	for i := 0; i < nu; i++ {
		for j := 0; j < nv; j++ {
			fixed, vary := i, j
			if swap {
				fixed, vary = j, i
			}
			x := float64(fixed)
			y := x + []float64{0, .5, 2}[vary]
			z := x * float64(vary) / 2
			poles = append(poles, Vec3{x, y, z})
		}
	}
	s, e := NewBSplineSurface(du, dv, nu, nv, poles, ku, kv, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestTriangularSurfaceRationalInverse(t *testing.T) {
	for _, swap := range []bool{false, true} {
		p := triangularInversePatch(t, swap)
		fixed := 0
		if swap {
			fixed = 1
		}
		var e error
		p, e = p.InsertUKnot(.5, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.InsertVKnot(.5, 1, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		r, e := InvertTriangularSurfaceProjectionRational(context.Background(), p, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(67, 75)}, TriangularSurfaceInverseOptions{FixedParameterAxis: fixed})
		if e != nil || !r.Found || !r.Complete || r.Parameters[fixed].Cmp(big.NewRat(1, 3)) != 0 || r.Parameters[1-fixed].Cmp(big.NewRat(2, 5)) != 0 {
			t.Fatal(r, e)
		}
		r, e = InvertTriangularSurfaceProjectionRational(context.Background(), p, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(5, 4)}, TriangularSurfaceInverseOptions{FixedParameterAxis: fixed})
		if e != nil || !r.Found || r.Parameters[1-fixed].Cmp(big.NewRat(1, 2)) != 0 {
			t.Fatal(r, e)
		}
	}
}
func TestTriangularSurfaceAlgebraicAndExclusion(t *testing.T) {
	ctx := context.Background()
	p := triangularInversePatch(t, false)
	r, e := InvertTriangularSurfaceProjectionRational(ctx, p, [2]*big.Rat{big.NewRat(1, 3), big.NewRat(5, 6)}, TriangularSurfaceInverseOptions{})
	if e != nil || !r.Found || !r.Complete || r.Parameters[1] != nil {
		t.Fatal(r, e)
	}
	residual := func(x *big.Rat) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Add(new(big.Rat).Mul(x, x), x), big.NewRat(1, 2))
	}
	if residual(r.Lower[1]).Sign() >= 0 || residual(r.Upper[1]).Sign() <= 0 {
		t.Fatal(r)
	}
	for _, q := range [][2]*big.Rat{{big.NewRat(2, 1), new(big.Rat)}, {big.NewRat(1, 3), big.NewRat(-1, 1)}} {
		r, e = InvertTriangularSurfaceProjectionRational(ctx, p, q, TriangularSurfaceInverseOptions{})
		if e != nil || !r.Excluded || !r.Complete || r.Found {
			t.Fatal(r, e)
		}
	}
}
func TestTriangularSurfaceBoundaryAndUnsupported(t *testing.T) {
	ctx := context.Background()
	p := triangularInversePatch(t, false)
	for _, q := range [][2]*big.Rat{{new(big.Rat), new(big.Rat)}, {big.NewRat(1, 1), big.NewRat(3, 1)}} {
		r, e := InvertTriangularSurfaceProjectionRational(ctx, p, q, TriangularSurfaceInverseOptions{})
		if e != nil || !r.Found {
			t.Fatal(r, e)
		}
	}
	_, e := InvertTriangularSurfaceProjectionRational(ctx, p, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, TriangularSurfaceInverseOptions{FixedParameterAxis: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	_, e = InvertTriangularSurfaceProjectionRational(ctx, p, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, TriangularSurfaceInverseOptions{RationalSurfaceInverseOptions: RationalSurfaceInverseOptions{MaxOperations: 1}})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = InvertTriangularSurfaceProjectionRational(canceled, p, [2]*big.Rat{new(big.Rat), new(big.Rat)}, TriangularSurfaceInverseOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
