package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestExactRootRefinementAndRationalConstructor(t *testing.T) {
	ctx := context.Background()
	root := importTestRoot(t, big.NewRat(-1, 2), true, "2/3", "3/4")
	before := root.Interval()
	width := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 150))
	r, e := root.Enclose(ctx, width, ExactCurvePlaneEventOptions{})
	if e != nil || !r.HasBounds || !r.Converged || new(big.Rat).Sub(r.Interval.Upper, r.Interval.Lower).Cmp(width) > 0 {
		t.Fatal(r, e)
	}
	if new(big.Rat).Mul(r.Interval.Lower, r.Interval.Lower).Cmp(big.NewRat(1, 2)) > 0 || new(big.Rat).Mul(r.Interval.Upper, r.Interval.Upper).Cmp(big.NewRat(1, 2)) < 0 {
		t.Fatal(r)
	}
	if root.Interval().Lower.Cmp(before.Lower) != 0 || root.Interval().Upper.Cmp(before.Upper) != 0 {
		t.Fatal("root mutated")
	}
	limited, e := root.Enclose(ctx, width, ExactCurvePlaneEventOptions{MaxSubdivisions: 1})
	if !errors.Is(e, ErrConvergence) || !limited.HasBounds || limited.Converged {
		t.Fatal(limited, e)
	}
	value := big.NewRat(1, 3)
	rational, _, e := NewExactRationalRoot(ctx, value, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	value.SetInt64(9)
	exact, e := rational.Enclose(ctx, new(big.Rat), ExactCurvePlaneEventOptions{})
	if e != nil || !exact.Converged || exact.Interval.Lower.Cmp(big.NewRat(1, 3)) != 0 {
		t.Fatal(exact, e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = root.Enclose(cancelCtx, width, ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
