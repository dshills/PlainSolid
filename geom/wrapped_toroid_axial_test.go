package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactCircularToroidAxialRoots(t *testing.T) {
	ctx := context.Background()
	w := locationToroid(t, true)
	o := ToroidAxialOptions{AbsoluteTolerance: 1e-12}
	for _, xy := range [][2]float64{{0, 0}, {4, 0}, {1, 1}} {
		r, e := IntersectWrappedCircularToroidAxial(ctx, w, xy[0], xy[1], o)
		if e != nil || len(r) != 0 {
			t.Fatal(xy, r, e)
		}
	}
	for _, xy := range [][2]float64{{2.5, 0}, {1.5, 2}, {1.5, 0}} {
		r, e := IntersectWrappedCircularToroidAxial(ctx, w, xy[0], xy[1], o)
		if e != nil || len(r) != 1 || r[0].Multiplicity != 2 || r[0].MinZ.Sign() != 0 || r[0].MaxZ.Sign() != 0 {
			t.Fatal("tangent", xy, r, e)
		}
	}
	r, e := IntersectWrappedCircularToroidAxial(ctx, w, 2, 0, o)
	if e != nil || len(r) != 2 {
		t.Fatal(r, e)
	}
	for i, want := range []*big.Rat{big.NewRat(-1, 2), big.NewRat(1, 2)} {
		if r[i].MinZ.Cmp(want) != 0 || r[i].MaxZ.Cmp(want) != 0 || r[i].Multiplicity != 1 {
			t.Fatal("exact endpoints", r)
		}
	}
	// rho=9/4 gives z²=1/4−1/16=3/16: compare exact rational
	// squares to the independent radical, with no floating square root oracle.
	r, e = IntersectWrappedCircularToroidAxial(ctx, w, 2.25, 0, o)
	if e != nil || len(r) != 2 {
		t.Fatal(e)
	}
	want := big.NewRat(3, 16)
	if new(big.Rat).Mul(r[1].MinZ, r[1].MinZ).Cmp(want) > 0 || new(big.Rat).Mul(r[1].MaxZ, r[1].MaxZ).Cmp(want) < 0 {
		t.Fatal("radical not enclosed", r)
	}
	if new(big.Rat).Sub(r[1].MaxZ, r[1].MinZ).Cmp(new(big.Rat).SetFloat64(o.AbsoluteTolerance)) > 0 {
		t.Fatal("width")
	}
	if new(big.Rat).Add(r[0].MinZ, r[1].MaxZ).Sign() != 0 || new(big.Rat).Add(r[0].MaxZ, r[1].MinZ).Sign() != 0 {
		t.Fatal("lost exact symmetry")
	}
	bad := o
	bad.MaxBisections = 1
	r, e = IntersectWrappedCircularToroidAxial(ctx, w, 2.25, 0, bad)
	if !errors.Is(e, ErrConvergence) || r != nil {
		t.Fatal("partial limit", r, e)
	}
	bad = o
	bad.Surface.MaxOperations = 1
	r, e = IntersectWrappedCircularToroidAxial(ctx, w, 2, 0, bad)
	if e == nil || r != nil {
		t.Fatal("budget", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	r, e = IntersectWrappedCircularToroidAxial(canceled, w, 2, 0, o)
	if !errors.Is(e, context.Canceled) || r != nil {
		t.Fatal("cancellation", e)
	}
}
