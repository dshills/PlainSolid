package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestPeriodicCurveDistanceOriginalWitness(t *testing.T) {
	ctx := context.Background()
	curve, err := NewPeriodicBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {0, 2, 0}}, []float64{1, .5, 1}, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	cycle := new(big.Int).Lsh(big.NewInt(1), 100)
	seam := new(big.Rat).SetInt(cycle)
	start := new(big.Rat).Sub(seam, big.NewRat(1, 10))
	end := new(big.Rat).Add(seam, big.NewRat(1, 6))
	query := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(0, 1), big.NewRat(1, 7)}
	bound, err := BoundPointPeriodicCurveDistanceRational(ctx, query, curve, start, end, PeriodicCurveDistanceOptions{})
	if err != nil || !bound.Complete || !bound.HasWitness {
		t.Fatalf("bound = %+v, %v", bound, err)
	}
	if bound.OriginalParameter.Cmp(end) != 0 || bound.Cycle.Cmp(cycle) != 0 || bound.Span != 0 {
		t.Fatalf("physical witness = %v, cycle %v, span %v", bound.OriginalParameter, bound.Cycle, bound.Span)
	}
	if bound.Squared.Lower.Cmp(big.NewRat(1, 49)) != 0 || bound.Squared.Upper.Cmp(big.NewRat(1, 49)) != 0 || bound.Witness[0].Cmp(query[0]) != 0 || bound.Witness[1].Sign() != 0 || bound.Witness[2].Sign() != 0 {
		t.Fatalf("squared distance = %+v, witness = %v", bound.Squared, bound.Witness)
	}
	query[0] = big.NewRat(0, 1)
	bound, err = BoundPointPeriodicCurveDistanceRational(ctx, query, curve, start, end, PeriodicCurveDistanceOptions{})
	if err != nil || !bound.HasWitness || bound.OriginalParameter.Cmp(seam) != 0 || bound.Cycle.Cmp(cycle) != 0 || bound.Span != 0 {
		t.Fatalf("seam witness = %+v, %v", bound, err)
	}
}

func TestPeriodicCurveDistanceResourceBounds(t *testing.T) {
	curve, err := NewPeriodicBSpline(1, []Vec3{{0, 0, 0}, {1, 0, 0}, {0, 2, 0}}, nil, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	start, end := big.NewRat(1, 3), big.NewRat(2, 3)
	query := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(0, 1)}
	// The middle linear span lies on 2x+y=2. Orthogonal projection of
	// (1,2) lands strictly inside that span with squared distance 4/5.
	want := big.NewRat(4, 5)
	options := PeriodicCurveDistanceOptions{CurveDistanceOptions: CurveDistanceOptions{MaxCells: 1}}
	partial, err := BoundPointPeriodicCurveDistanceRational(ctx, query, curve, start, end, options)
	if !errors.Is(err, ErrConvergence) || partial.Complete || !partial.HasWitness || partial.Squared.Lower.Cmp(want) > 0 || partial.Squared.Upper.Cmp(want) < 0 {
		t.Fatalf("partial result = %+v, %v", partial, err)
	}
	options.MaxCells = 0
	options.SquaredTolerance = 1e-5
	bound, err := BoundPointPeriodicCurveDistanceRational(ctx, query, curve, start, end, options)
	if err != nil || !bound.Complete || !bound.HasWitness || bound.Squared.Lower.Cmp(want) > 0 || bound.Squared.Upper.Cmp(want) < 0 {
		t.Fatalf("global result = %+v, %v", bound, err)
	}
	// For the raw middle span x=2-3t and y=6t-2. This independently
	// checks the exact map used to return the original witness parameter.
	witnessX := new(big.Rat).Sub(big.NewRat(2, 1), new(big.Rat).Mul(big.NewRat(3, 1), bound.OriginalParameter))
	witnessY := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(6, 1), bound.OriginalParameter), big.NewRat(2, 1))
	if bound.Witness[0].Cmp(witnessX) != 0 || bound.Witness[1].Cmp(witnessY) != 0 || bound.Span != 1 || bound.Cycle.Sign() != 0 {
		t.Fatalf("reported parameter %s does not give witness %v", bound.OriginalParameter, bound.Witness)
	}
	options.MaxOperations = bound.ExactOperations
	exactLimit, err := BoundPointPeriodicCurveDistanceRational(ctx, query, curve, start, end, options)
	if err != nil || !exactLimit.Complete || exactLimit.ExactOperations != bound.ExactOperations {
		t.Fatalf("exact operation limit = %+v, %v", exactLimit, err)
	}
	options.MaxOperations--
	limited, err := BoundPointPeriodicCurveDistanceRational(ctx, query, curve, start, end, options)
	if !errors.Is(err, ErrConvergence) || limited.Complete {
		t.Fatalf("one operation short = %+v, %v", limited, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := BoundPointPeriodicCurveDistanceRational(canceled, query, curve, start, end, options); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}
