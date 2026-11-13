package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func graphSignRoot(t *testing.T) *ExactGraphLineRoot {
	piece := graphRootPiece(t, rationalPolynomial{big.NewRat(-1, 2), new(big.Rat), big.NewRat(1, 1)}, big.NewRat(0, 1), big.NewRat(1, 1), false)
	root, e := NewExactGraphLineRoot(context.Background(), piece, new(big.Rat), new(big.Rat), ExactParameterInterval{Lower: big.NewRat(0, 1), Upper: big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return root
}
func TestExactGraphRootCapOrdering(t *testing.T) {
	root := graphSignRoot(t)
	for _, test := range []struct {
		cap   *big.Rat
		order int
	}{{big.NewRat(2, 3), 1}, {big.NewRat(3, 4), -1}, {big.NewRat(-1, 1), 1}, {big.NewRat(2, 1), -1}} {
		r, e := CompareExactGraphLineRootRational(context.Background(), root, test.cap, ExactCurvePlaneEventOptions{})
		if e != nil || !r.Proven || r.Order != test.order {
			t.Fatal(r, e)
		}
	}
	piece := graphRootPiece(t, rationalPolynomial{big.NewRat(-1, 2), big.NewRat(1, 1)}, big.NewRat(0, 1), big.NewRat(1, 1), false)
	rational, e := NewExactGraphLineRoot(context.Background(), piece, new(big.Rat), new(big.Rat), ExactParameterInterval{Lower: big.NewRat(0, 1), Upper: big.NewRat(1, 1)}, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := CompareExactGraphLineRootRational(context.Background(), rational, big.NewRat(1, 2), ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Order != 0 {
		t.Fatal(r, e)
	}
}
func TestExactGraphRootSignIndependentTarget(t *testing.T) {
	root := graphSignRoot(t)
	epsilon := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, sign := range []int{-1, 0, 1} {
		constant := new(big.Rat).Add(big.NewRat(1, 4), new(big.Rat).Mul(big.NewRat(int64(sign), 1), epsilon))
		target := graphRootPiece(t, rationalPolynomial{constant, new(big.Rat), big.NewRat(-1, 1), new(big.Rat), big.NewRat(1, 1)}, big.NewRat(1, 3), big.NewRat(1, 1), true)
		r, e := ClassifyExactGraphRootHeight(context.Background(), root, target, new(big.Rat), new(big.Rat), ExactCurvePlaneEventOptions{})
		if e != nil || !r.Proven || r.Side != sign {
			t.Fatal(r, e)
		}
		if sign != 0 {
			limited, e := ClassifyExactGraphRootHeight(context.Background(), root, target, new(big.Rat), new(big.Rat), ExactCurvePlaneEventOptions{MaxSubdivisions: 2})
			if e == nil || limited.Proven {
				t.Fatal(limited, e)
			}
		}
	}
	// Identically coincident target does not need isolated target roots.
	target := graphRootPiece(t, rationalPolynomial{big.NewRat(1, 1), big.NewRat(1, 1)}, big.NewRat(0, 1), big.NewRat(1, 1), true)
	r, e := ClassifyExactGraphRootHeight(context.Background(), root, target, big.NewRat(1, 1), big.NewRat(2, 1), ExactCurvePlaneEventOptions{})
	if e != nil || !r.Proven || r.Side != 0 {
		t.Fatal(r, e)
	}
}
func TestExactGraphRootSignOutsidePieceAndCancellation(t *testing.T) {
	root := graphSignRoot(t)
	target := graphRootPiece(t, rationalPolynomial{big.NewRat(1, 1)}, big.NewRat(0, 1), big.NewRat(1, 2), false)
	r, e := ClassifyExactGraphRootHeight(context.Background(), root, target, new(big.Rat), new(big.Rat), ExactCurvePlaneEventOptions{})
	if !errors.Is(e, ErrDomain) || r.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = ClassifyExactGraphRootHeight(ctx, root, target, new(big.Rat), new(big.Rat), ExactCurvePlaneEventOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
