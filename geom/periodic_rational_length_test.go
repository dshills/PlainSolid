package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicExactRationalWindowLengthThroughSeam(t *testing.T) {
	ctx := context.Background()
	c, err := NewPeriodicBSpline(1, []Vec3{{}, {X: 3}, {X: 3, Y: 4}, {Y: 4}}, nil, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.ExactCyclicSpline(ctx, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.InsertKnot(ctx, big.NewRat(1, 7), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	seam := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(-1), 101))
	start := new(big.Rat).Sub(seam, big.NewRat(1, 4))
	end := new(big.Rat).Add(seam, big.NewRat(5, 4))
	// Two vertical units before the seam, a complete 3-by-4 corner
	// after it, then half the final horizontal edge: 2+3+4+3/2.
	const want = 10.5
	for i, source := range []interface {
		BoundLengthRational(context.Context, *big.Rat, *big.Rat, PeriodicCurveLengthOptions) (CurveLengthBounds, error)
	}{c, s} {
		bound, err := source.BoundLengthRational(ctx, start, end, PeriodicCurveLengthOptions{})
		if err != nil || !bound.Complete || bound.Lower > want || bound.Upper < want || bound.Upper-bound.Lower > 1e-12 || bound.Spans != i+4 {
			t.Fatal("length bound lost a seam piece or an inserted span", bound, err)
		}
		limited := PeriodicCurveLengthOptions{CurveLengthBoundOptions: CurveLengthBoundOptions{MaxOperations: bound.ExactOperations - 1}}
		if b, err := source.BoundLengthRational(ctx, start, end, limited); !errors.Is(err, ErrConvergence) || b.Complete {
			t.Fatal("one-short total budget reported complete length", b, err)
		}
	}
	if b, err := s.BoundLengthRational(ctx, start, end, PeriodicCurveLengthOptions{MaxKnotSpans: 4}); !errors.Is(err, ErrConvergence) || b.Complete {
		t.Fatal("incomplete exact span extraction reported a full length", b, err)
	}
}
