package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func periodicDistanceSurface(t *testing.T) *PeriodicBSplineSurface {
	t.Helper()
	data := BSplineSurfaceData{
		DegreeU: 1, DegreeV: 1, CountU: 3, CountV: 3,
		KnotsV: []float64{-2, -2, .25, 1, 1},
		Poles:  []Vec3{{0, 0, 0}, {0, 0, 3}, {0, 0, 4}, {2, 0, 0}, {2, 0, 3}, {2, 0, 4}, {0, 2, 0}, {0, 2, 3}, {0, 2, 4}},
	}
	surface, err := NewPeriodicBSplineSurface(data, &PeriodicSplineAxis{Period: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return surface
}

func TestPeriodicSurfaceDistanceRetainsOriginalWitness(t *testing.T) {
	surface := periodicDistanceSurface(t)
	ctx := context.Background()
	cycle := new(big.Int).Lsh(big.NewInt(1), 100)
	seam := new(big.Rat).SetInt(cycle)
	u := ExactParameterInterval{Lower: new(big.Rat).Sub(seam, big.NewRat(1, 6)), Upper: new(big.Rat).Add(seam, big.NewRat(1, 3))}
	v := ExactParameterInterval{Lower: big.NewRat(-1, 2), Upper: big.NewRat(2, 3)}
	query := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 7), big.NewRat(5, 2)}
	out, err := BoundPointPeriodicSurfaceDistanceRational(ctx, query, surface, u, v, PeriodicSurfaceDistanceOptions{})
	if err != nil || !out.Complete || !out.HasUpperPoint {
		t.Fatalf("distance result = %+v, %v", out, err)
	}
	wantU := new(big.Rat).Add(seam, big.NewRat(1, 6))
	if out.OriginalUV[0].Cmp(wantU) != 0 || out.OriginalUV[1].Cmp(big.NewRat(-1, 8)) != 0 || out.U.Cycle.Cmp(cycle) != 0 || out.U.Span != 0 || out.V.Cycle != nil || out.V.Span != 0 {
		t.Fatalf("unexpected original witness: %v with owners %+v and %+v", out.OriginalUV, out.U, out.V)
	}
	if out.Witness[0].Cmp(query[0]) != 0 || out.Witness[1].Sign() != 0 || out.Witness[2].Cmp(query[2]) != 0 || out.Squared.Lower.Cmp(big.NewRat(1, 49)) != 0 || out.Squared.Upper.Cmp(big.NewRat(1, 49)) != 0 {
		t.Fatalf("distance = %+v, witness %v", out.Squared, out.Witness)
	}
	limited := PeriodicSurfaceDistanceOptions{SurfaceDistanceOptions: SurfaceDistanceOptions{MaxOperations: out.ExactOperations}}
	exact, err := BoundPointPeriodicSurfaceDistanceRational(ctx, query, surface, u, v, limited)
	if err != nil || !exact.Complete || exact.ExactOperations != out.ExactOperations {
		t.Fatalf("exact work limit = %+v, %v", exact, err)
	}
	limited.MaxOperations--
	short, err := BoundPointPeriodicSurfaceDistanceRational(ctx, query, surface, u, v, limited)
	if !errors.Is(err, ErrConvergence) || short.Complete {
		t.Fatalf("one operation short = %+v, %v", short, err)
	}
}

func TestPeriodicSurfaceDistanceRetainsPartialBounds(t *testing.T) {
	surface := periodicDistanceSurface(t)
	ctx := context.Background()
	u := ExactParameterInterval{Lower: big.NewRat(1, 3), Upper: big.NewRat(2, 3)}
	v := ExactParameterInterval{Lower: big.NewRat(-1, 2), Upper: big.NewRat(1, 4)}
	query := [3]*big.Rat{big.NewRat(2, 1), big.NewRat(2, 1), big.NewRat(5, 2)}
	options := PeriodicSurfaceDistanceOptions{SurfaceDistanceOptions: SurfaceDistanceOptions{MaxCells: 1, SquaredTolerance: .01}}
	partial, err := BoundPointPeriodicSurfaceDistanceRational(ctx, query, surface, u, v, options)
	// The orthogonal projection onto x+y=2 lies in the interior of the
	// patch. The true squared distance is 2.
	want := big.NewRat(2, 1)
	if !errors.Is(err, ErrConvergence) || partial.Complete || !partial.HasUpperPoint || partial.VisitedCells != 1 || partial.Squared.Lower.Cmp(want) > 0 || partial.Squared.Upper.Cmp(want) < 0 {
		t.Fatalf("partial result = %+v, %v", partial, err)
	}
	options.MaxCells = 0
	full, err := BoundPointPeriodicSurfaceDistanceRational(ctx, query, surface, u, v, options)
	if err != nil || !full.Complete || !full.HasUpperPoint || full.Squared.Lower.Cmp(want) > 0 || full.Squared.Upper.Cmp(want) < 0 {
		t.Fatalf("global result = %+v, %v", full, err)
	}
	if full.OriginalUV[0].Cmp(big.NewRat(1, 2)) != 0 || full.OriginalUV[1].Cmp(big.NewRat(-1, 8)) != 0 || full.U.Span != 1 || full.V.Span != 0 {
		t.Fatalf("closest point has wrong raw UV or owner: %+v", full)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := BoundPointPeriodicSurfaceDistanceRational(canceled, query, surface, u, v, options); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}
