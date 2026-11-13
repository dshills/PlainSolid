package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func nontriangularInversePatch(t *testing.T) *SurfacePatch {
	t.Helper()
	var poles []Vec3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := float64(i)/2, float64(j)/2
			u2, v2 := float64(i*(i-1))/2, float64(j*(j-1))/2
			poles = append(poles, Vec3{u + v2/4, v + u2/4, u * v})
		}
	}
	s, e := NewBSplineSurface(2, 2, 3, 3, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestProjectionKrawczykNontriangular(t *testing.T) {
	p := nontriangularInversePatch(t)
	r, e := InvertSurfaceProjectionInterval(context.Background(), p, [2]*big.Rat{big.NewRat(28, 75), big.NewRat(77, 180)}, RationalSurfaceInverseOptions{})
	if e != nil || !r.Found || !r.Complete || !r.Injectivity.ProvenInjective {
		t.Fatal(r, e)
	}
	for axis, x := range []*big.Rat{big.NewRat(1, 3), big.NewRat(2, 5)} {
		if r.Lower[axis].Cmp(x) > 0 || r.Upper[axis].Cmp(x) < 0 {
			t.Fatal(r)
		}
	}
	r, e = InvertSurfaceProjectionInterval(context.Background(), p, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, RationalSurfaceInverseOptions{})
	if e != nil || !r.Found {
		t.Fatal(r, e)
	}
	f := func(x *big.Rat) *big.Rat {
		return new(big.Rat).Sub(new(big.Rat).Add(x, new(big.Rat).Mul(big.NewRat(1, 4), new(big.Rat).Mul(x, x))), big.NewRat(1, 2))
	}
	for axis := 0; axis < 2; axis++ {
		if f(r.Lower[axis]).Sign() > 0 || f(r.Upper[axis]).Sign() < 0 {
			t.Fatal(r)
		}
	}
}
func TestProjectionKrawczykExclusionAndBoundaryPending(t *testing.T) {
	ctx := context.Background()
	p := nontriangularInversePatch(t)
	r, e := InvertSurfaceProjectionInterval(ctx, p, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, RationalSurfaceInverseOptions{})
	if e != nil || !r.Excluded || !r.Complete || r.Found {
		t.Fatal(r, e)
	}
	r, e = InvertSurfaceProjectionInterval(ctx, p, [2]*big.Rat{new(big.Rat), new(big.Rat)}, RationalSurfaceInverseOptions{MaxDepth: 4})
	if !errors.Is(e, ErrConvergence) || r.Found || r.Excluded || len(r.Pending) == 0 {
		t.Fatal(r, e)
	}
	_, e = InvertSurfaceProjectionInterval(ctx, p, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, RationalSurfaceInverseOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = InvertSurfaceProjectionInterval(canceled, p, [2]*big.Rat{new(big.Rat), new(big.Rat)}, RationalSurfaceInverseOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestProjectionKrawczykRefinedKnotCells(t *testing.T) {
	p := nontriangularInversePatch(t)
	var e error
	p, e = p.InsertUKnot(.25, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertVKnot(.75, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := InvertSurfaceProjectionInterval(context.Background(), p, [2]*big.Rat{big.NewRat(28, 75), big.NewRat(77, 180)}, RationalSurfaceInverseOptions{})
	if e != nil || !r.Found || !r.Complete {
		t.Fatal(r, e)
	}
}
