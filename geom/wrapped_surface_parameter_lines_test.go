package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestWrappedSurfaceParameterLines(t *testing.T) {
	ctx := context.Background()
	a, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := a.InsertKnot(ctx, 0, .125, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, _, e := CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q := func(x int64) *big.Rat { return big.NewRat(x, 1) }
	x := [2][2]*big.Rat{{q(0), q(-1)}, {q(4), q(2)}}
	y := [2][2]*big.Rat{{q(4), q(-1)}, {q(8), q(2)}}
	if same, e := r.SameParameterLines(ctx, x, y, SurfacePatchOptions{}); e != nil || !same {
		t.Fatal(same, e)
	}
	y[1][0] = q(12)
	if same, e := r.SameParameterLines(ctx, x, y, SurfacePatchOptions{}); e != nil || same {
		t.Fatal("varying whole turns", same, e)
	}
	y[0][0] = big.NewRat(1, 2)
	y[1][0] = big.NewRat(9, 2)
	if same, e := r.SameParameterLines(ctx, x, y, SurfacePatchOptions{}); e != nil || same {
		t.Fatal("fractional turn", same, e)
	}
	y[0][0] = q(4)
	y[1][0] = q(8)
	if same, e := r.SameParameterLines(ctx, x, y, SurfacePatchOptions{MaxOperations: 1}); e == nil || same {
		t.Fatal("budget", same, e)
	}
}
