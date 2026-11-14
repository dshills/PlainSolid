package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestWrappedRectangleArrangement(t *testing.T) {
	ctx := context.Background()
	a, _, e := NewWrappedSurfacePatch(ctx, wrappedCylinderPatch(t), WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	b, _, e := a.InsertKnot(ctx, 0, .125, 1, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	relation, _, e := CompareWrappedSurfaceCharts(ctx, a, b, ExactSurfaceComparisonOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rect := func(u0, u1, v0, v1 int64) [2][2]*big.Rat {
		return [2][2]*big.Rat{{big.NewRat(u0, 1), big.NewRat(u1, 1)}, {big.NewRat(v0, 1), big.NewRat(v1, 1)}}
	}
	r, e := relation.ArrangeRectanglesRational(ctx, rect(3, 5, 0, 2), rect(0, 2, 1, 3), WrappedRectangleOverlapOptions{})
	if e != nil || !r.Complete || len(r.Cells) != 7 || r.CanceledEdges != 8 || r.TargetPeriodShift[0].Cmp(big.NewInt(1)) != 0 {
		t.Fatal(r, e)
	}
	area := func(edges []WrappedRectangleEdge) *big.Rat {
		sum := new(big.Rat)
		for i, edge := range edges {
			next := edges[(i+1)%len(edges)]
			for axis := 0; axis < 2; axis++ {
				if edge.End[axis].Cmp(next.Start[axis]) != 0 {
					t.Fatal("open cycle")
				}
			}
			sum.Add(sum, new(big.Rat).Sub(new(big.Rat).Mul(edge.Start[0], edge.End[1]), new(big.Rat).Mul(edge.End[0], edge.Start[1])))
		}
		return sum.Quo(sum, big.NewRat(2, 1))
	}
	if area(r.Union).Cmp(big.NewRat(7, 1)) != 0 || area(r.Intersection).Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("wrong exact areas")
	}
	for _, cycle := range [][]WrappedRectangleEdge{r.Union, r.Intersection} {
		for _, edge := range cycle {
			if edge.Owners == 0 {
				t.Fatal("missing owner")
			}
			for end, p := range [2][2]*big.Rat{edge.Start, edge.End} {
				if edge.Owners&1 != 0 {
					for axis := 0; axis < 2; axis++ {
						if edge.AUV[end][axis].Cmp(p[axis]) != 0 {
							t.Fatal("A owner")
						}
					}
				}
				if edge.Owners&2 != 0 {
					if new(big.Rat).Add(edge.BUV[end][0], big.NewRat(4, 1)).Cmp(p[0]) != 0 || edge.BUV[end][1].Cmp(p[1]) != 0 {
						t.Fatal("B original owner", edge)
					}
				}
			}
		}
	}
	reversed, _, err := b.Reparameterize(ctx, ParameterRectangle{UMin: 2, UMax: 10, VMin: -2, VMax: 3}, true, false, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	reverseRelation, _, err := CompareWrappedSurfaceCharts(ctx, a, reversed, ExactSurfaceComparisonOptions{NormalizeDomains: true, ReverseBU: true})
	if err != nil {
		t.Fatal(err)
	}
	reversedResult, err := reverseRelation.ArrangeRectanglesRational(ctx, rect(0, 1, 0, 2), rect(7, 9, 1, 3), WrappedRectangleOverlapOptions{})
	if err != nil || area(reversedResult.Union).Cmp(big.NewRat(7, 2)) != 0 || area(reversedResult.Intersection).Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("reversed chart", reversedResult, err)
	}
	for _, edge := range reversedResult.Union {
		if edge.Owners&2 == 0 {
			continue
		}
		for end, p := range [2][2]*big.Rat{edge.Start, edge.End} {
			want := new(big.Rat).Sub(big.NewRat(10, 1), new(big.Rat).Mul(big.NewRat(2, 1), p[0]))
			if edge.BUV[end][0].Cmp(want) != 0 {
				t.Fatal("reversed owner units", edge)
			}
		}
	}
	// Coincident boundaries retain both owners rather than disappearing.
	same, e := relation.ArrangeRectanglesRational(ctx, rect(0, 1, 0, 1), rect(0, 1, 0, 1), WrappedRectangleOverlapOptions{})
	if e != nil || len(same.Union) != 4 {
		t.Fatal(same, e)
	}
	for _, edge := range same.Union {
		if edge.Owners != 3 {
			t.Fatal(edge)
		}
	}
	for _, pair := range [][2][2][2]*big.Rat{{rect(0, 1, 0, 1), rect(1, 2, 0, 1)}, {rect(0, 1, 0, 1), rect(1, 2, 1, 2)}, {rect(0, 3, 0, 1), rect(2, 4, 0, 1)}, {rect(0, 8, 0, 1), rect(0, 1, 0, 1)}} {
		bad, e := relation.ArrangeRectanglesRational(ctx, pair[0], pair[1], WrappedRectangleOverlapOptions{})
		if !errors.Is(e, ErrDomain) || bad.Complete || len(bad.Cells) != 0 {
			t.Fatal("unsupported arrangement", bad, e)
		}
	}
	bad, e := relation.ArrangeRectanglesRational(ctx, rect(3, 5, 0, 2), rect(0, 2, 1, 3), WrappedRectangleOverlapOptions{MaxOperations: 1})
	if e == nil || bad.Complete || len(bad.Union) != 0 {
		t.Fatal("budget", bad, e)
	}
}
