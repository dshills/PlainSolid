package geom

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"
)

func TestVerifiedNonparallelNormalOffsetSweepChainRetainsOriginals(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	sweeps, joins := nonparallelSweepChainOriginals(t, false)
	ab := proveSweepChainPair(t, sweeps[0], sweeps[1], joins[0])
	bc := proveSweepChainPair(t, sweeps[1], sweeps[2], joins[1])
	separator := VerifiedNormalOffsetSweepChainSeparator{Covector: [3]*big.Rat{q(1, 1)}, Coordinate: q(1, 2)}
	chain, proof, err := NewVerifiedNonparallelNormalOffsetSweepChain(ctx, ab, bc, separator, NormalOffsetCoverageOptions{})
	if err != nil || chain == nil || !proof.Complete || chain.Sweeps() != sweeps || proof.Gap.Cmp(q(1, 1)) != 0 {
		t.Fatal("independently separated wedge chain did not verify", chain, proof, err)
	}
	for i, bound := range [2][2]*big.Rat{{q(-1, 1), q(0, 1)}, {q(1, 1), q(9, 4)}} {
		got := proof.EndRanges[i]
		if got.Lower.Cmp(bound[0]) != 0 || got.Upper.Cmp(bound[1]) != 0 {
			t.Fatal("end projection did not cover exactly its full planar original sweep extent", i, got, bound)
		}
	}
	checkSweepChainBoundary(t, chain, [3]uint8{1 << SurfaceEdgeUMax, 1<<SurfaceEdgeUMin | 1<<SurfaceEdgeUMax, 1 << SurfaceEdgeUMin})
	gotJoins := chain.Joins()
	if gotJoins[0].Owners != [2]int{0, 1} || gotJoins[1].Owners != [2]int{1, 2} ||
		gotJoins[0].Coordinate.Sign() != 0 || gotJoins[1].Coordinate.Cmp(q(1, 1)) != 0 ||
		(*gotJoins[1].Original.Covector)[1].Cmp(q(-1, 4)) != 0 {
		t.Fatal("lost original nonparallel plane or owner identity", gotJoins)
	}
	t.Logf("nonparallel original walls x=0 and x-y/4=1; exact end ranges %v, gap %s, work %d", proof.EndRanges, proof.Gap, proof.ExactOperations)

	if legacy, prior, err := NewVerifiedNormalOffsetSweepChain(ctx, ab, bc, SurfacePatchOptions{}); !errors.Is(err, ErrDomain) ||
		legacy != nil || prior.Complete || prior.PlaneGap != nil {
		t.Fatal("the original parallel constructor must retain its original gate", legacy, prior, err)
	}
	short := NormalOffsetCoverageOptions{Patch: SurfacePatchOptions{MaxOperations: proof.ExactOperations - 1}}
	if incomplete, p, err := NewVerifiedNonparallelNormalOffsetSweepChain(ctx, ab, bc, separator, short); !errors.Is(err, ErrConvergence) ||
		incomplete != nil || p.Complete || p.Gap != nil || p.Separator != nil || p.EndRanges[0].Lower != nil {
		t.Fatal("short work must retain no partially verified chain", incomplete, p, err)
	}
	// Proof and constructor inputs have independent ownership.
	separator.Covector[0].SetInt64(0)
	separator.Coordinate.SetInt64(-99)
	proof.Separator.Covector[0].SetInt64(0)
	proof.Separator.Coordinate.SetInt64(99)

	// Integrate the exact original projected polygons and the
	// original rational thickness. Their prism volumes sum to 5/8.
	wantVolume := exactNonparallelControlVolume(t, sweeps)
	if wantVolume.Cmp(q(5, 8)) != 0 {
		t.Fatal("original corner control did not yield (1+9/8+1)*(1/5)", wantVolume)
	}
	opts := NormalOffsetSweepMomentOptions{VolumeTolerance: q(1, 100), CentroidTolerance: q(1000, 1),
		SecondMomentTolerance: q(1000, 1), InertiaTolerance: q(1000, 1)}
	moments, err := chain.MassMomentsRational(ctx, opts)
	if err != nil || !moments.Complete || moments.Volume.Lower.Cmp(wantVolume) > 0 || moments.Volume.Upper.Cmp(wantVolume) < 0 {
		t.Fatal("composed original volume missed the independent exact prism volume", moments, err)
	}
	t.Logf("exact reference volume %s in composed enclosure [%s,%s]", wantVolume,
		moments.Volume.Lower.FloatString(12), moments.Volume.Upper.FloatString(12))

	for _, test := range []struct {
		name     string
		point    [3]*big.Rat
		location NormalOffsetSweepLocation
		owners   []int
		raw      [][2]*big.Rat
		faces    []NormalOffsetSweepSide
	}{
		{"first original seam", [3]*big.Rat{q(0, 1), q(1, 3), q(1, 17)}, NormalOffsetSweepInterior,
			[]int{0, 1}, [][2]*big.Rat{{q(64, 1), q(1, 3)}, {q(0, 1), q(1, 3)}}, nil},
		{"second original seam", [3]*big.Rat{q(13, 12), q(1, 3), q(1, 17)}, NormalOffsetSweepInterior,
			[]int{1, 2}, [][2]*big.Rat{{q(1, 1), q(1, 3)}, {q(0, 1), q(1, 3)}}, nil},
		{"retained upper seam perimeter", [3]*big.Rat{q(13, 12), q(1, 3), q(1, 10)}, NormalOffsetSweepBoundary,
			[]int{1, 2}, [][2]*big.Rat{{q(1, 1), q(1, 3)}, {q(0, 1), q(1, 3)}}, []NormalOffsetSweepSide{NormalOffsetSweepUpper}},
		{"first end on remote side of extended second plane", [3]*big.Rat{q(-7, 8), q(-9, 1), q(1, 17)}, NormalOffsetSweepInterior,
			[]int{0}, [][2]*big.Rat{{q(8, 1), q(5, 8)}}, nil},
		{"intersection of extended planes", [3]*big.Rat{q(0, 1), q(-4, 1), q(1, 17)}, NormalOffsetSweepExterior, nil, nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := chain.LocateRational(ctx, test.point, NormalOffsetSweepQueryOptions{})
			if err != nil || got.Location != test.location || len(got.Owners) != len(test.owners) {
				t.Fatal("incorrect nonparallel point classification", got, err)
			}
			for i, owner := range got.Owners {
				if owner.Owner != test.owners[i] || !slices.Equal(owner.Faces, test.faces) {
					t.Fatal("lost original seam station ownership", owner, test)
				}
				raw := test.raw[i]
				checkJoinedSweepPointStation(t, sweeps[owner.Owner], test.point, owner, [3]*big.Rat{raw[0], raw[1], test.point[2]}, true)
			}
		})
	}
}

func TestVerifiedNonparallelNormalOffsetSweepChainUsesFullSignedLayers(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	sweeps, joins := nonparallelSweepChainOriginals(t, false)
	ab := proveSweepChainPair(t, sweeps[0], sweeps[1], joins[0])
	bc := proveSweepChainPair(t, sweeps[1], sweeps[2], joins[1])
	tilted := VerifiedNormalOffsetSweepChainSeparator{Covector: [3]*big.Rat{q(1, 1), nil, q(2, 1)}, Coordinate: q(1, 2)}
	chain, p, err := NewVerifiedNonparallelNormalOffsetSweepChain(ctx, ab, bc, tilted, NormalOffsetCoverageOptions{})
	if err != nil || chain == nil || !p.Complete || p.Gap.Cmp(q(3, 5)) != 0 ||
		p.EndRanges[0].Lower.Cmp(q(-6, 5)) != 0 || p.EndRanges[0].Upper.Cmp(q(1, 5)) != 0 ||
		p.EndRanges[1].Lower.Cmp(q(4, 5)) != 0 || p.EndRanges[1].Upper.Cmp(q(49, 20)) != 0 {
		t.Fatal("separator must certify the actual full offsets, including its normal component", chain, p, err)
	}
	t.Logf("tilted x+2z end separator encloses %v in %d exact operations", p.EndRanges, p.ExactOperations)
	bad := tilted
	bad.Covector[2] = q(6, 1)
	// The source-only projections would still separate. The original
	// signed layers now give A a maximum >=3/5 and C a minimum <=2/5.
	if chain, p, err := NewVerifiedNonparallelNormalOffsetSweepChain(ctx, ab, bc, bad, NormalOffsetCoverageOptions{MaxDepth: 5}); (!errors.Is(err, ErrConvergence) && !errors.Is(err, ErrDomain)) || chain != nil || p.Complete || p.Gap != nil {
		t.Fatal("a source-only separator cannot authenticate the displaced solids", chain, p, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if chain, p, err := NewVerifiedNonparallelNormalOffsetSweepChain(cancelled, ab, bc, tilted, NormalOffsetCoverageOptions{}); !errors.Is(err, context.Canceled) || chain != nil || p.Complete || p.Gap != nil {
		t.Fatal("cancelled separator published a chain", chain, p, err)
	}
}

func TestVerifiedNonparallelNormalOffsetSweepChainRefusesEndContact(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	sep := VerifiedNormalOffsetSweepChainSeparator{Covector: [3]*big.Rat{q(1, 1)}, Coordinate: q(1, 2)}
	t.Run("genuine affine L with end edge contact", func(t *testing.T) {
		a := sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}})
		b := sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {4, 4}})
		c := authoredSweepWedgeChart(t, [2][2][2]int64{{{0, 4}, {0, 8}}, {{4, 4}, {4, 8}}}, ParameterRectangle{0, 1, 0, 1},
			NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}})
		ab := proveSweepChainPair(t, a, b, VerifiedNormalOffsetSweepJoin{
			Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1,
		})
		bc := proveSweepChainPair(t, b, c, VerifiedNormalOffsetSweepJoin{
			Edges: [2]SurfaceEdge{SurfaceEdgeVMax, SurfaceEdgeVMin}, Axis: 1, AInteriorSign: -1,
		})
		d := q(1, 17)
		point := [3]*big.Rat{q(0, 1), q(1, 1), d}
		for _, end := range []*VerifiedNormalOffsetSweep{a, c} {
			contact, err := end.LocateRational(ctx, point, NormalOffsetSweepQueryOptions{})
			if err != nil || contact.Location != NormalOffsetSweepBoundary {
				t.Fatal("affine L end sweeps must genuinely touch at the common swept edge", contact, err)
			}
		}
		if got, p, err := NewVerifiedNonparallelNormalOffsetSweepChain(ctx, ab, bc, sep, NormalOffsetCoverageOptions{}); !errors.Is(err, ErrDomain) || got != nil || p.Complete {
			t.Fatal("touching end sweeps cannot have a strict closed-set separator", got, p, err)
		}
	})
	t.Run("opposite original walls with overlapping ends", func(t *testing.T) {
		sweeps, joins := nonparallelSweepChainOriginals(t, true)
		ab := proveSweepChainPair(t, sweeps[0], sweeps[1], joins[0])
		bc := proveSweepChainPair(t, sweeps[1], sweeps[2], joins[1])
		point := [3]*big.Rat{q(-139, 160), q(-75, 8), q(1, 17)}
		for _, i := range []int{0, 2} {
			contact, err := sweeps[i].LocateRational(ctx, point, NormalOffsetSweepQueryOptions{})
			if err != nil || contact.Location != NormalOffsetSweepInterior {
				t.Fatal("negative control needs real overlap across independently proved joins", i, contact, err)
			}
		}
		if got, p, err := NewVerifiedNonparallelNormalOffsetSweepChain(ctx, ab, bc, sep, NormalOffsetCoverageOptions{}); !errors.Is(err, ErrDomain) || got != nil || p.Complete || p.EndRanges[1].Lower != nil {
			t.Fatal("local pairs cannot stand in for strict end disjointness", got, p, err)
		}
	})
}

func nonparallelSweepChainOriginals(t *testing.T, overlap bool) (sweeps [3]*VerifiedNormalOffsetSweep, joins [2]VerifiedNormalOffsetSweepJoin) {
	t.Helper()
	projection := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
	sweeps[0] = authoredSweepWedgeChart(t, [2][2][2]int64{{{-4, -44}, {-4, -40}}, {{0, 0}, {0, 4}}},
		ParameterRectangle{0, 64, 0, 1}, projection)
	sweeps[1] = authoredSweepWedgeChart(t, [2][2][2]int64{{{0, 0}, {0, 4}}, {{4, 0}, {5, 4}}},
		ParameterRectangle{0, 1, 0, 1}, projection)
	if overlap {
		sweeps[2] = authoredSweepWedgeChart(t, [2][2][2]int64{{{4, 0}, {5, 4}}, {{-4, -40}, {-3, -36}}},
			ParameterRectangle{0, 2, 0, 1}, NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}})
	} else {
		sweeps[2] = authoredSweepWedgeChart(t, [2][2][2]int64{{{4, 0}, {5, 4}}, {{8, 0}, {9, 4}}},
			ParameterRectangle{0, 1, 0, 1}, projection)
	}
	joins[0] = VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	joins[1] = joins[0]
	joins[1].Covector = &[3]*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 4)}
	return sweeps, joins
}

func authoredSweepWedgeChart(t *testing.T, quarterXY [2][2][2]int64, domain ParameterRectangle,
	projection NormalOffsetSweepProjection) *VerifiedNormalOffsetSweep {
	t.Helper()
	ctx := context.Background()
	recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, Domain: domain}
	for _, row := range quarterXY {
		for _, p := range row {
			recipe.Controls = append(recipe.Controls, [4]string{big.NewRat(p[0], 4).RatString(), big.NewRat(p[1], 4).RatString(), "0", "1"})
		}
	}
	source, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, big.NewRat(-1, 10), big.NewRat(1, 10),
		projection, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("authored planar chart did not independently seal", len(proof.Unresolved), proof.ExactOperations, err)
	}
	return sweep
}

func exactNonparallelControlVolume(t *testing.T, sweeps [3]*VerifiedNormalOffsetSweep) *big.Rat {
	t.Helper()
	sum := new(big.Rat)
	for _, chart := range sweeps {
		domain, displacement := chart.Domain()
		stations := [4][2]float64{
			{domain.UMin, domain.VMin}, {domain.UMax, domain.VMin},
			{domain.UMax, domain.VMax}, {domain.UMin, domain.VMax},
		}
		var polygon [4][2]*big.Rat
		for j, uv := range stations {
			point, err := chart.EvaluateRational(context.Background(), new(big.Rat).SetFloat64(uv[0]),
				new(big.Rat).SetFloat64(uv[1]), new(big.Rat), SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for axis := range point.Lower {
				if point.Lower[axis].Cmp(point.Upper[axis]) != 0 {
					t.Fatal("expected rational authored prism vertex", point)
				}
			}
			polygon[j] = [2]*big.Rat{point.Lower[0], point.Lower[1]}
		}
		twiceArea := new(big.Rat)
		for j, p := range polygon {
			q := polygon[(j+1)%len(polygon)]
			twiceArea.Add(twiceArea, new(big.Rat).Sub(new(big.Rat).Mul(p[0], q[1]), new(big.Rat).Mul(p[1], q[0])))
		}
		twiceArea.Quo(twiceArea, big.NewRat(2, 1))
		thickness := new(big.Rat).Sub(displacement.Upper, displacement.Lower)
		sum.Add(sum, new(big.Rat).Mul(twiceArea, thickness))
	}
	return sum
}
