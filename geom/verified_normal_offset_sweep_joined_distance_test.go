package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedJoinedNormalOffsetSweepBoundaryDistanceOmitsInternalWalls(t *testing.T) {
	ctx := context.Background()
	a := sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}})
	b := sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {4, 4}})
	c := sweepChainPlanarChart(t, [2][2]int64{{4, 4}, {8, 8}})
	join := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	pair, chain := joinedSweepMomentFixture(t, [3]*VerifiedNormalOffsetSweep{a, b, c}, join, join)
	query := func(x, y, z *big.Rat) [3]*big.Rat { return [3]*big.Rat{x, y, z} }
	q := big.NewRat
	want := q(4, 625)
	o := NormalOffsetSweepDistanceOptions{SquaredTolerance: new(big.Rat)}
	for _, test := range []struct {
		name             string
		point            [3]*big.Rat
		target           [3]*big.Rat
		owner            int
		internalDistance *big.Rat
		sweeps           []*VerifiedNormalOffsetSweep
		faces            []VerifiedNormalOffsetSweepBoundaryFace
		bound            func(context.Context, [3]*big.Rat, NormalOffsetSweepDistanceOptions) (NormalOffsetSweepDistance, error)
	}{
		{"first removed wall", query(q(0, 1), q(1, 2), q(1, 50)), query(q(1, 1), q(1, 2), q(1, 10)), 0, q(0, 1),
			[]*VerifiedNormalOffsetSweep{a, b}, pair.Boundary(), pair.BoundPointBoundaryDistanceRational},
		{"near first removed wall", query(q(1, 50), q(1, 3), q(1, 50)), query(q(1, 50), q(1, 3), q(1, 10)), 1, q(1, 2500),
			[]*VerifiedNormalOffsetSweep{a, b}, pair.Boundary(), pair.BoundPointBoundaryDistanceRational},
		{"second removed wall", query(q(1, 1), q(1, 2), q(1, 50)), query(q(1, 1), q(1, 2), q(1, 10)), 1, q(0, 1),
			[]*VerifiedNormalOffsetSweep{a, b, c}, chain.Boundary(), chain.BoundPointBoundaryDistanceRational},
		{"near second removed wall", query(q(51, 50), q(1, 3), q(1, 50)), query(q(1, 50), q(1, 3), q(1, 10)), 2, q(1, 2500),
			[]*VerifiedNormalOffsetSweep{a, b, c}, chain.Boundary(), chain.BoundPointBoundaryDistanceRational},
	} {
		t.Run(test.name, func(t *testing.T) {
			original, err := test.sweeps[test.owner].BoundPointBoundaryDistanceRational(ctx, test.point, o)
			if err != nil || !original.Complete || original.Squared.Upper.Cmp(test.internalDistance) != 0 {
				t.Fatal("single-chart control should prefer the removed wall", original, err)
			}
			got, err := test.bound(ctx, test.point, o)
			if err != nil || !got.Complete || got.Squared.Lower.Cmp(want) != 0 || got.Squared.Upper.Cmp(want) != 0 ||
				got.Witness.Owner != test.owner || got.Witness.Side != NormalOffsetSweepUpper || got.VisitedCells != len(test.faces) {
				t.Fatal("internal wall or wrong outer-face owner affected the global proof", got, err)
			}
			for i, target := range test.target {
				if got.Witness.Parameters[i].Cmp(target) != 0 {
					t.Fatal("composed witness lost an original owner station", i, target, got.Witness)
				}
			}
			checkJoinedSweepDistanceWitness(t, test.sweeps, test.faces, test.point, got)
			limited := o
			limited.MaxCells = len(test.faces) - 1
			partial, err := test.bound(ctx, test.point, limited)
			if !errors.Is(err, ErrConvergence) || partial.Complete || partial.Squared.Lower != nil ||
				partial.Squared.Upper == nil || partial.Squared.Upper.Cmp(want) < 0 || partial.VisitedCells != limited.MaxCells {
				t.Fatal("incomplete retained face cover published a coarse lower bound", partial, err)
			}
			checkJoinedSweepDistanceWitness(t, test.sweeps, test.faces, test.point, partial)
			t.Logf("owner %d, face %d, squared distance %s, original station %v, %d faces", test.owner,
				got.Witness.Side, got.Squared.Upper, got.Witness.Parameters, got.VisitedCells)
		})
	}
	perimeter := query(q(0, 1), q(1, 2), q(1, 10))
	hit, err := pair.BoundPointBoundaryDistanceRational(ctx, perimeter, o)
	if err != nil || !hit.Complete || hit.Squared.Upper.Sign() != 0 || hit.Witness.Side != NormalOffsetSweepUpper {
		t.Fatal("actual retained perimeter should still allow an exact zero", hit, err)
	}
	checkJoinedSweepDistanceWitness(t, []*VerifiedNormalOffsetSweep{a, b}, pair.Boundary(), perimeter, hit)
}

func TestVerifiedJoinedNormalOffsetSweepBoundaryDistanceCurvedPlaced(t *testing.T) {
	ctx := context.Background()
	sweeps, joins := placedSweepDistanceCylinder(t)
	pair, chain := joinedSweepMomentFixture(t, sweeps, joins[0], joins[1])
	lower := big.NewRat(-1, 11)
	goal := big.NewRat(1, 100)
	query := obliqueSweepTestTransform(big.NewRat(17, 8), new(big.Rat),
		new(big.Rat).Sub(lower, big.NewRat(1, 10)))
	o := NormalOffsetSweepDistanceOptions{SquaredTolerance: big.NewRat(1, 2000)}
	// Before rotation the elliptic offset height is at least the
	// original lower t for every x,y. The lower layer midpoint of
	// C has y=0, so its distance attains the independent plane bound.
	got, err := chain.BoundPointBoundaryDistanceRational(ctx, query, o)
	if err != nil || !got.Complete || got.Squared.Lower.Cmp(goal) > 0 || got.Squared.Upper.Cmp(goal) != 0 ||
		new(big.Rat).Sub(got.Squared.Upper, got.Squared.Lower).Cmp(o.SquaredTolerance) > 0 ||
		got.Witness.Owner != 2 || got.Witness.Side != NormalOffsetSweepLower {
		t.Fatal("placed curved chain distance failed its independent lower-layer proof", got, err)
	}
	checkJoinedSweepDistanceWitness(t, sweeps[:], chain.Boundary(), query, got)
	t.Logf("placed curved chain squared interval [%s,%s], owner %d, %d visited cells and %d operations",
		got.Squared.Lower.FloatString(12), got.Squared.Upper, got.Witness.Owner, got.VisitedCells, got.ExactOperations)

	short := o
	short.MaxCells = len(chain.Boundary()) - 1
	withoutCover, err := chain.BoundPointBoundaryDistanceRational(ctx, query, short)
	if !errors.Is(err, ErrConvergence) || withoutCover.Complete || withoutCover.Squared.Lower != nil || withoutCover.Witness == nil {
		t.Fatal("curved root prefix published a spurious global lower bound", withoutCover, err)
	}
	checkJoinedSweepDistanceWitness(t, sweeps[:], chain.Boundary(), query, withoutCover)
	short.MaxCells++
	coarse, err := chain.BoundPointBoundaryDistanceRational(ctx, query, short)
	if !errors.Is(err, ErrConvergence) || coarse.Complete || coarse.Squared.Lower == nil ||
		coarse.Squared.Lower.Cmp(got.Squared.Lower) > 0 || coarse.Squared.Upper.Cmp(goal) != 0 {
		t.Fatal("complete retained root cover did not retain its valid coarse enclosure", coarse, err)
	}
	checkJoinedSweepDistanceWitness(t, sweeps[:], chain.Boundary(), query, coarse)
	short = o
	short.Patch.MaxOperations = coarse.ExactOperations + 10
	work, err := chain.BoundPointBoundaryDistanceRational(ctx, query, short)
	if !errors.Is(err, ErrConvergence) || work.Complete || work.Squared.Lower == nil ||
		work.Squared.Lower.Cmp(coarse.Squared.Lower) < 0 || work.Squared.Lower.Cmp(goal) > 0 || work.Squared.Upper.Cmp(goal) != 0 {
		t.Fatal("exact subdivision budget discarded the prior certified full cover", work, err)
	}
	checkJoinedSweepDistanceWitness(t, sweeps[:], chain.Boundary(), query, work)

	// A rationally rotated query lies in the interior of the removed
	// A/B wall. In local coordinates either offset layer has squared
	// distance at least its t², since the y² term of |S+tN|² has
	// coefficient 1 - t/(32*sqrt(1+y²/1024)) > 0. The retained
	// x walls and the shifted y walls are farther than either layer.
	seam := obliqueSweepTestTransform(big.NewRat(5, 8), new(big.Rat), new(big.Rat))
	removed, _ := sweeps[0].Boundary(NormalOffsetSweepUMax)
	internal, err := removed.EvaluateRational(ctx, new(big.Rat), new(big.Rat),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range seam {
		if internal.Lower[i].Cmp(x) != 0 || internal.Upper[i].Cmp(x) != 0 {
			t.Fatal("the rotated query must lie on the physically removed wall", internal, seam)
		}
	}
	hole, err := pair.BoundPointBoundaryDistanceRational(ctx, seam, o)
	seamGoal := big.NewRat(1, 121)
	if err != nil || !hole.Complete || hole.Squared.Lower.Sign() <= 0 ||
		hole.Squared.Lower.Cmp(seamGoal) > 0 || hole.Squared.Upper.Cmp(seamGoal) < 0 {
		t.Fatal("placed curved internal wall leaked into boundary candidates", hole, err)
	}
	checkJoinedSweepDistanceWitness(t, sweeps[:2], pair.Boundary(), seam, hole)
	t.Logf("placed curved removed wall squared interval [%s,%s], owner %d, %d visited cells and %d operations",
		hole.Squared.Lower.FloatString(12), hole.Squared.Upper.FloatString(12), hole.Witness.Owner, hole.VisitedCells, hole.ExactOperations)
}

func TestVerifiedJoinedNormalOffsetSweepBoundaryDistanceMixedOwners(t *testing.T) {
	ctx := context.Background()
	flat := sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}})
	var net [][4]*big.Rat
	for i := int64(0); i < 3; i++ {
		for y := int64(0); y < 2; y++ {
			net = append(net, [4]*big.Rat{big.NewRat(i, 2), big.NewRat(y, 1),
				big.NewRat([3]int64{0, 0, 1}[i], 64), big.NewRat(1, 1)})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(ctx, 2, 1, net, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	curved, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(-1, 10), big.NewRat(1, 10),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("expected independently verified curved mate", proof, err)
	}
	join := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	pair := proveSweepChainPair(t, flat, curved, join)
	// The flat lower layer is exactly 1/5 away. The curved owner
	// lies in x>=0 and is at least 1/2 away.
	query := [3]*big.Rat{big.NewRat(-1, 2), big.NewRat(1, 2), big.NewRat(-3, 10)}
	result, err := pair.BoundPointBoundaryDistanceRational(ctx, query,
		NormalOffsetSweepDistanceOptions{SquaredTolerance: new(big.Rat)})
	if err != nil || !result.Complete || result.Witness.Owner != 0 || result.Witness.Side != NormalOffsetSweepLower ||
		result.Squared.Lower.Cmp(big.NewRat(1, 25)) != 0 || result.Squared.Upper.Cmp(big.NewRat(1, 25)) != 0 {
		t.Fatal("shared exact search lost an affine minimum beside curved candidates", result, err)
	}
	checkJoinedSweepDistanceWitness(t, []*VerifiedNormalOffsetSweep{flat, curved}, pair.Boundary(), query, result)
}

func placedSweepDistanceCylinder(t *testing.T) (sweeps [3]*VerifiedNormalOffsetSweep, joins [2]VerifiedNormalOffsetSweepJoin) {
	t.Helper()
	ctx := context.Background()
	for owner := range sweeps {
		recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 2, Domain: ParameterRectangle{2, 3, -1, 1}}
		view := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if owner == 1 {
			recipe.DegreeU, recipe.DegreeV, recipe.Domain = 2, 1, ParameterRectangle{2, 4, -1, 0}
			view = NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		for i := 0; i <= recipe.DegreeU; i++ {
			for j := 0; j <= recipe.DegreeV; j++ {
				xi, yi := i, j
				weight := big.NewRat(1, 1)
				if owner == 1 {
					xi, yi = j, 2-i
					weight = [3]*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(9, 4)}[i]
				}
				x := big.NewRat(int64(5+8*(owner-1+xi)), 8)
				y, z := big.NewRat(int64(yi-1), 1), big.NewRat([3]int64{1, -1, 1}[yi], 64)
				world := obliqueSweepTestTransform(x, y, z)
				recipe.Controls = append(recipe.Controls, [4]string{
					world[0].Mul(world[0], weight).RatString(), world[1].Mul(world[1], weight).RatString(),
					world[2].Mul(world[2], weight).RatString(), weight.RatString(),
				})
			}
		}
		patch, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, patch, big.NewRat(-1, 11), big.NewRat(2, 13), view, NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("the original placed cylinder chart must verify", owner, proof, err)
		}
		sweeps[owner] = sweep
	}
	joins[0], joins[1] = sweepChainPlacedJoins()
	return sweeps, joins
}

func checkJoinedSweepDistanceWitness(t *testing.T, sweeps []*VerifiedNormalOffsetSweep, faces []VerifiedNormalOffsetSweepBoundaryFace,
	point [3]*big.Rat, got NormalOffsetSweepDistance) {
	t.Helper()
	if got.Witness == nil || got.Witness.Owner < 0 || got.Witness.Owner >= len(sweeps) {
		t.Fatal("missing original chart owner", got.Witness)
	}
	for _, face := range faces {
		if face.Owner == got.Witness.Owner && face.Side == got.Witness.Side {
			checkSweepBoundaryDistanceWitness(t, sweeps[face.Owner], point, got)
			return
		}
	}
	t.Fatal("upper witness was not on a retained analytic outer face", got.Witness)
}
