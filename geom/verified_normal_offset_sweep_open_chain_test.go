package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"
)

func TestVerifiedNormalOffsetSweepOpenChainFiveOriginalCharts(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	cuts := []int64{-4, 0, 6, 14, 16, 20}
	sweeps, joins, separators := authoredSweepOpenPrisms(t, cuts)
	reversed := slices.Clone(separators)
	slices.Reverse(reversed)
	options := NormalOffsetCoverageOptions{MaxCells: 12}
	chain, proof, err := NewVerifiedNormalOffsetSweepOpenChain(ctx, joins, reversed, options)
	if err != nil || chain == nil || !proof.Complete || proof.VisitedCells != 12 || len(proof.Nonadjacent) != 6 || !slices.Equal(chain.Sweeps(), sweeps) {
		t.Fatal("five original charts did not prove every independent separation under the shared root budget", chain, proof, err)
	}
	_, canonical, err := NewVerifiedNormalOffsetSweepOpenChain(ctx, joins, separators, options)
	if err != nil || !reflect.DeepEqual(proof, canonical) {
		t.Fatal("unordered original separator input changed canonical proof order or exact work", proof, canonical, err)
	}
	for n, separation := range proof.Nonadjacent {
		want := separators[n]
		i, j := want.Owners[0], want.Owners[1]
		if separation.Owners != want.Owners {
			t.Fatal("nonadjacent proof lost original pair order", separation.Owners, want.Owners)
		}
		y, z := want.Separator.Covector[1], want.Separator.Covector[2]
		shift := new(big.Rat).Mul(new(big.Rat).Abs(z), q(1, 10))
		lo := new(big.Rat).Neg(shift)
		hi := new(big.Rat).Set(shift)
		if y.Sign() < 0 {
			lo.Add(lo, y)
		} else {
			hi.Add(hi, y)
		}
		for k, owner := range [2]int{i, j} {
			low := new(big.Rat).Add(q(cuts[owner], 4), lo)
			high := new(big.Rat).Add(q(cuts[owner+1], 4), hi)
			if separation.Ranges[k].Lower.Cmp(low) != 0 || separation.Ranges[k].Upper.Cmp(high) != 0 {
				t.Fatal("independent original signed-layer projection missed its exact slab extrema", owner, separation)
			}
		}
		gap := new(big.Rat).Sub(separation.Ranges[1].Lower, separation.Ranges[0].Upper)
		if gap.Sign() <= 0 || separation.Gap.Cmp(gap) != 0 {
			t.Fatal("complete original pair certificate lacks its exact strict gap", separation)
		}
	}
	t.Logf("six independent nonadjacent owner proofs shared %d cells and %d exact operations", proof.VisitedCells, proof.ExactOperations)

	missing := separators[:len(separators)-1]
	duplicate := slices.Clone(separators)
	duplicate[len(duplicate)-1] = duplicate[0]
	for _, bad := range [][]VerifiedNormalOffsetSweepNonadjacentSeparator{missing, duplicate} {
		if got, p, err := NewVerifiedNormalOffsetSweepOpenChain(ctx, joins, bad, options); !errors.Is(err, ErrInvalid) ||
			got != nil || p.Complete || len(p.Nonadjacent) != 0 {
			t.Fatal("missing original pair proof cannot be replaced by a duplicate", got, p, err)
		}
	}
	short := options
	short.MaxCells--
	if got, p, err := NewVerifiedNormalOffsetSweepOpenChain(ctx, joins, separators, short); !errors.Is(err, ErrConvergence) ||
		got != nil || p.Complete || len(p.Nonadjacent) != 0 || p.VisitedCells != 11 {
		t.Fatal("shared root budget must also cover the last independent pair", got, p, err)
	}
	short = options
	short.Patch.MaxOperations = proof.ExactOperations - 1
	if got, p, err := NewVerifiedNormalOffsetSweepOpenChain(ctx, joins, separators, short); !errors.Is(err, ErrConvergence) ||
		got != nil || p.Complete || len(p.Nonadjacent) != 0 {
		t.Fatal("short exact work published a partial chain", got, p, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, p, err := NewVerifiedNormalOffsetSweepOpenChain(cancelled, joins, separators, options); !errors.Is(err, context.Canceled) ||
		got != nil || p.Complete || len(p.Nonadjacent) != 0 {
		t.Fatal("cancelled chain published a proof", got, p, err)
	}
	original := joins[0]
	joins[0] = nil
	chain.Sweeps()[0] = nil
	if chain.joins[0] != original || chain.Sweeps()[0] != sweeps[0] {
		t.Fatal("caller slice mutation reached the sealed originals")
	}
	faces := chain.Boundary()
	checkOriginalOpenPrismFaces(t, chain.Sweeps(), faces)
	if len(faces) != 22 {
		t.Fatal("expected 22 original retained faces", len(faces))
	}
	for i, join := range chain.Joins() {
		if join.Owners != [2]int{i, i + 1} || join.Coordinate.Cmp(q(cuts[i+1], 4)) != 0 {
			t.Fatal("the original full-wall plane or owner order was lost", join)
		}
	}

	// The authored physical union is a 6 x 1 x 1/5 box.
	momentOptions := fourSweepMomentOptions(q(1, 1_000_000))
	momentOptions.MaxCells = 5
	moments, err := chain.MassMomentsRational(ctx, momentOptions)
	if err != nil || !moments.Complete || moments.VisitedCells != 5 ||
		moments.Volume.Lower.Cmp(q(6, 5)) != 0 || moments.Volume.Upper.Cmp(q(6, 5)) != 0 ||
		moments.Centroid[0].Lower.Cmp(q(2, 1)) != 0 || moments.Centroid[0].Upper.Cmp(q(2, 1)) != 0 {
		t.Fatal("exact composed volume or centroid missed the independently authored five-prism box", moments, err)
	}
	areaOptions := NormalOffsetSweepAreaOptions{AbsoluteError: q(0, 1), MaxCells: 22}
	exactArea, err := chain.BoundaryAreaRational(ctx, areaOptions)
	if err != nil || !exactArea.Complete || exactArea.VisitedCells != 22 ||
		exactArea.Bounds.Lower.Cmp(q(74, 5)) != 0 || exactArea.Bounds.Upper.Cmp(q(74, 5)) != 0 {
		t.Fatal("five-owner specialized boundary area is not the exact box area", exactArea, err)
	}
	checkOpenAreaOwners(t, exactArea, faces)
	directedOptions := NormalOffsetSweepAreaEnclosureOptions{NormalOffsetSweepAreaOptions: areaOptions}
	directed, err := chain.EncloseBoundaryAreaRational(ctx, directedOptions)
	if err != nil || !directed.Complete || directed.VisitedCells != 22 ||
		directed.Bounds.Lower.Cmp(exactArea.Bounds.Lower) != 0 || directed.Bounds.Upper.Cmp(exactArea.Bounds.Upper) != 0 {
		t.Fatal("five-owner directed area did not verify the exact physical boundary", directed, err)
	}
	checkOpenAreaOwners(t, directed, faces)
	directedOptions.MaxCells--
	prefix, err := chain.EncloseBoundaryAreaRational(ctx, directedOptions)
	if !errors.Is(err, ErrConvergence) || prefix.Complete || prefix.Bounds.Lower != nil || len(prefix.Faces) != 21 {
		t.Fatal("incomplete five-chart face cover published a total", prefix, err)
	}
	checkOpenAreaOwners(t, prefix, faces[:21])
	point := [3]*big.Rat{q(9, 2), q(1, 2), q(1, 50)}
	dist, err := chain.BoundPointBoundaryDistanceRational(ctx, point, NormalOffsetSweepDistanceOptions{MaxCells: 22, SquaredTolerance: new(big.Rat)})
	if err != nil || !dist.Complete || dist.VisitedCells != 22 || dist.Witness == nil ||
		dist.Witness.Owner != 4 || dist.Witness.Side != NormalOffsetSweepUpper ||
		dist.Squared.Lower.Cmp(q(4, 625)) != 0 || dist.Squared.Upper.Cmp(q(4, 625)) != 0 {
		t.Fatal("distance search lost the fifth original source owner", dist, err)
	}
	checkJoinedSweepDistanceWitness(t, sweeps, faces, point, dist)
	t.Logf("volume %s, exact area %s across %d retained faces, boundary distance witness owner %d",
		moments.Volume.Lower, exactArea.Bounds.Lower, len(faces), dist.Witness.Owner)
}

func TestVerifiedNormalOffsetSweepOpenChainBoundedOwnerCounts(t *testing.T) {
	for _, count := range []int{2, 8} {
		cuts := make([]int64, count+1)
		for i := range cuts {
			cuts[i] = int64(4 * i)
		}
		sweeps, joins, separators := authoredSweepOpenPrisms(t, cuts)
		visits := (count - 1) * (count - 2)
		chain, proof, err := NewVerifiedNormalOffsetSweepOpenChain(context.Background(), joins, separators,
			NormalOffsetCoverageOptions{MaxCells: visits})
		if err != nil || chain == nil || !proof.Complete || len(proof.Nonadjacent) != visits/2 ||
			proof.VisitedCells != visits || !slices.Equal(chain.Sweeps(), sweeps) || len(chain.Boundary()) != 4*count+2 {
			t.Fatal("owner bound changed the original chart ledger", count, chain, proof, err)
		}
		checkOriginalOpenPrismFaces(t, sweeps, chain.Boundary())
		t.Logf("qualified %d original owners, %d independent pairs, %d retained faces", count, visits/2, len(chain.Boundary()))
		if count == 8 {
			tooMany := append(joins, joins[0])
			chain, proof, err := NewVerifiedNormalOffsetSweepOpenChain(context.Background(), tooMany, nil, NormalOffsetCoverageOptions{})
			if !errors.Is(err, ErrInvalid) || chain != nil || proof.Complete {
				t.Fatal("ninth original owner escaped the bounded chain gate", chain, proof, err)
			}
		}
	}
}

func TestVerifiedNormalOffsetSweepOpenChainRefusesNonadjacentContact(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	domain := ParameterRectangle{0, 2, 0, 1}
	project := NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
	d := authoredSweepWedgeChart(t, [2][2][2]int64{{{-4, -40}, {-3, -36}}, {{-12, -80}, {-11, -76}}}, domain, project)
	e := authoredSweepWedgeChart(t, [2][2][2]int64{{{-12, -80}, {-11, -76}}, {{-20, -120}, {-19, -116}}}, domain, project)
	for _, test := range []struct {
		name      string
		point     [3]*big.Rat
		location  NormalOffsetSweepLocation
		separator VerifiedNormalOffsetSweepChainSeparator
	}{
		{"positive interior overlap", [3]*big.Rat{q(-139, 160), q(-75, 8), q(1, 17)}, NormalOffsetSweepInterior,
			VerifiedNormalOffsetSweepChainSeparator{Covector: [3]*big.Rat{q(1, 1)}, Coordinate: q(1, 2)}},
		{"shared swept edge only", [3]*big.Rat{q(-1, 1), q(-10, 1), q(1, 17)}, NormalOffsetSweepBoundary,
			VerifiedNormalOffsetSweepChainSeparator{Covector: [3]*big.Rat{q(10, 1), q(-1, 1)}, Coordinate: q(0, 1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			abc, joins := nonparallelSweepChainOriginals(t, true)
			depth := 16
			if test.location == NormalOffsetSweepBoundary {
				// In xy, A has y-10*x in [0,1], while C has
				// y-10*x = -10 + 5*u - 3*v/2. Equality occurs
				// only at C's original corner u=2,v=0.
				abc[0] = authoredSweepWedgeChart(t, [2][2][2]int64{{{-4, -40}, {-4, -36}}, {{0, 0}, {0, 4}}},
					ParameterRectangle{0, 64, 0, 1}, NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}})
				depth = 5
			}
			pairs := []*VerifiedNormalOffsetSweepPair{
				proveSweepChainPair(t, abc[0], abc[1], joins[0]),
				proveSweepChainPair(t, abc[1], abc[2], joins[1]),
				proveSweepChainPair(t, abc[2], d, joins[1]),
				proveSweepChainPair(t, d, e, joins[1]),
			}
			for _, owner := range []int{0, 2} {
				got, err := abc[owner].LocateRational(ctx, test.point, NormalOffsetSweepQueryOptions{})
				if err != nil || got.Location != test.location {
					t.Fatal("negative requires exact contact between the independent nonadjacent original owners", owner, got, err)
				}
			}
			var separators []VerifiedNormalOffsetSweepNonadjacentSeparator
			for i := 0; i < 3; i++ {
				for j := i + 2; j < 5; j++ {
					separators = append(separators, VerifiedNormalOffsetSweepNonadjacentSeparator{
						Owners: [2]int{i, j}, Separator: test.separator,
					})
				}
			}
			chain, proof, err := NewVerifiedNormalOffsetSweepOpenChain(ctx, pairs, separators, NormalOffsetCoverageOptions{MaxDepth: depth})
			if (!errors.Is(err, ErrDomain) && !errors.Is(err, ErrConvergence)) || chain != nil || proof.Complete || len(proof.Nonadjacent) != 0 {
				t.Fatal("complete adjacent local joins cannot substitute for an absent strict gap", chain, proof, err)
			}
			t.Logf("four genuine adjacent pairs with exact contact of nonadjacent owners 0 and 2 refused after %d operations", proof.ExactOperations)
		})
	}
}

func authoredSweepOpenPrisms(t *testing.T, cuts []int64) (
	sweeps []*VerifiedNormalOffsetSweep, joins []*VerifiedNormalOffsetSweepPair, separators []VerifiedNormalOffsetSweepNonadjacentSeparator) {
	t.Helper()
	q := big.NewRat
	join := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	for i := 1; i < len(cuts); i++ {
		a, b := cuts[i-1], cuts[i]
		sweeps = append(sweeps, sweepChainPlanarChart(t, [2][2]int64{{a, a}, {b, b}}))
		if i > 1 {
			joins = append(joins, proveSweepChainPair(t, sweeps[i-2], sweeps[i-1], join))
		}
	}
	for i := 0; i < len(sweeps)-2; i++ {
		for j := i + 2; j < len(sweeps); j++ {
			y := q(int64(2*((i+j)%2)-1), 8)
			z := q(int64((i+j)%3-1), 1)
			separators = append(separators, VerifiedNormalOffsetSweepNonadjacentSeparator{
				Owners: [2]int{i, j},
				Separator: VerifiedNormalOffsetSweepChainSeparator{
					Covector: [3]*big.Rat{q(1, 1), y, z}, Coordinate: q(cuts[i+1]+cuts[j], 8),
				},
			})
		}
	}
	return sweeps, joins, separators
}

func checkOriginalOpenPrismFaces(t *testing.T, sweeps []*VerifiedNormalOffsetSweep, faces []VerifiedNormalOffsetSweepBoundaryFace) {
	t.Helper()
	index := 0
	for i, sweep := range sweeps {
		for side := NormalOffsetSweepUMin; side <= NormalOffsetSweepUpper; side++ {
			if (i > 0 && side == NormalOffsetSweepUMin) || (i < len(sweeps)-1 && side == NormalOffsetSweepUMax) {
				continue
			}
			face, _ := sweep.Boundary(side)
			if index >= len(faces) || faces[index].Owner != i || faces[index].Side != side || faces[index].Face != face {
				t.Fatal("dynamic owner order did not retain a specific original analytic face", index, i, side)
			}
			index++
		}
	}
	if index != len(faces) {
		t.Fatal("an omitted join or duplicate entered the analytic outer face list", index, len(faces))
	}
}

func checkOpenAreaOwners(t *testing.T, area NormalOffsetSweepJoinedBoundaryArea, faces []VerifiedNormalOffsetSweepBoundaryFace) {
	t.Helper()
	if len(area.Faces) != len(faces) {
		t.Fatal("incorrect original directed area cover size", len(area.Faces), len(faces))
	}
	for i, face := range area.Faces {
		if face.Owner != faces[i].Owner || face.Side != faces[i].Side {
			t.Fatal("directed cover changed an original owner identity", i, face, faces[i])
		}
	}
}
