package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicSurfaceLineOriginalParameterIsolation(t *testing.T) {
	surface := periodicDistanceSurface(t)
	ctx := context.Background()
	cycle := new(big.Int).Lsh(big.NewInt(1), 100)
	seam := new(big.Rat).SetInt(cycle)
	u := ExactParameterInterval{Lower: new(big.Rat).Sub(seam, big.NewRat(1, 10)), Upper: new(big.Rat).Add(seam, big.NewRat(2, 3))}
	v := ExactParameterInterval{Lower: big.NewRat(-1, 2), Upper: big.NewRat(2, 3)}
	parameter := ExactParameterInterval{Lower: big.NewRat(1, 11), Upper: big.NewRat(31, 6)}
	origin := [3]*big.Rat{big.NewRat(1, 5), big.NewRat(-1, 4), big.NewRat(5, 2)}
	direction := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(3, 7), big.NewRat(0, 1)}
	out, err := IntersectRationalLinePeriodicSurface(ctx, origin, direction, surface, u, v, parameter, PeriodicSurfaceLineOptions{})
	if err != nil || !out.Complete || len(out.Hits) != 2 || len(out.Pending) != 0 {
		t.Fatalf("expected two proved contacts, got %+v, %v", out, err)
	}
	want := []struct {
		offset, parameter, y *big.Rat
	}{
		{big.NewRat(1, 30), big.NewRat(7, 12), big.NewRat(0, 1)},
		{big.NewRat(19, 30), big.NewRat(287, 60), big.NewRat(9, 5)},
	}
	for i, hit := range out.Hits {
		if hit.U.Span != i || hit.U.Cycle.Cmp(cycle) != 0 || hit.V.Span != 0 || hit.V.Cycle != nil {
			t.Fatalf("wrong original owners for hit %d: %+v and %+v", i, hit.U, hit.V)
		}
		assertRationalParameterBracket(t, hit.Isolation.U, new(big.Rat).Add(seam, want[i].offset))
		assertRationalParameterBracket(t, hit.Isolation.V, big.NewRat(-1, 8))
		assertRationalParameterBracket(t, hit.Isolation.Parameter, want[i].parameter)
		assertExactPointInBox(t, [3]*big.Rat{origin[0], want[i].y, origin[2]}, hit.Bounds)
	}
	options := PeriodicSurfaceLineOptions{MaxSearchCells: 1}
	partial, err := IntersectRationalLinePeriodicSurface(ctx, origin, direction, surface, u, v, parameter, options)
	if !errors.Is(err, ErrConvergence) || partial.Complete || partial.VisitedCells != 1 || len(partial.Pending) < 2 {
		t.Fatalf("search-cell exhaustion omitted possible contacts: %+v, %v", partial, err)
	}
	for i, w := range want {
		found := false
		for _, p := range partial.Pending {
			if p.U.Span == i && p.V.Span == 0 && p.U.Cycle.Cmp(cycle) == 0 {
				assertRationalParameterBracket(t, p.Window.U, new(big.Rat).Add(seam, w.offset))
				assertRationalParameterBracket(t, p.Window.Parameter, w.parameter)
				found = true
			}
		}
		if !found {
			t.Fatalf("pending cells omitted physical contact %d", i)
		}
	}
	options.PeriodicSurfaceBezierOptions.MaxCells = 1
	partial, err = IntersectRationalLinePeriodicSurface(ctx, origin, direction, surface, u, v, parameter, options)
	if !errors.Is(err, ErrConvergence) || partial.Complete || len(partial.Pending) != 1 || partial.Pending[0].U.Span != -1 {
		t.Fatalf("grid exhaustion should retain the original search window: %+v, %v", partial, err)
	}
	assertRationalParameterBracket(t, partial.Pending[0].Window.U, u.Lower)
	assertRationalParameterBracket(t, partial.Pending[0].Window.U, u.Upper)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	partial, err = IntersectRationalLinePeriodicSurface(canceled, origin, direction, surface, u, v, parameter, PeriodicSurfaceLineOptions{})
	if !errors.Is(err, context.Canceled) || partial.Complete || len(partial.Pending) != 1 {
		t.Fatalf("cancellation dropped the whole search window: %+v, %v", partial, err)
	}
}

func TestPeriodicSurfaceLineCoincidenceIsPending(t *testing.T) {
	surface := periodicDistanceSurface(t)
	ctx := context.Background()
	u := ExactParameterInterval{Lower: big.NewRat(0, 1), Upper: big.NewRat(1, 3)}
	v := ExactParameterInterval{Lower: big.NewRat(-1, 2), Upper: big.NewRat(1, 4)}
	parameter := ExactParameterInterval{Lower: big.NewRat(0, 1), Upper: big.NewRat(1, 1)}
	origin := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(0, 1), big.NewRat(2, 1)}
	direction := [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 1)}
	out, err := IntersectRationalLinePeriodicSurface(ctx, origin, direction, surface, u, v, parameter, PeriodicSurfaceLineOptions{MaxDepth: 2})
	if !errors.Is(err, ErrConvergence) || out.Complete || len(out.Hits) != 0 || len(out.Pending) == 0 {
		t.Fatalf("coincident line lost its pending cells: %+v, %v", out, err)
	}
}

func TestPeriodicSurfaceLineKeepsBothSeamOwners(t *testing.T) {
	surface := periodicDistanceSurface(t)
	u := ExactParameterInterval{Lower: big.NewRat(-1, 6), Upper: big.NewRat(1, 6)}
	v := ExactParameterInterval{Lower: big.NewRat(-1, 2), Upper: big.NewRat(1, 4)}
	parameter := ExactParameterInterval{Lower: big.NewRat(0, 1), Upper: big.NewRat(1, 1)}
	origin := [3]*big.Rat{big.NewRat(-1, 4), big.NewRat(-1, 4), big.NewRat(5, 2)}
	direction := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(0, 1)}
	out, err := IntersectRationalLinePeriodicSurface(context.Background(), origin, direction, surface, u, v, parameter, PeriodicSurfaceLineOptions{})
	if err != nil || !out.Complete || len(out.Hits) != 2 || len(out.Pending) != 0 {
		t.Fatalf("seam proof should retain both incidences: %+v, %v", out, err)
	}
	for i, hit := range out.Hits {
		if hit.U.Cycle.Cmp(big.NewInt(int64(i-1))) != 0 || hit.U.Span != 2*(1-i) {
			t.Fatalf("wrong seam owner %d: %+v", i, hit.U)
		}
		assertRationalParameterBracket(t, hit.Isolation.U, big.NewRat(0, 1))
		assertRationalParameterBracket(t, hit.Isolation.V, big.NewRat(-1, 8))
		assertRationalParameterBracket(t, hit.Isolation.Parameter, big.NewRat(1, 4))
	}
}

func assertRationalParameterBracket(t *testing.T, got ExactParameterInterval, want *big.Rat) {
	t.Helper()
	if got.Lower == nil || got.Upper == nil || got.Lower.Cmp(want) > 0 || got.Upper.Cmp(want) < 0 {
		t.Fatalf("range %+v misses %v", got, want)
	}
}
