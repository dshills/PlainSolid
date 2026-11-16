package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSweepChainRetainsPlacedOriginals(t *testing.T) {
	ctx := context.Background()
	sweeps := placedSweepChainFixture(t)
	first, second := sweepChainPlacedJoins()
	ab := proveSweepChainPair(t, sweeps[0], sweeps[1], first)
	bc := proveSweepChainPair(t, sweeps[1], sweeps[2], second)
	chain, proof, err := NewVerifiedNormalOffsetSweepChain(ctx, ab, bc, SurfacePatchOptions{})
	if err != nil || chain == nil || !proof.Complete || proof.PlaneGap.Cmp(big.NewRat(65, 7)) != 0 || chain.Sweeps() != sweeps {
		t.Fatal("proved three-chart volume lost the exact end-chart gap or original owners", chain, proof, err)
	}
	t.Logf("two rotated swept walls with common scaled gap %s; pair constants %s and %s; chain exact work %d",
		proof.PlaneGap, ab.coordinate, bc.coordinate, proof.ExactOperations)
	joins := chain.Joins()
	if joins[0].Owners != [2]int{0, 1} || joins[1].Owners != [2]int{1, 2} ||
		joins[0].Coordinate.Cmp(big.NewRat(477, 112)) != 0 || joins[1].Coordinate.Cmp(big.NewRat(-4551, 224)) != 0 {
		t.Fatal("lost original exact scaling or chain owners", joins)
	}
	for i, original := range []VerifiedNormalOffsetSweepJoin{first, second} {
		j := joins[i].Original
		if j.Edges != original.Edges || !j.ReverseB || j.BStationRatio.Cmp(big.NewRat(3, 2)) != 0 {
			t.Fatal("lost middle chart station transport", i, j)
		}
	}
	checkSweepChainBoundary(t, chain, [3]uint8{1 << SurfaceEdgeUMax, 1<<SurfaceEdgeVMin | 1<<SurfaceEdgeVMax, 1 << SurfaceEdgeUMin})

	// The middle raw v runs across the stripe, while its raw u station
	// decreases in physical y on both common walls.
	for _, displacement := range []*big.Rat{big.NewRat(-1, 11), big.NewRat(1, 17), big.NewRat(2, 13)} {
		for i, x := range []*big.Rat{big.NewRat(5, 8), big.NewRat(13, 8)} {
			join := joins[i]
			var points [2]RationalSurfaceOffsetPoint
			for j, owner := range join.Owners {
				face, err := sweeps[owner].Boundary(NormalOffsetSweepSide(join.Original.Edges[j]))
				if err != nil {
					t.Fatal(err)
				}
				free := new(big.Rat)
				if owner == 1 {
					free = big.NewRat(14, 5)
				}
				points[j], err = face.EvaluateRational(ctx, free, displacement, SurfacePatchEvaluationOptions{},
					SurfaceNormalOffsetEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
			}
			// Both source x derivatives vanish at their wall. At y=0
			// the exact original unit normal is the rotated local Z.
			world := obliqueSweepTestTransform(x, new(big.Rat), displacement)
			for axis := range world {
				for _, point := range points {
					if point.Lower[axis].Cmp(world[axis]) > 0 || point.Upper[axis].Cmp(world[axis]) < 0 {
						t.Fatal("authored seam did not retain its independent physical station", i, displacement, point, world)
					}
				}
			}
		}
	}
	outerA, err := sweeps[0].EvaluateRational(ctx, big.NewRat(5, 2), big.NewRat(1, 4), big.NewRat(2, 13),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	outerC, err := sweeps[2].EvaluateRational(ctx, big.NewRat(5, 2), big.NewRat(-1, 4), big.NewRat(-1, 11),
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, ahi := obliqueSweepTestProjection(outerA, *first.Covector)
	clo, _ := obliqueSweepTestProjection(outerC, *first.Covector)
	if new(big.Rat).Sub(clo, ahi).Cmp(proof.PlaneGap) <= 0 {
		t.Fatal("outer chart points at unequal displacement should exceed the wall gap", ahi, clo)
	}

	joins[0].Coordinate.SetInt64(900)
	(*joins[1].Original.Covector)[0].SetInt64(900)
	joins[1].Original.BStationRatio.SetInt64(900)
	proof.PlaneGap.SetInt64(900)
	recovered := chain.Joins()
	if recovered[0].Coordinate.Cmp(ab.coordinate) != 0 ||
		(*recovered[1].Original.Covector)[0].Cmp((*second.Covector)[0]) != 0 ||
		recovered[1].Original.BStationRatio.Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal("caller mutation reached the original join certificate", recovered)
	}

	short := SurfacePatchOptions{MaxOperations: proof.ExactOperations - 1}
	pair, partial, err := NewVerifiedNormalOffsetSweepChain(ctx, ab, bc, short)
	if !errors.Is(err, ErrConvergence) || pair != nil || partial.Complete || partial.PlaneGap != nil {
		t.Fatal("short chain work budget published a partial volume", pair, partial, err)
	}
}

func TestVerifiedNormalOffsetSweepChainChecksFullParallelOrder(t *testing.T) {
	ctx := context.Background()
	left := sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}})
	middle := sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {4, 4}})
	right := sweepChainPlanarChart(t, [2][2]int64{{4, 4}, {8, 8}})
	ascending := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	descending := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMin, SurfaceEdgeUMax}, Axis: 0, AInteriorSign: 1}
	first := proveSweepChainPair(t, right, middle, descending)
	secondJoin := descending
	secondJoin.Covector = &[3]*big.Rat{big.NewRat(5, 2)}
	second := proveSweepChainPair(t, middle, left, secondJoin)
	chain, proof, err := NewVerifiedNormalOffsetSweepChain(ctx, first, second, SurfacePatchOptions{})
	if err != nil || chain == nil || !proof.Complete || proof.PlaneGap.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("reversed plane order or axis/covector scaling did not preserve the volume", chain, proof, err)
	}
	checkSweepChainBoundary(t, chain, [3]uint8{1 << SurfaceEdgeUMin, 1<<SurfaceEdgeUMin | 1<<SurfaceEdgeUMax, 1 << SurfaceEdgeUMax})

	// In the planar control each exact source volume is area 1 times
	// signed sweep width 1/5. The disjoint interiors give sum 3/5.
	volume := new(big.Rat)
	for _, sweep := range chain.Sweeps() {
		bound, err := sweep.VolumeRational(ctx, NormalOffsetSweepVolumeOptions{AbsoluteError: big.NewRat(1, 1000)})
		if err != nil || !bound.Complete || bound.Bounds.Lower.Cmp(bound.Bounds.Upper) != 0 {
			t.Fatal("expected independently exact planar source volume", bound, err)
		}
		volume.Add(volume, bound.Bounds.Lower)
	}
	if volume.Cmp(big.NewRat(3, 5)) != 0 {
		t.Fatal("wrong composed planar material volume", volume)
	}

	ab := proveSweepChainPair(t, left, middle, ascending)
	// Both pair inputs are independently complete, but reusing the
	// middle wall folds the chain over and the two end charts overlap.
	sameRegion := sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}})
	folded := proveSweepChainPair(t, middle, sameRegion, descending)
	otherMiddle := sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {4, 4}})
	wrongOwner := proveSweepChainPair(t, otherMiddle, right, ascending)
	// The genuine related second wall x-y/4=1 has a different
	// covector despite the two original middle sides being opposite.
	wedge := sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {4, 5}})
	outsideWedge := sweepChainPlanarChart(t, [2][2]int64{{4, 5}, {8, 9}})
	firstWedge := proveSweepChainPair(t, left, wedge, ascending)
	secondWedgeJoin := ascending
	secondWedgeJoin.Covector = &[3]*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 4)}
	secondWedge := proveSweepChainPair(t, wedge, outsideWedge, secondWedgeJoin)
	for _, negative := range []struct {
		name   string
		ab, bc *VerifiedNormalOffsetSweepPair
	}{
		{"folded middle wall and overlapping ends", ab, folded},
		{"different original middle owner", ab, wrongOwner},
		{"nonparallel walls on opposite middle sides", firstWedge, secondWedge},
	} {
		t.Run(negative.name, func(t *testing.T) {
			chain, proof, err := NewVerifiedNormalOffsetSweepChain(ctx, negative.ab, negative.bc, SurfacePatchOptions{})
			if !errors.Is(err, ErrDomain) || chain != nil || proof.Complete || proof.PlaneGap != nil {
				t.Fatal("uncertified chain published a material volume", chain, proof, err)
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	chain, proof, err = NewVerifiedNormalOffsetSweepChain(cancelled, first, second, SurfacePatchOptions{})
	if !errors.Is(err, context.Canceled) || chain != nil || proof.Complete || proof.PlaneGap != nil {
		t.Fatal("cancelled chain published material", chain, proof, err)
	}
}

func checkSweepChainBoundary(t *testing.T, chain *VerifiedNormalOffsetSweepChain, omitted [3]uint8) {
	t.Helper()
	faces := chain.Boundary()
	if len(faces) != 14 {
		t.Fatal("expected three original charts with only the four internal wall copies removed", len(faces))
	}
	var index int
	for owner, sweep := range chain.Sweeps() {
		for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
			if omitted[owner]&(1<<side) != 0 {
				continue
			}
			original, _ := sweep.Boundary(side)
			actual := faces[index]
			index++
			if actual.Owner != owner || actual.Side != side || actual.Face != original {
				t.Fatal("outer face lost original identity, parameters, or winding", owner, side, actual)
			}
		}
	}
}

func proveSweepChainPair(t *testing.T, a, b *VerifiedNormalOffsetSweep, join VerifiedNormalOffsetSweepJoin) *VerifiedNormalOffsetSweepPair {
	t.Helper()
	pair, proof, err := NewVerifiedNormalOffsetSweepPair(context.Background(), a, b, join, NormalOffsetCoverageOptions{})
	if err != nil || pair == nil || !proof.Complete {
		t.Fatal("fixture must supply a genuinely verified independent pair", proof, err)
	}
	return pair
}

func sweepChainPlanarChart(t *testing.T, x [2][2]int64) *VerifiedNormalOffsetSweep {
	t.Helper()
	ctx := context.Background()
	var controls [][4]*big.Rat
	for _, side := range x {
		for j, pole := range side {
			controls = append(controls, [4]*big.Rat{big.NewRat(pole, 4), big.NewRat(int64(j), 1), new(big.Rat), big.NewRat(1, 1)})
		}
	}
	source, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, controls, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, big.NewRat(-1, 10), big.NewRat(1, 10),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("planar chart did not verify", proof, err)
	}
	return sweep
}

func sweepChainPlacedJoins() (VerifiedNormalOffsetSweepJoin, VerifiedNormalOffsetSweepJoin) {
	first := obliqueSweepPairJoin()
	second := obliqueSweepPairJoin()
	second.Edges, second.AInteriorSign = [2]SurfaceEdge{SurfaceEdgeVMax, SurfaceEdgeUMin}, 1
	for _, q := range *second.Covector {
		q.Mul(q, big.NewRat(-3, 2))
	}
	return first, second
}

// This authors three original supports of the quartic graph
// z=[(x-5/8)(x-13/8)]²/512 + y²/64 before proper rational rotation.
// Both original wall normals have zero transverse component.
func placedSweepChainFixture(t *testing.T) (sweeps [3]*VerifiedNormalOffsetSweep) {
	t.Helper()
	ctx := context.Background()
	xHeights := [3][5]*big.Rat{
		{big.NewRat(4, 1), big.NewRat(1, 1), big.NewRat(1, 6), new(big.Rat), new(big.Rat)},
		{new(big.Rat), new(big.Rat), big.NewRat(1, 6), new(big.Rat), new(big.Rat)},
		{new(big.Rat), new(big.Rat), big.NewRat(1, 6), big.NewRat(1, 1), big.NewRat(4, 1)},
	}
	for owner := range sweeps {
		recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 4, DegreeV: 2, Domain: ParameterRectangle{2, 3, -1, 1}}
		view := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if owner == 1 {
			recipe.DegreeU, recipe.DegreeV = 2, 4
			recipe.Domain = ParameterRectangle{2, 4, -1, 0}
			view = NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		for i := 0; i <= recipe.DegreeU; i++ {
			for j := 0; j <= recipe.DegreeV; j++ {
				xi, yi, weight := i, j, big.NewRat(1, 1)
				if owner == 1 {
					xi, yi = j, 2-i
					weight = [3]*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(9, 4)}[i]
				}
				x, y := big.NewRat(int64(5+8*(owner-1)+2*xi), 8), big.NewRat(int64(yi-1), 1)
				z := new(big.Rat).Quo(xHeights[owner][xi], big.NewRat(512, 1))
				z.Add(z, big.NewRat([3]int64{1, -1, 1}[yi], 64))
				world := obliqueSweepTestTransform(x, y, z)
				recipe.Controls = append(recipe.Controls, [4]string{
					world[0].Mul(world[0], weight).RatString(), world[1].Mul(world[1], weight).RatString(),
					world[2].Mul(world[2], weight).RatString(), weight.RatString(),
				})
			}
		}
		source, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, big.NewRat(-1, 11), big.NewRat(2, 13),
			view, NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("independently authored quartic chart did not verify", owner, proof, err)
		}
		sweeps[owner] = sweep
	}
	return sweeps
}
