package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedFourChartNormalOffsetSweepChainRetainsOriginals(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	sweeps := [4]*VerifiedNormalOffsetSweep{
		sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}}),
		sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {6, 6}}),
		sweepChainPlanarChart(t, [2][2]int64{{6, 6}, {14, 14}}),
		sweepChainPlanarChart(t, [2][2]int64{{14, 14}, {16, 16}}),
	}
	join := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	pairs := fourSweepOriginalPairs(t, sweeps, [3]VerifiedNormalOffsetSweepJoin{join, join, join})
	separators := [3]VerifiedNormalOffsetSweepChainSeparator{
		{Covector: [3]*big.Rat{q(1, 1), nil, q(1, 1)}, Coordinate: q(1, 2)},
		{Covector: [3]*big.Rat{q(2, 1), q(-1, 1)}, Coordinate: q(3, 1)},
		{Covector: [3]*big.Rat{q(1, 1), nil, q(-2, 1)}, Coordinate: q(5, 2)},
	}
	chain, proof, err := NewVerifiedFourChartNormalOffsetSweepChain(ctx, pairs, separators, NormalOffsetCoverageOptions{})
	if err != nil || chain == nil || !proof.Complete || chain.Sweeps() != sweeps {
		t.Fatal("original four-chart topology did not verify", chain, proof, err)
	}
	for i, owners := range [3][2]int{{0, 2}, {0, 3}, {1, 3}} {
		report := proof.Nonadjacent[i]
		gap := [3]*big.Rat{q(13, 10), q(6, 1), q(8, 5)}[i]
		want := [3][4]*big.Rat{
			{q(-11, 10), q(1, 10), q(7, 5), q(18, 5)},
			{q(-3, 1), q(0, 1), q(6, 1), q(8, 1)},
			{q(-1, 5), q(17, 10), q(33, 10), q(21, 5)},
		}[i]
		if report.Owners != owners || report.Gap.Cmp(gap) != 0 ||
			report.Ranges[0].Lower.Cmp(want[0]) != 0 || report.Ranges[0].Upper.Cmp(want[1]) != 0 ||
			report.Ranges[1].Lower.Cmp(want[2]) != 0 || report.Ranges[1].Upper.Cmp(want[3]) != 0 {
			t.Fatal("a nonadjacent owner pair lost its independent signed-layer projection", i, report)
		}
		t.Logf("nonadjacent original owners %v, projection %v, gap %s", owners, report.Ranges, report.Gap)
	}
	for i, got := range chain.Joins() {
		expected := [3]*big.Rat{q(0, 1), q(3, 2), q(7, 2)}[i]
		if got.Owners != [2]int{i, i + 1} || got.Original.Edges != join.Edges || got.Coordinate.Cmp(expected) != 0 {
			t.Fatal("original full wall join was replaced", i, got)
		}
	}
	checkFourSweepBoundary(t, chain)
	t.Logf("verified all original pairs in %d operations", proof.ExactOperations)

	for i := range separators {
		bad := separators
		bad[i].Coordinate = q(-99, 1)
		if got, p, err := NewVerifiedFourChartNormalOffsetSweepChain(ctx, pairs, bad, NormalOffsetCoverageOptions{}); !errors.Is(err, ErrDomain) || got != nil || p.Complete || p.Nonadjacent[0].Gap != nil {
			t.Fatal("each independent separator is required to publish a chain", i, got, p, err)
		}
	}
	wrongMiddle := pairs
	sameGeometry := sweepChainPlanarChart(t, [2][2]int64{{6, 6}, {14, 14}})
	wrongMiddle[2] = proveSweepChainPair(t, sameGeometry, sweeps[3], join)
	if got, p, err := NewVerifiedFourChartNormalOffsetSweepChain(ctx, wrongMiddle, separators, NormalOffsetCoverageOptions{}); !errors.Is(err, ErrDomain) || got != nil || p.Complete {
		t.Fatal("both adjacent pairs must refer to the same original interior chart", got, p, err)
	}
	short := NormalOffsetCoverageOptions{Patch: SurfacePatchOptions{MaxOperations: proof.ExactOperations - 1}}
	if got, p, err := NewVerifiedFourChartNormalOffsetSweepChain(ctx, pairs, separators, short); !errors.Is(err, ErrConvergence) || got != nil || p.Complete || p.Nonadjacent[0].Gap != nil {
		t.Fatal("partial nonadjacent proof cannot publish the chain", got, p, err)
	}

	// The four independent widths are 1, 3/2, 2 and 1/2. The
	// material is a 5 by 1 by 1/5 rectangular box in world space.
	momentsOptions := fourSweepMomentOptions(q(1, 1_000_000))
	momentsOptions.MaxCells = 4
	moments, err := chain.MassMomentsRational(ctx, momentsOptions)
	if err != nil || !moments.Complete || moments.VisitedCells != 4 ||
		moments.Volume.Lower.Cmp(q(1, 1)) != 0 || moments.Volume.Upper.Cmp(q(1, 1)) != 0 ||
		moments.Centroid[0].Lower.Cmp(q(3, 2)) != 0 || moments.Centroid[0].Upper.Cmp(q(3, 2)) != 0 {
		t.Fatal("exact original extensive moments missed the box volume or centroid", moments, err)
	}
	momentsOptions.MaxCells = 3
	if short, err := chain.MassMomentsRational(ctx, momentsOptions); !errors.Is(err, ErrConvergence) ||
		short.Complete || short.Volume.Lower != nil {
		t.Fatal("whole four-chart volume requires all original roots", short, err)
	}
	areaOptions := NormalOffsetSweepAreaOptions{AbsoluteError: q(0, 1), MaxCells: 18}
	for _, measure := range []func() (NormalOffsetSweepJoinedBoundaryArea, error){
		func() (NormalOffsetSweepJoinedBoundaryArea, error) {
			return chain.BoundaryAreaRational(ctx, areaOptions)
		},
		func() (NormalOffsetSweepJoinedBoundaryArea, error) {
			return chain.EncloseBoundaryAreaRational(ctx, NormalOffsetSweepAreaEnclosureOptions{NormalOffsetSweepAreaOptions: areaOptions})
		},
	} {
		area, err := measure()
		if err != nil || !area.Complete || area.VisitedCells != 18 ||
			area.Bounds.Lower.Cmp(q(62, 5)) != 0 || area.Bounds.Upper.Cmp(q(62, 5)) != 0 {
			t.Fatal("both original boundary measurements must integrate exactly 18 retained faces", area, err)
		}
		checkFourSweepAreaOwners(t, area, chain.Boundary())
	}
	// At x=7/2 the third joining wall is an interior zero-distance
	// candidate for an original chart. The retained outer minimum is
	// the cap, and a point inside chart 3 keeps the fourth owner.
	for _, x := range []*big.Rat{q(7, 2), q(15, 4)} {
		point := [3]*big.Rat{x, q(1, 2), q(1, 50)}
		o := NormalOffsetSweepDistanceOptions{SquaredTolerance: q(0, 1), MaxCells: 18}
		got, err := chain.BoundPointBoundaryDistanceRational(ctx, point, o)
		if err != nil || !got.Complete || got.VisitedCells != 18 || got.Witness == nil || got.Witness.Side != NormalOffsetSweepUpper ||
			got.Squared.Lower.Cmp(q(4, 625)) != 0 || got.Squared.Upper.Cmp(q(4, 625)) != 0 ||
			(x.Cmp(q(15, 4)) == 0 && got.Witness.Owner != 3) {
			t.Fatal("third join or fourth original owner corrupted the retained distance", got, err)
		}
		checkJoinedSweepDistanceWitness(t, sweeps[:], chain.Boundary(), point, got)
		o.MaxCells--
		short, err := chain.BoundPointBoundaryDistanceRational(ctx, point, o)
		if !errors.Is(err, ErrConvergence) || short.Complete || short.Squared.Lower != nil || short.Witness == nil {
			t.Fatal("partial face search published an unproved coarse global lower bound", short, err)
		}
	}
}

func TestVerifiedFourChartNormalOffsetSweepChainNonparallelDirectedArea(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	projection := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
	domain := ParameterRectangle{0, 1, 0, 1}
	sweeps := [4]*VerifiedNormalOffsetSweep{
		sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}}),
		authoredSweepWedgeChart(t, [2][2][2]int64{{{0, 0}, {0, 4}}, {{4, 0}, {5, 4}}}, domain, projection),
		authoredSweepWedgeChart(t, [2][2][2]int64{{{4, 0}, {5, 4}}, {{8, 0}, {7, 4}}}, domain, projection),
		authoredSweepWedgeChart(t, [2][2][2]int64{{{8, 0}, {7, 4}}, {{12, 0}, {11, 4}}}, domain, projection),
	}
	joins := [3]VerifiedNormalOffsetSweepJoin{}
	for i := range joins {
		joins[i] = VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	}
	joins[1].Covector = &[3]*big.Rat{q(1, 1), q(-1, 4)}
	joins[2].Covector = &[3]*big.Rat{q(1, 1), q(1, 4)}
	pairs := fourSweepOriginalPairs(t, sweeps, joins)
	separators := [3]VerifiedNormalOffsetSweepChainSeparator{}
	for i, level := range [3]*big.Rat{q(1, 2), q(1, 1), q(3, 2)} {
		separators[i] = VerifiedNormalOffsetSweepChainSeparator{Covector: [3]*big.Rat{q(1, 1)}, Coordinate: level}
	}
	chain, proof, err := NewVerifiedFourChartNormalOffsetSweepChain(ctx, pairs, separators, NormalOffsetCoverageOptions{})
	if err != nil || chain == nil || !proof.Complete {
		t.Fatal("all three distinct original joining planes must retain their independent end proofs", chain, proof, err)
	}
	checkFourSweepBoundary(t, chain)
	for i, join := range chain.Joins() {
		x := sweepJoinPlaneCovector(joins[i])
		y := sweepJoinPlaneCovector(join.Original)
		for axis := range x {
			if x[axis].Cmp(y[axis]) != 0 {
				t.Fatal("original nonparallel join was recentered", i, join)
			}
		}
	}

	// The original planar footprint is the quadrilateral
	// x=-1 through x=3-y/4, with 0<=y<=1. The cap areas are
	// 31/8 each and the outer perimeter is (35+sqrt(17))/4.
	got, err := chain.MassMomentsRational(ctx, fourSweepMomentOptions(q(1, 50)))
	volume := q(31, 40)
	if err != nil || !got.Complete || got.Volume.Lower.Cmp(volume) > 0 || got.Volume.Upper.Cmp(volume) < 0 {
		t.Fatal("bounded original volume missed the independent exact trapezoidal prism", got, err)
	}
	o := NormalOffsetSweepAreaEnclosureOptions{NormalOffsetSweepAreaOptions: NormalOffsetSweepAreaOptions{AbsoluteError: q(1, 10)}}
	area, err := chain.EncloseBoundaryAreaRational(ctx, o)
	if err != nil || !area.Complete || new(big.Rat).Sub(area.Bounds.Upper, area.Bounds.Lower).Cmp(o.AbsoluteError) > 0 {
		t.Fatal("four-owner directed area should refine the actual nonparallel outer geometry", area, err)
	}
	checkFourSweepAreaOwners(t, area, chain.Boundary())
	for _, face := range area.Faces {
		expected := q(1, 5)
		switch face.Side {
		case NormalOffsetSweepLower, NormalOffsetSweepUpper:
			expected = [4]*big.Rat{q(1, 1), q(9, 8), q(3, 4), q(1, 1)}[face.Owner]
		case NormalOffsetSweepVMax:
			expected = [4]*big.Rat{q(1, 5), q(1, 4), q(1, 10), q(1, 5)}[face.Owner]
		case NormalOffsetSweepUMax:
			// The last original wall has area sqrt(17)/20.
			requireFourSweepIrrationalArea(t, face.Bounds, q(0, 1))
			continue
		}
		if face.Bounds.Lower.Cmp(expected) > 0 || face.Bounds.Upper.Cmp(expected) < 0 {
			t.Fatal("original owner face missed its independent planar integral", face, expected)
		}
	}
	requireFourSweepIrrationalArea(t, area.Bounds, q(19, 2))
	t.Logf("nonparallel control volume %s in [%s,%s]; retained area [%s,%s], 18 original face owners, %d area cells and %d operations",
		volume, got.Volume.Lower.FloatString(12), got.Volume.Upper.FloatString(12), area.Bounds.Lower.FloatString(12),
		area.Bounds.Upper.FloatString(12), area.VisitedCells, area.ExactOperations)
	o.MaxCells = 18
	coarse, err := chain.EncloseBoundaryAreaRational(ctx, o)
	if !errors.Is(err, ErrConvergence) || coarse.Complete || coarse.VisitedCells != 18 || len(coarse.Faces) != 18 ||
		coarse.Bounds.Lower == nil || coarse.Bounds.Lower.Cmp(area.Bounds.Lower) > 0 || coarse.Bounds.Upper.Cmp(area.Bounds.Upper) < 0 {
		t.Fatal("full retained face coverage should preserve a coarse four-owner bound", coarse, err)
	}
	checkFourSweepAreaOwners(t, coarse, chain.Boundary())
	o.MaxCells--
	prefix, err := chain.EncloseBoundaryAreaRational(ctx, o)
	if !errors.Is(err, ErrConvergence) || prefix.Complete || len(prefix.Faces) != 17 ||
		prefix.Bounds.Lower != nil || prefix.Bounds.Upper != nil {
		t.Fatal("17 retained root cells cannot publish a full four-owner area", prefix, err)
	}
	checkFourSweepAreaOwners(t, prefix, chain.Boundary()[:17])
}

func requireFourSweepIrrationalArea(t *testing.T, bounds ExactParameterInterval, base *big.Rat) {
	t.Helper()
	q := big.NewRat
	for i, bound := range [2]*big.Rat{bounds.Lower, bounds.Upper} {
		edgeTerm := new(big.Rat).Mul(q(20, 1), new(big.Rat).Sub(bound, base))
		squared := new(big.Rat).Mul(edgeTerm, edgeTerm)
		if (i == 0 && edgeTerm.Sign() >= 0 && squared.Cmp(q(17, 1)) > 0) ||
			(i == 1 && (edgeTerm.Sign() < 0 || squared.Cmp(q(17, 1)) < 0)) {
			t.Fatal("nonparallel directed area missed the independently known radical", bounds, base)
		}
	}
}

func fourSweepOriginalPairs(t *testing.T, sweeps [4]*VerifiedNormalOffsetSweep, joins [3]VerifiedNormalOffsetSweepJoin) (r [3]*VerifiedNormalOffsetSweepPair) {
	t.Helper()
	for i, join := range joins {
		r[i] = proveSweepChainPair(t, sweeps[i], sweeps[i+1], join)
	}
	return r
}

func fourSweepMomentOptions(tolerance *big.Rat) NormalOffsetSweepMomentOptions {
	wide := big.NewRat(1000, 1)
	return NormalOffsetSweepMomentOptions{VolumeTolerance: tolerance, CentroidTolerance: wide, SecondMomentTolerance: wide, InertiaTolerance: wide}
}

func checkFourSweepBoundary(t *testing.T, chain *VerifiedFourChartNormalOffsetSweepChain) {
	t.Helper()
	boundary := chain.Boundary()
	if len(boundary) != 18 {
		t.Fatal("four original closed charts have six omitted full-wall face copies", len(boundary))
	}
	index := 0
	for i, chart := range chain.Sweeps() {
		for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
			if (i > 0 && side == NormalOffsetSweepUMin) || (i < 3 && side == NormalOffsetSweepUMax) {
				continue
			}
			original, _ := chart.Boundary(side)
			face := boundary[index]
			if face.Owner != i || face.Side != side || face.Face != original {
				t.Fatal("retained face lost its original role, owner, or analytic support", index, face)
			}
			index++
		}
	}
}

func checkFourSweepAreaOwners(t *testing.T, result NormalOffsetSweepJoinedBoundaryArea, boundary []VerifiedNormalOffsetSweepBoundaryFace) {
	t.Helper()
	if len(result.Faces) != len(boundary) {
		t.Fatal("area does not cover the complete requested original owner list", result)
	}
	for i, face := range result.Faces {
		if face.Owner != boundary[i].Owner || face.Side != boundary[i].Side {
			t.Fatal("directed area changed original ownership", i, face)
		}
	}
}
