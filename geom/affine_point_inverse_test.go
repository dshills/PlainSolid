package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestAffineInvertRationalPoint(t *testing.T) {
	ctx := context.Background()
	m := Affine{{3, 1, 0, 1}, {0, 2, 1, -1}, {1, 0, 2, .5}}
	world := [3]*big.Rat{big.NewRat(8, 5), big.NewRat(-43, 35), big.NewRat(83, 42)}
	want := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(-2, 5), big.NewRat(4, 7)}
	local, e := m.InvertRationalPoint(ctx, world, AffineChartInverseOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, x := range local {
		if x.Cmp(want[i]) != 0 {
			t.Fatal("exact inverse point", local, want)
		}
	}
	if _, e := m.InvertRationalPoint(ctx, world, AffineChartInverseOptions{MaxOperations: 1}); !errors.Is(e, ErrConvergence) {
		t.Fatal("work limit", e)
	}
	m[1] = m[0]
	if _, e := m.InvertRationalPoint(ctx, world, AffineChartInverseOptions{}); !errors.Is(e, ErrDegenerate) {
		t.Fatal("singular mapping", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := m.InvertRationalPoint(canceled, world, AffineChartInverseOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal("canceled inverse", e)
	}
}
