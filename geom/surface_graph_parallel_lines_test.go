package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestParallelGraphLinesExhaustiveBands(t *testing.T) {
	power := []*big.Rat{big.NewRat(2, 9), big.NewRat(-1, 1), big.NewRat(11, 9), big.NewRat(-1, 1), big.NewRat(1, 1)}
	for axis := 0; axis < 2; axis++ {
		a, b := factoredGraphLineFixture(t, axis, power)
		for swap, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			r, e := DiscoverSplineGraphParallelLines(context.Background(), pair[0], pair[1], axis, GraphFactorAxisLineOptions{})
			if e != nil || !r.Proven || len(r.Lines) != 2 || len(r.Bands) != 3 {
				t.Fatal(r, e)
			}
			for i, line := range r.Lines {
				if line.Coordinate.Cmp(big.NewRat(int64(i+1), 3)) != 0 {
					t.Fatal(line)
				}
			}
			sign := 1
			if swap == 1 {
				sign = -1
			}
			for i, band := range r.Bands {
				want := sign
				if i == 1 {
					want = -sign
				}
				if band.Side != want {
					t.Fatal(band)
				}
				uv := [2]*big.Rat{big.NewRat(2, 5), big.NewRat(2, 5)}
				uv[axis] = new(big.Rat).Quo(new(big.Rat).Add(band.Start, band.End), big.NewRat(2, 1))
				x, e := pair[0].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				y, e := pair[1].EvaluateRational(context.Background(), uv[0], uv[1], SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if new(big.Rat).Sub(x.Point[2], y.Point[2]).Sign() != band.Side {
					t.Fatal("independent sign")
				}
			}
		}
	}
}
func TestParallelGraphLinesKnotsAndStrictSeparation(t *testing.T) {
	a, b := factoredGraphLineFixture(t, 0, []*big.Rat{big.NewRat(-3, 32), big.NewRat(11, 16), big.NewRat(-3, 2), big.NewRat(1, 1)})
	r, e := DiscoverSplineGraphParallelLines(context.Background(), a, b, 0, GraphFactorAxisLineOptions{})
	if e != nil || !r.Proven || len(r.Lines) != 3 || !r.Lines[1].OnKnot {
		t.Fatal(r, e)
	}
	a, b = factoredGraphLineFixture(t, 0, []*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(1, 1)})
	r, e = DiscoverSplineGraphParallelLines(context.Background(), a, b, 0, GraphFactorAxisLineOptions{})
	if e != nil || !r.Proven || len(r.Lines) != 0 || len(r.Bands) != 1 || r.Bands[0].Side != 1 {
		t.Fatal(r, e)
	}
}
func TestParallelGraphLinesConservativeFailures(t *testing.T) {
	for _, p := range [][]*big.Rat{{big.NewRat(1, 9), big.NewRat(-2, 3), big.NewRat(1, 1)}, {big.NewRat(-1, 2), big.NewRat(0, 1), big.NewRat(1, 1)}} {
		a, b := factoredGraphLineFixture(t, 0, p)
		r, e := DiscoverSplineGraphParallelLines(context.Background(), a, b, 0, GraphFactorAxisLineOptions{})
		if !errors.Is(e, ErrConvergence) || r.Proven || len(r.Repeated)+len(r.Unrecovered) == 0 {
			t.Fatal(r, e)
		}
	}
	a, b := factoredGraphLineFixture(t, 0, []*big.Rat{big.NewRat(2, 9), big.NewRat(-1, 1), big.NewRat(1, 1)})
	for _, o := range []GraphFactorAxisLineOptions{{MaxCandidates: 1}, {GraphPointContactOptions: GraphPointContactOptions{MaxOperations: 1}}} {
		r, e := DiscoverSplineGraphParallelLines(context.Background(), a, b, 0, o)
		if e == nil || r.Proven {
			t.Fatal(r, e)
		}
	}
	tiny := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 80))
	for i := range a.net {
		a.net[i][2].Add(a.net[i][2], new(big.Rat).Mul(tiny, a.net[i][1]))
	}
	r, e := DiscoverSplineGraphParallelLines(context.Background(), a, b, 0, GraphFactorAxisLineOptions{})
	if e == nil || r.Proven {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = DiscoverSplineGraphParallelLines(ctx, a, b, 0, GraphFactorAxisLineOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
