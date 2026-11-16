package geom

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"
)

func TestVerifiedJoinedNormalOffsetSweepLocatesAffineJoins(t *testing.T) {
	ctx := context.Background()
	sweeps := [3]*VerifiedNormalOffsetSweep{
		sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}}),
		sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {4, 4}}),
		sweepChainPlanarChart(t, [2][2]int64{{4, 4}, {8, 8}}),
	}
	join := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	second := join
	second.Covector = &[3]*big.Rat{big.NewRat(-7, 3)}
	second.AInteriorSign = 1
	pair, chain := joinedSweepMomentFixture(t, sweeps, join, second)
	q := big.NewRat
	pt := func(x, y, d *big.Rat) [3]*big.Rat { return [3]*big.Rat{x, y, d} }
	zero, one, y, d := q(0, 1), q(1, 1), q(3, 7), q(-1, 17)
	tiny := q(1, 1<<60)
	for _, tc := range []struct {
		name   string
		point  [3]*big.Rat
		want   NormalOffsetSweepLocation
		owners []int
		faces  [][]NormalOffsetSweepSide
	}{
		{"first joined interior", pt(zero, y, d), NormalOffsetSweepInterior, []int{0, 1}, nil},
		{"second joined interior", pt(one, y, d), NormalOffsetSweepInterior, []int{1, 2}, nil},
		{"near one side", pt(new(big.Rat).Neg(tiny), y, d), NormalOffsetSweepInterior, []int{0}, nil},
		{"near other side", pt(new(big.Rat).Add(one, tiny), y, d), NormalOffsetSweepInterior, []int{2}, nil},
		{"first cap perimeter", pt(zero, y, q(1, 10)), NormalOffsetSweepBoundary, []int{0, 1},
			[][]NormalOffsetSweepSide{{NormalOffsetSweepUpper}, {NormalOffsetSweepUpper}}},
		{"second side perimeter", pt(one, zero, d), NormalOffsetSweepBoundary, []int{1, 2},
			[][]NormalOffsetSweepSide{{NormalOffsetSweepVMin}, {NormalOffsetSweepVMin}}},
		{"second cap corner", pt(one, one, q(-1, 10)), NormalOffsetSweepBoundary, []int{1, 2},
			[][]NormalOffsetSweepSide{{NormalOffsetSweepVMax, NormalOffsetSweepLower}, {NormalOffsetSweepVMax, NormalOffsetSweepLower}}},
		{"single outer corner", pt(q(-1, 1), zero, q(1, 10)), NormalOffsetSweepBoundary, []int{0},
			[][]NormalOffsetSweepSide{{NormalOffsetSweepUMin, NormalOffsetSweepVMin, NormalOffsetSweepUpper}}},
		{"on separating plane beyond wall", pt(one, new(big.Rat).Add(one, tiny), d), NormalOffsetSweepExterior, nil, nil},
		{"outside signed displacement", pt(zero, y, new(big.Rat).Sub(q(-1, 10), tiny)), NormalOffsetSweepExterior, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := chain.LocateRational(ctx, tc.point, NormalOffsetSweepQueryOptions{})
			if err != nil || result.Location != tc.want || len(result.Owners) != len(tc.owners) ||
				result.VisitedCells < 1 || result.VisitedCells > 2 {
				t.Fatal("wrong joined location or selected owner count", result, err)
			}
			for i, owner := range result.Owners {
				if owner.Owner != tc.owners[i] {
					t.Fatal("lost original chart order", owner, tc.owners)
				}
				var faces []NormalOffsetSweepSide
				if tc.faces != nil {
					faces = tc.faces[i]
				}
				if !slices.Equal(owner.Faces, faces) {
					t.Fatal("wrong retained original faces", owner.Faces, faces)
				}
				wantU := new(big.Rat).Sub(tc.point[0], q(int64(owner.Owner-1), 1))
				checkJoinedSweepPointStation(t, sweeps[owner.Owner], tc.point, owner,
					[3]*big.Rat{wantU, tc.point[1], tc.point[2]}, true)
			}
			t.Logf("classification %d, %d original witnesses, %d cells, %d exact operations",
				result.Location, len(result.Owners), result.VisitedCells, result.ExactOperations)
		})
	}

	point := pt(zero, y, d)
	for owner, sweep := range sweeps[:2] {
		r, err := sweep.LocateRational(ctx, point, NormalOffsetSweepQueryOptions{})
		if err != nil || r.Location != NormalOffsetSweepBoundary {
			t.Fatal("the joined interior point must be on both actual original walls", owner, r, err)
		}
	}
	hit, err := pair.LocateRational(ctx, point, NormalOffsetSweepQueryOptions{})
	if err != nil || hit.Location != NormalOffsetSweepInterior || len(hit.Owners) != 2 ||
		hit.VisitedCells != 2 || hit.Owners[0].Owner != 0 || hit.Owners[1].Owner != 1 {
		t.Fatal("pair should agree with the chain on the shared first wall", hit, err)
	}
	for _, limit := range []NormalOffsetSweepQueryOptions{
		{MaxCells: 1},
		{Patch: SurfacePatchOptions{MaxOperations: hit.ExactOperations - 1}},
	} {
		partial, err := pair.LocateRational(ctx, point, limit)
		if !errors.Is(err, ErrConvergence) || partial.Location != NormalOffsetSweepUnresolved || len(partial.Owners) != 0 {
			t.Fatal("incomplete chart work published a prefix of the joined membership", partial, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if r, err := pair.LocateRational(cancelled, point, NormalOffsetSweepQueryOptions{}); !errors.Is(err, context.Canceled) ||
		r.Location != NormalOffsetSweepUnresolved || len(r.Owners) != 0 {
		t.Fatal("cancelled query published a result", r, err)
	}
}

func TestVerifiedJoinedNormalOffsetSweepLocatesPlacedAffineJoin(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	zero := q(0, 1)
	var sweeps [2]*VerifiedNormalOffsetSweep
	for owner := range sweeps {
		recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, Domain: ParameterRectangle{2, 3, -1, 1}}
		if owner == 1 {
			recipe.Domain = ParameterRectangle{2, 4, -1, 0}
		}
		for i := range 2 {
			for j := range 2 {
				x, y := i, 2*j-1
				if owner == 1 {
					x, y = j, 1-2*i
				}
				world := obliqueSweepTestTransform(q(int64(5+8*(owner-1+x)), 8), q(int64(y), 1), zero)
				recipe.Controls = append(recipe.Controls, [4]string{
					world[0].RatString(), world[1].RatString(), world[2].RatString(), "1",
				})
			}
		}
		source, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		projection := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if owner == 1 {
			projection = NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, q(-1, 11), q(2, 13), projection, NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("placed affine source did not verify", owner, proof, err)
		}
		sweeps[owner] = sweep
	}
	join := obliqueSweepPairJoin()
	join.BStationRatio = nil
	pair := proveSweepChainPair(t, sweeps[0], sweeps[1], join)
	for _, test := range []struct {
		displacement *big.Rat
		location     NormalOffsetSweepLocation
		faces        []NormalOffsetSweepSide
	}{
		{q(-1, 17), NormalOffsetSweepInterior, nil},
		{q(2, 13), NormalOffsetSweepBoundary, []NormalOffsetSweepSide{NormalOffsetSweepUpper}},
	} {
		world := obliqueSweepTestTransform(q(5, 8), q(1, 3), test.displacement)
		result, err := pair.LocateRational(ctx, world, NormalOffsetSweepQueryOptions{})
		if err != nil || result.Location != test.location || len(result.Owners) != 2 {
			t.Fatal("cannot locate the independently placed rational point on the common wall", result, err)
		}
		expected := [2][3]*big.Rat{{q(3, 1), q(1, 3), test.displacement}, {q(8, 3), q(-1, 1), test.displacement}}
		for i, owner := range result.Owners {
			if owner.Owner != i || !slices.Equal(owner.Faces, test.faces) {
				t.Fatal("rotated join lost original owner incidence", owner, test.faces)
			}
			checkJoinedSweepPointStation(t, sweeps[i], world, owner, expected[i], true)
		}
		t.Logf("oblique original A=(3,1/3), reversed B=(8/3,-1), displacement %s, classification %d, %d operations",
			test.displacement, result.Location, result.ExactOperations)
	}
}

func checkJoinedSweepPointStation(t *testing.T, sweep *VerifiedNormalOffsetSweep, target [3]*big.Rat,
	owner NormalOffsetSweepJoinedPointOwner, exactStation [3]*big.Rat, singleton bool) {
	t.Helper()
	for i, want := range exactStation {
		bounds := owner.Parameters[i]
		if bounds.Lower.Cmp(want) > 0 || bounds.Upper.Cmp(want) < 0 ||
			(singleton && bounds.Lower.Cmp(bounds.Upper) != 0) {
			t.Fatal("wrong original inverse bounds", owner, i, want)
		}
	}
	physical, err := sweep.EvaluateRational(context.Background(), exactStation[0], exactStation[1], exactStation[2],
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, coordinate := range target {
		if physical.Lower[i].Cmp(coordinate) > 0 || physical.Upper[i].Cmp(coordinate) < 0 {
			t.Fatal("original owner station did not reproduce the exact model point", owner, physical, target)
		}
	}
}
