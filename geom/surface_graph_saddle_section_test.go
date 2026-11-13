package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestQuadraticSaddleFiniteSectionCorners(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	uv := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
	rectangle := [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1)}}
	r, e := ClipQuadraticGraphSaddleSection(context.Background(), a, b, uv, rectangle, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Terminals) != 4 {
		t.Fatal(r, e)
	}
	masks := map[uint8]bool{}
	for _, terminal := range r.Terminals {
		if !terminal.Exact {
			t.Fatal(terminal)
		}
		masks[terminal.SideMask] = true
		x, e := b.EvaluateRational(context.Background(), terminal.Lower[0], terminal.Lower[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for i := range x.Point {
			if x.Point[i].Cmp(terminal.Point[i]) != 0 {
				t.Fatal("terminal mismatch")
			}
		}
	}
	for _, mask := range []uint8{1 | 4, 1 | 8, 2 | 4, 2 | 8} {
		if !masks[mask] {
			t.Fatal(masks)
		}
	}
	// Exact nonbinary rectangle clips each branch on one horizontal side.
	rectangle = [2][2]*big.Rat{{big.NewRat(1, 6), big.NewRat(5, 6)}, {big.NewRat(1, 3), big.NewRat(2, 3)}}
	r, e = ClipQuadraticGraphSaddleSection(context.Background(), a, b, uv, rectangle, GraphPointContactOptions{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for _, terminal := range r.Terminals {
		if !terminal.Exact || (terminal.SideMask != 4 && terminal.SideMask != 8) {
			t.Fatal(terminal)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ClipQuadraticGraphSaddleSection(ctx, a, b, uv, rectangle, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	r, e = ClipQuadraticGraphSaddleSection(context.Background(), a, b, uv, rectangle, GraphPointContactOptions{MaxOperations: 1})
	if e == nil || r.Complete {
		t.Fatal(r, e)
	}
}
func TestQuadraticSaddleFiniteSectionAlgebraicTerminals(t *testing.T) {
	a, b := graphContactFixture(t, false, true)
	n := a.degreeU
	q := func(i int) *big.Rat {
		return new(big.Rat).Add(big.NewRat(1, 4), new(big.Rat).Sub(big.NewRat(int64(i*(i-1)), int64(n*(n-1))), big.NewRat(int64(i), int64(n))))
	}
	for i := 0; i < a.countU; i++ {
		for j := 0; j < a.countV; j++ {
			k := i*a.countV + j
			a.net[k][2] = new(big.Rat).Add(b.net[k][2], new(big.Rat).Sub(q(i), new(big.Rat).Mul(big.NewRat(2, 1), q(j))))
		}
	}
	rectangle := [2][2]*big.Rat{{new(big.Rat), big.NewRat(1, 1)}, {new(big.Rat), big.NewRat(1, 1)}}
	r, e := ClipQuadraticGraphSaddleSection(context.Background(), a, b, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, rectangle, GraphPointContactOptions{})
	if e != nil || !r.Complete || len(r.Terminals) != 4 {
		t.Fatal(r, e)
	}
	for _, terminal := range r.Terminals {
		if terminal.Exact || (terminal.SideMask != 1 && terminal.SideMask != 2) || terminal.Lower[0].Cmp(terminal.Upper[0]) != 0 {
			t.Fatal(terminal)
		}
		// v-.5=±sqrt(1/8); independently verify the rational enclosure.
		lo := new(big.Rat).Sub(terminal.Lower[1], big.NewRat(1, 2))
		hi := new(big.Rat).Sub(terminal.Upper[1], big.NewRat(1, 2))
		if hi.Sign() < 0 {
			lo, hi = new(big.Rat).Neg(hi), new(big.Rat).Neg(lo)
		}
		if new(big.Rat).Mul(lo, lo).Cmp(big.NewRat(1, 8)) > 0 || new(big.Rat).Mul(hi, hi).Cmp(big.NewRat(1, 8)) < 0 {
			t.Fatal(terminal)
		}
	}
}
