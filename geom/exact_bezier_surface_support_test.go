package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactBezierSurfaceSupportComparison(t *testing.T) {
	ctx := context.Background()
	// A=(u,v,0). B multiplies all homogeneous coordinates by 1+u,
	// raising U degree to two with genuinely nonproportional positive weights.
	makeSurface := func(degree int, net [][4]*big.Rat) *ExactBezierSurface {
		p, e := NewExactHomogeneousBezierSurface(ctx, degree, 1, net, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		recipe, e := p.ExportExactBezierRecipe()
		if e != nil {
			t.Fatal(e)
		}
		p, e = ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		s, e := NewExactBezierSurface(p)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	net := func(xs, ws []*big.Rat) [][4]*big.Rat {
		var n [][4]*big.Rat
		for i, x := range xs {
			for j := 0; j < 2; j++ {
				n = append(n, [4]*big.Rat{new(big.Rat).Set(x), new(big.Rat).Mul(ws[i], big.NewRat(int64(j), 1)), new(big.Rat), new(big.Rat).Set(ws[i])})
			}
		}
		return n
	}
	a := makeSurface(1, net([]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}, []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}))
	bnet := net([]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 2), big.NewRat(2, 1)}, []*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(2, 1)})
	b := makeSurface(2, bnet)
	r, e := a.Compare(ctx, b, ExactSurfaceComparisonOptions{})
	if e != nil || !r.Complete || !r.Equal || r.ChartOrientation != 1 {
		t.Fatalf("equality: %+v %v", r, e)
	}
	// Reverse actual B coefficients, requiring an explicit orientation reversing map.
	for j := 0; j < 2; j++ {
		bnet[j], bnet[4+j] = bnet[4+j], bnet[j]
	}
	reversed := makeSurface(2, bnet)
	r, e = a.Compare(ctx, reversed, ExactSurfaceComparisonOptions{ReverseBU: true})
	if e != nil || !r.Equal || r.ChartOrientation != -1 {
		t.Fatalf("reverse: %+v %v", r, e)
	}
	r, e = a.Compare(ctx, reversed, ExactSurfaceComparisonOptions{})
	if e != nil || !r.Complete || r.Equal || r.ChartOrientation != 0 {
		t.Fatalf("different: %+v %v", r, e)
	}
	bnet[2][2] = big.NewRat(1, 1000000)
	perturbed := makeSurface(2, bnet)
	r, e = a.Compare(ctx, perturbed, ExactSurfaceComparisonOptions{ReverseBU: true})
	if e != nil || !r.Complete || r.Equal {
		t.Fatalf("perturbation: %+v %v", r, e)
	}
	r, e = a.Compare(ctx, b, ExactSurfaceComparisonOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete || r.ChartOrientation != 0 {
		t.Fatalf("budget: %+v %v", r, e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = a.Compare(cancelled, b, ExactSurfaceComparisonOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatalf("cancel: %+v %v", r, e)
	}
	got, e := b.Bounds()
	want, we := b.ExactPatch().Bounds()
	if e != nil || we != nil || got != want {
		t.Fatalf("bounds: %+v %+v %v", got, want, e)
	}
	var empty *ExactBezierSurface
	if _, e = empty.Bounds(); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e = a.Compare(ctx, empty, ExactSurfaceComparisonOptions{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
