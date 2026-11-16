package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedJoinedNormalOffsetSweepEnclosesAffineBoundaryArea(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	charts := [3]*VerifiedNormalOffsetSweep{
		sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}}),
		sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {4, 4}}),
		sweepChainPlanarChart(t, [2][2]int64{{4, 4}, {8, 8}}),
	}
	join := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	pair, chain := joinedSweepMomentFixture(t, charts, join, join)
	for _, test := range []struct {
		size  int
		want  *big.Rat
		faces []VerifiedNormalOffsetSweepBoundaryFace
		query func(context.Context, NormalOffsetSweepAreaEnclosureOptions) (NormalOffsetSweepJoinedBoundaryArea, error)
	}{
		{10, q(26, 5), pair.Boundary(), pair.EncloseBoundaryAreaRational},
		{14, q(38, 5), chain.Boundary(), chain.EncloseBoundaryAreaRational},
	} {
		r, err := test.query(ctx, NormalOffsetSweepAreaEnclosureOptions{NormalOffsetSweepAreaOptions: NormalOffsetSweepAreaOptions{
			AbsoluteError: q(0, 1), MaxCells: test.size,
		}})
		if err != nil || !r.Complete || r.Bounds.Lower.Cmp(test.want) != 0 || r.Bounds.Upper.Cmp(test.want) != 0 {
			t.Fatal("all affine retained cells must close exactly with no omitted visits", r, err)
		}
		checkJoinedSweepAreaOwners(t, r, test.faces)
		t.Logf("exact affine area %s with %d retained root cells and %d operations", r.Bounds.Lower, r.VisitedCells, r.ExactOperations)
	}
}

func TestVerifiedJoinedNormalOffsetSweepEnclosesCurvedBoundaryArea(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	charts, joins := placedSweepDistanceCylinder(t)
	pair, chain := joinedSweepMomentFixture(t, charts, joins[0], joins[1])
	length, theta := referenceParabolicSweepLengthAndTurn()
	lower, upper := q(-1, 11), q(2, 13)
	thickness := new(big.Rat).Sub(upper, lower)
	moment := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(upper, upper), new(big.Rat).Mul(lower, lower)), q(2, 1))
	ends := referenceSweepArcCombination(length, theta, thickness, new(big.Rat).Neg(moment))
	radial := exactInterval{thickness, thickness}
	cap0 := referenceSweepArcCombination(length, theta, q(1, 1), new(big.Rat).Neg(lower))
	cap1 := referenceSweepArcCombination(length, theta, q(1, 1), new(big.Rat).Neg(upper))
	options := NormalOffsetSweepAreaEnclosureOptions{NormalOffsetSweepAreaOptions: NormalOffsetSweepAreaOptions{
		AbsoluteError: q(1, 25),
	}}
	for _, test := range []struct {
		name   string
		charts int64
		faces  []VerifiedNormalOffsetSweepBoundaryFace
		query  func(context.Context, NormalOffsetSweepAreaEnclosureOptions) (NormalOffsetSweepJoinedBoundaryArea, error)
	}{
		{"pair", 2, pair.Boundary(), pair.EncloseBoundaryAreaRational},
		{"chain", 3, chain.Boundary(), chain.EncloseBoundaryAreaRational},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, err := test.query(ctx, options)
			if err != nil || !r.Complete || len(r.Faces) != len(test.faces) ||
				new(big.Rat).Sub(r.Bounds.Upper, r.Bounds.Lower).Cmp(options.AbsoluteError) > 0 {
				t.Fatal("expected independently placed curved outer boundary within the common budget", r, err)
			}
			for i, face := range r.Faces {
				expected := test.faces[i]
				if face.Owner != expected.Owner || face.Side != expected.Side {
					t.Fatal("bounded area lost original chart or side identity", i, face, expected)
				}
				want := radial
				switch face.Side {
				case NormalOffsetSweepLower:
					want = cap0
				case NormalOffsetSweepUpper:
					want = cap1
				default:
					rulingAxis := 0
					if face.Owner == 1 {
						rulingAxis = 1
					}
					if int(face.Side)/2 == rulingAxis {
						want = ends
					}
				}
				if face.Bounds.Lower.Cmp(want.lo) > 0 || face.Bounds.Upper.Cmp(want.hi) < 0 {
					t.Fatal("retained original face misses independent analytic integral", face, want)
				}
			}
			// One unit ruling per chart. The independent union has just two
			// physical end walls and the same original parabolic profile.
			for i, bounds := range [2][3]*big.Rat{{cap0.lo, cap1.lo, ends.lo}, {cap0.hi, cap1.hi, ends.hi}} {
				want := new(big.Rat).Add(bounds[0], bounds[1])
				want.Add(want, new(big.Rat).Mul(thickness, q(2, 1)))
				want.Mul(want, q(test.charts, 1))
				want.Add(want, new(big.Rat).Mul(bounds[2], q(2, 1)))
				if (i == 0 && r.Bounds.Lower.Cmp(want) > 0) || (i == 1 && r.Bounds.Upper.Cmp(want) < 0) {
					t.Fatal("joined area total missed the independent continuous outer material", r.Bounds, want)
				}
			}
			t.Logf("%d original retained faces, area [%s,%s], %d cells and %d exact operations",
				len(r.Faces), r.Bounds.Lower.FloatString(12), r.Bounds.Upper.FloatString(12), r.VisitedCells, r.ExactOperations)

			partialOptions := options
			partialOptions.MaxCells = len(test.faces) - 1
			partial, err := test.query(ctx, partialOptions)
			if !errors.Is(err, ErrConvergence) || partial.Complete || partial.VisitedCells != partialOptions.MaxCells ||
				len(partial.Faces) >= len(test.faces) || partial.Bounds.Lower != nil || partial.Bounds.Upper != nil {
				t.Fatal("incomplete retained original cover published a total", partial, err)
			}
			coarseOptions := options
			coarseOptions.Patch.MaxOperations = r.ExactOperations / 3
			coarse, err := test.query(ctx, coarseOptions)
			if !errors.Is(err, ErrConvergence) || coarse.Complete || len(coarse.Faces) != len(test.faces) ||
				coarse.Bounds.Lower == nil || coarse.Bounds.Upper == nil ||
				coarse.Bounds.Lower.Cmp(r.Bounds.Lower) > 0 || coarse.Bounds.Upper.Cmp(r.Bounds.Upper) < 0 {
				t.Fatal("shared refinement arithmetic must retain the last complete coarse outer cover", coarse, err)
			}
		})
	}
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	result, err := chain.EncloseBoundaryAreaRational(stopped, options)
	if !errors.Is(err, context.Canceled) || result.Complete || len(result.Faces) != 0 || result.Bounds.Upper != nil {
		t.Fatal("cancelled joined area integration published a result", result, err)
	}
}
