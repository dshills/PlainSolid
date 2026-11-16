package geom

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"
)

func TestVerifiedJoinedNormalOffsetSweepLocatesPlacedCurvedJoins(t *testing.T) {
	ctx := context.Background()
	sweeps, joins := placedSweepDistanceCylinder(t)
	pair, chain := joinedSweepMomentFixture(t, sweeps, joins[0], joins[1])
	q := big.NewRat
	for _, test := range []struct {
		name             string
		x, y, d          *big.Rat
		location         NormalOffsetSweepLocation
		ownerIDs         []int
		originalStations [][2]*big.Rat
		faces            [][]NormalOffsetSweepSide
	}{
		{"first wall interior", q(5, 8), q(0, 1), q(-1, 17), NormalOffsetSweepInterior,
			[]int{0, 1}, [][2]*big.Rat{{q(3, 1), q(0, 1)}, {q(14, 5), q(-1, 1)}}, nil},
		{"second wall interior", q(13, 8), q(0, 1), q(1, 17), NormalOffsetSweepInterior,
			[]int{1, 2}, [][2]*big.Rat{{q(14, 5), q(0, 1)}, {q(2, 1), q(0, 1)}}, nil},
		{"cap perimeter", q(13, 8), q(0, 1), q(2, 13), NormalOffsetSweepBoundary,
			[]int{1, 2}, [][2]*big.Rat{{q(14, 5), q(0, 1)}, {q(2, 1), q(0, 1)}},
			[][]NormalOffsetSweepSide{{NormalOffsetSweepUpper}, {NormalOffsetSweepUpper}}},
		{"side perimeter in opposing raw coordinates", q(5, 8), q(1, 1), q(1, 64), NormalOffsetSweepBoundary,
			[]int{0, 1}, [][2]*big.Rat{{q(3, 1), q(1, 1)}, {q(2, 1), q(-1, 1)}},
			[][]NormalOffsetSweepSide{{NormalOffsetSweepVMax}, {NormalOffsetSweepUMin}}},
		{"inside the middle", q(9, 8), q(0, 1), q(1, 17), NormalOffsetSweepInterior,
			[]int{1}, [][2]*big.Rat{{q(14, 5), q(-1, 2)}}, nil},
		{"just outside the cap in the joining plane", q(13, 8), q(0, 1),
			new(big.Rat).Add(q(2, 13), q(1, 1<<60)), NormalOffsetSweepExterior, nil, nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			// At y=0 the original unit normal is the rotated local Z.
			// At y=1 here d equals the original quadratic source height,
			// and the signed displacement is zero.
			point := obliqueSweepTestTransform(test.x, test.y, test.d)
			result, err := chain.LocateRational(ctx, point, NormalOffsetSweepQueryOptions{})
			if err != nil || result.Location != test.location || len(result.Owners) != len(test.ownerIDs) ||
				result.VisitedCells < 1 || result.VisitedCells > 2 {
				t.Fatal("failed exact membership of the original rotated charts", result, err)
			}
			for i, owner := range result.Owners {
				var faces []NormalOffsetSweepSide
				if test.faces != nil {
					faces = test.faces[i]
				}
				if owner.Owner != test.ownerIDs[i] || !slices.Equal(owner.Faces, faces) || owner.AlgebraicStation != nil {
					t.Fatal("lost original face ownership or the rational station", owner, faces)
				}
				raw := test.originalStations[i]
				d := test.d
				if test.y.Sign() != 0 {
					d = q(0, 1)
				}
				checkJoinedSweepPointStation(t, sweeps[owner.Owner], point, owner, [3]*big.Rat{raw[0], raw[1], d}, true)
			}
			t.Logf("classification %d, %d original charts, %d operations", result.Location, len(result.Owners), result.ExactOperations)
		})
	}

	// The formal original profile is z=y²/64, so the exact normal
	// foot satisfies (y-1/10)+(y/32)*(y²/64-1/30)=0. The two original
	// chart coordinates have independent affine and reversed rational laws.
	point := obliqueSweepTestTransform(q(5, 8), q(1, 10), q(1, 30))
	result, err := pair.LocateRational(ctx, point, NormalOffsetSweepQueryOptions{})
	if err != nil || result.Location != NormalOffsetSweepInterior || len(result.Owners) != 2 || result.VisitedCells != 2 {
		t.Fatal("missed the jointly owned nonrational source foot", result, err)
	}
	for i, owner := range result.Owners {
		if owner.Owner != i || len(owner.Faces) != 0 || owner.AlgebraicStation == nil {
			t.Fatal("missing an original algebraic wall witness", owner)
		}
		cert := owner.AlgebraicStation
		if cert.ProfileAxis != 1-i {
			t.Fatal("lost the original free parameter axis", cert)
		}
		fixed := owner.Parameters[i]
		wall := []*big.Rat{q(3, 1), q(-1, 1)}[i]
		if fixed.Lower.Cmp(wall) != 0 || fixed.Upper.Cmp(wall) != 0 {
			t.Fatal("point is not at the exact original wall station", owner)
		}
		root, err := ImportExactRoot(ctx, cert.ProfileRoot, ExactRootCodecOptions{})
		if err != nil {
			t.Fatal("original profile root cannot be reopened", owner, err)
		}
		if root.Interval().Lower.Cmp(root.Interval().Upper) >= 0 || cert.SignedDisplacement.Lower.Cmp(q(3, 100)) <= 0 ||
			cert.SignedDisplacement.Upper.Cmp(q(1, 30)) >= 0 {
			t.Fatal("expected a nontrivial isolated foot and positive signed displacement", cert)
		}
	}
	a, b := result.Owners[0].Parameters[1], result.Owners[1].Parameters[0]
	bToPhysical := func(u *big.Rat) *big.Rat {
		s := new(big.Rat).Quo(new(big.Rat).Sub(u, q(2, 1)), q(2, 1))
		projective := new(big.Rat).Quo(new(big.Rat).Mul(q(3, 2), s), new(big.Rat).Add(q(1, 1), new(big.Rat).Mul(q(1, 2), s)))
		return new(big.Rat).Sub(q(1, 1), new(big.Rat).Mul(q(2, 1), projective))
	}
	normalFoot := func(y *big.Rat) *big.Rat {
		z := new(big.Rat).Quo(new(big.Rat).Mul(y, y), q(64, 1))
		slope := new(big.Rat).Quo(y, q(32, 1))
		vertical := new(big.Rat).Mul(slope, new(big.Rat).Sub(z, q(1, 30)))
		return new(big.Rat).Add(new(big.Rat).Sub(y, q(1, 10)), vertical)
	}
	for _, interval := range [][2]*big.Rat{{a.Lower, a.Upper}, {bToPhysical(b.Upper), bToPhysical(b.Lower)}} {
		if normalFoot(interval[0]).Sign() > 0 || normalFoot(interval[1]).Sign() < 0 {
			t.Fatal("original raw enclosure lost the independently written physical normal-foot root", interval)
		}
	}
	t.Logf("two independent curved original roots in raw intervals [%s,%s] and [%s,%s], %d operations",
		a.Lower.FloatString(13), a.Upper.FloatString(13), b.Lower.FloatString(13), b.Upper.FloatString(13), result.ExactOperations)

	for _, limit := range []NormalOffsetSweepQueryOptions{
		{Patch: SurfacePatchOptions{MaxOperations: 500}},
		{MaxCells: 1},
	} {
		partial, err := pair.LocateRational(ctx, point, limit)
		if !errors.Is(err, ErrConvergence) || partial.Location != NormalOffsetSweepUnresolved || len(partial.Owners) != 0 {
			t.Fatal("short exact work published a prefix of the curved point classification", partial, err)
		}
	}
}

func TestVerifiedJoinedNormalOffsetSweepLocatesAlgebraicCapPerimeter(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	source := normalOffsetParabolicCylinder(t, false)
	var sweeps [2]*VerifiedNormalOffsetSweep
	for i, d := range [2]ParameterRectangle{
		{21.0 / 32, 11.0 / 16, 0, .5},
		{21.0 / 32, 11.0 / 16, .5, 1},
	} {
		patch, err := source.Trim(d)
		if err != nil {
			t.Fatal(err)
		}
		sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, patch, q(11, 20), q(3, 5),
			NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("original curved region is uncertified", i, proof, err)
		}
		sweeps[i] = sweep
	}
	pair := proveSweepChainPair(t, sweeps[0], sweeps[1], VerifiedNormalOffsetSweepJoin{
		Edges: [2]SurfaceEdge{SurfaceEdgeVMax, SurfaceEdgeVMin}, Axis: 1, AInteriorSign: -1,
	})
	// At the original profile x²=11/100, t=3/5 sends physical x
	// to zero and produces the rational cap height 61/100.
	point := [3]*big.Rat{q(0, 1), q(1, 2), q(61, 100)}
	got, err := pair.LocateRational(ctx, point, NormalOffsetSweepQueryOptions{})
	if err != nil || got.Location != NormalOffsetSweepBoundary || len(got.Owners) != 2 {
		t.Fatal("the joined cap boundary requires both exact algebraic owner stations", got, err)
	}
	for i, owner := range got.Owners {
		if owner.Owner != i || !slices.Equal(owner.Faces, []NormalOffsetSweepSide{NormalOffsetSweepUpper}) ||
			owner.AlgebraicStation == nil || owner.AlgebraicStation.ProfileAxis != 0 {
			t.Fatal("algebraic shared perimeter lost its retained cap incidences", owner)
		}
		for j, expected := range [2]*big.Rat{q(1, 2), q(3, 5)} {
			bounds := owner.Parameters[j+1]
			if bounds.Lower.Cmp(expected) != 0 || bounds.Upper.Cmp(expected) != 0 {
				t.Fatal("algebraic cap lost exact seam or displacement equality", owner)
			}
		}
		root, err := ImportExactRoot(ctx, owner.AlgebraicStation.ProfileRoot, ExactRootCodecOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// The physical profile x=5/16+s/16 gives this equation
		// for normalized s after clearing the denominators.
		exact, err := root.SignPolynomial(ctx, []*big.Rat{q(-79, 1), q(250, 1), q(25, 1)}, ExactCurvePlaneEventOptions{})
		if err != nil || exact.Sign != 0 {
			t.Fatal("reconstructed root differs from the independently authored cap profile", exact, err)
		}
	}
	t.Logf("retained both cap faces and excluded both joining wall copies at the exact algebraic perimeter, %d operations", got.ExactOperations)
}

func TestVerifiedJoinedNormalOffsetSweepLocatesRefusesIncompleteNonRuledOffWallProof(t *testing.T) {
	ctx := context.Background()
	sweeps := placedSweepChainFixture(t)
	first, second := sweepChainPlacedJoins()
	_, chain := joinedSweepMomentFixture(t, sweeps, first, second)
	// This known middle-chart point is away from both exact join
	// planes. Its original quartic height is 1/8192 at the center.
	height := new(big.Rat).Add(big.NewRat(1, 8192), big.NewRat(1, 17))
	point := obliqueSweepTestTransform(big.NewRat(9, 8), new(big.Rat), height)
	got, err := chain.LocateRational(ctx, point, NormalOffsetSweepQueryOptions{MaxDepth: 1, MaxCells: 2})
	if !errors.Is(err, ErrConvergence) || got.Location != NormalOffsetSweepUnresolved || len(got.Owners) != 0 ||
		got.VisitedCells != 2 {
		t.Fatal("unresolved non-ruled source did not retain its original inverse limit", got, err)
	}
}
