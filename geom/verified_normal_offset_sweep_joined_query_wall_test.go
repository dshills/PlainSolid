package geom

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"
)

func TestVerifiedJoinedNormalOffsetSweepLocatesNonRuledObliqueWalls(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	sweeps := placedSweepChainFixture(t)
	first, second := sweepChainPlacedJoins()
	pair, chain := joinedSweepMomentFixture(t, sweeps, first, second)
	zero, apex := q(0, 1), q(14, 5)
	for _, test := range []struct {
		name     string
		x, y, z  *big.Rat
		d        *big.Rat
		location NormalOffsetSweepLocation
		owners   []int
		stations [][2]*big.Rat
		faces    [][]NormalOffsetSweepSide
	}{
		{"first wall", q(5, 8), zero, q(-1, 17), q(-1, 17), NormalOffsetSweepInterior, []int{0, 1},
			[][2]*big.Rat{{q(3, 1), zero}, {apex, q(-1, 1)}}, nil},
		{"second wall", q(13, 8), zero, q(1, 17), q(1, 17), NormalOffsetSweepInterior, []int{1, 2},
			[][2]*big.Rat{{apex, zero}, {q(2, 1), zero}}, nil},
		{"first lower cap", q(5, 8), zero, q(-1, 11), q(-1, 11), NormalOffsetSweepBoundary, []int{0, 1},
			[][2]*big.Rat{{q(3, 1), zero}, {apex, q(-1, 1)}},
			[][]NormalOffsetSweepSide{{NormalOffsetSweepLower}, {NormalOffsetSweepLower}}},
		{"second upper cap", q(13, 8), zero, q(2, 13), q(2, 13), NormalOffsetSweepBoundary, []int{1, 2},
			[][2]*big.Rat{{apex, zero}, {q(2, 1), zero}},
			[][]NormalOffsetSweepSide{{NormalOffsetSweepUpper}, {NormalOffsetSweepUpper}}},
		{"second physical side with opposite raw bounds", q(13, 8), q(-1, 1), q(1, 64), zero, NormalOffsetSweepBoundary,
			[]int{1, 2}, [][2]*big.Rat{{q(4, 1), zero}, {q(2, 1), q(-1, 1)}},
			[][]NormalOffsetSweepSide{{NormalOffsetSweepUMax}, {NormalOffsetSweepVMin}}},
		{"outside the wall by 2^-60", q(13, 8), zero, new(big.Rat).Add(q(2, 13), q(1, 1<<60)), nil,
			NormalOffsetSweepExterior, nil, nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			world := obliqueSweepTestTransform(test.x, test.y, test.z)
			got, err := chain.LocateRational(ctx, world, NormalOffsetSweepQueryOptions{MaxDepth: 1, MaxCells: 2})
			if err != nil || got.Location != test.location || len(got.Owners) != len(test.owners) || got.VisitedCells != 2 {
				t.Fatal("the independently authored nonruled wall must be decided from both original edge restrictions", got, err)
			}
			for i, owner := range got.Owners {
				var faces []NormalOffsetSweepSide
				if test.faces != nil {
					faces = test.faces[i]
				}
				if owner.Owner != test.owners[i] || !slices.Equal(owner.Faces, faces) {
					t.Fatal("wall-interior or perimeter proof lost original identities", owner, faces)
				}
				raw := test.stations[i]
				checkJoinedSweepPointStation(t, sweeps[owner.Owner], world, owner, [3]*big.Rat{raw[0], raw[1], test.d}, true)
			}
			t.Logf("classification %d, %d original owners, %d operations", got.Location, len(got.Owners), got.ExactOperations)
		})
	}

	// Both original charts have a nonaffine quartic across this
	// wall, so neither admits a whole-chart perpendicular ruling.
	// The finite foot on the wall remains the independently authored
	// quadratic physical profile, with two distinct raw clocks.
	world := obliqueSweepTestTransform(q(5, 8), q(1, 10), q(1, 30))
	got, err := pair.LocateRational(ctx, world, NormalOffsetSweepQueryOptions{MaxDepth: 1, MaxCells: 2})
	if err != nil || got.Location != NormalOffsetSweepInterior || len(got.Owners) != 2 || got.VisitedCells != 2 {
		t.Fatal("exact oblique wall point should have two original algebraic owners", got, err)
	}
	for i, owner := range got.Owners {
		if owner.AlgebraicStation == nil || owner.AlgebraicStation.ProfileAxis != 1-i || len(owner.Faces) != 0 {
			t.Fatal("expected both original algebraic source stations and no outer face", owner)
		}
		original := []*big.Rat{q(3, 1), q(-1, 1)}[i]
		if owner.Parameters[i].Lower.Cmp(original) != 0 || owner.Parameters[i].Upper.Cmp(original) != 0 {
			t.Fatal("algebraic root escaped the original wall coordinate", owner)
		}
		cert := owner.AlgebraicStation
		root, err := ImportExactRoot(ctx, cert.ProfileRoot, ExactRootCodecOptions{})
		if err != nil || root.Interval().Lower.Cmp(root.Interval().Upper) >= 0 ||
			cert.SignedDisplacement.Lower.Cmp(q(3, 100)) <= 0 || cert.SignedDisplacement.Upper.Cmp(q(1, 30)) >= 0 {
			t.Fatal("nontrivial owner profile and displacement did not survive exact reopening", cert, err)
		}
		// For y=2s-1 and the reversed projective y=(1-5s/2)/(1+s/2),
		// clearing the independent physical foot polynomial gives these
		// original normalized cubics.
		coefficients := [2][]*big.Rat{
			{q(-33775, 1), q(61466, 1), q(-180, 1), q(120, 1)},
			{q(221048, 1), q(-406020, 1), q(-568566, 1), q(-158387, 1)},
		}[i]
		exact, err := root.SignPolynomial(ctx, coefficients, ExactCurvePlaneEventOptions{})
		if err != nil || exact.Sign != 0 {
			t.Fatal("raw root must match the independently composed wall foot polynomial", i, exact, err)
		}
	}
	for _, budget := range []NormalOffsetSweepQueryOptions{
		{MaxCells: 1}, {Patch: SurfacePatchOptions{MaxOperations: 500}},
	} {
		partial, err := pair.LocateRational(ctx, world, budget)
		if !errors.Is(err, ErrConvergence) || partial.Location != NormalOffsetSweepUnresolved || len(partial.Owners) != 0 {
			t.Fatal("unresolved root ledger published a partial joined result", partial, err)
		}
	}
	t.Logf("two nonruled original oblique wall roots re-opened at their exact owners in %d operations", got.ExactOperations)
}
