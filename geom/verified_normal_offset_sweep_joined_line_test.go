package geom

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"
)

func TestVerifiedJoinedNormalOffsetSweepIntersectsAcrossOmittedWalls(t *testing.T) {
	ctx := context.Background()
	sweeps, pair, chain := straightLineSweepChain(t)
	q := big.NewRat
	dir := [3]*big.Rat{q(3, 1), q(0, 1), q(0, 1)}
	origin := [3]*big.Rat{q(-2, 1), q(2, 5), q(1, 100)}
	o := NormalOffsetSweepJoinedLineOptions{}
	for i, c := range []struct {
		query func(context.Context, [3]*big.Rat, [3]*big.Rat, NormalOffsetSweepJoinedLineOptions) (NormalOffsetSweepJoinedLineIntersection, error)
		faces []VerifiedNormalOffsetSweepBoundaryFace
		end   *big.Rat
	}{{pair.IntersectRationalLine, pair.Boundary(), q(1, 1)}, {chain.IntersectRationalLine, chain.Boundary(), q(4, 3)}} {
		got, err := c.query(ctx, origin, dir, o)
		checkJoinedLineMaterial(t, got, err, [][2]*big.Rat{{q(1, 3), c.end}})
		if len(got.BoundaryEvents) != 2 || len(got.BoundarySpans) != 0 {
			t.Fatal("internal wall propagated into the outer boundary ledger", got)
		}
		for j, event := range got.BoundaryEvents {
			owner, side := 0, NormalOffsetSweepUMin
			if j == 1 {
				owner, side = i+1, NormalOffsetSweepUMax
			}
			if len(event.Witnesses) != 1 || event.Witnesses[0].Owner != owner ||
				!slices.Equal(event.Witnesses[0].Faces, []NormalOffsetSweepSide{side}) ||
				event.Witnesses[0].Station[2].Cmp(q(1, 100)) != 0 {
				t.Fatal("crossing lost its original outer face or nonbinary station", event)
			}
		}
		checkJoinedLineRationalWitnesses(t, sweeps[:i+2], c.faces, origin, dir, got)
		t.Logf("across %d original omitted walls, material [%s,%s], %d outer events, %d exact operations",
			i+1, got.Material[0].Lower, got.Material[0].Upper, len(got.BoundaryEvents), got.ExactOperations)
		short := o
		short.Patch.MaxOperations = got.ExactOperations - 1
		denied, err := c.query(ctx, origin, dir, short)
		checkJoinedLineAtomic(t, denied, err, ErrConvergence)
	}

	reversed := origin
	reversed[0] = q(3, 1)
	backward := [3]*big.Rat{q(-3, 1), q(0, 1), q(0, 1)}
	r, err := chain.IntersectRationalLine(ctx, reversed, backward, o)
	checkJoinedLineMaterial(t, r, err, [][2]*big.Rat{{q(1, 3), q(4, 3)}})
	if len(r.BoundaryEvents) != 2 || !slices.Equal(joinedLineOwners(r.BoundaryEvents[0]), []int{2}) ||
		!slices.Equal(joinedLineOwners(r.BoundaryEvents[1]), []int{0}) {
		t.Fatal("line direction reversal changed outer station ownership", r)
	}
	checkJoinedLineRationalWitnesses(t, sweeps[:], chain.Boundary(), reversed, backward, r)

	outside := origin
	outside[1] = q(2, 1)
	miss, err := chain.IntersectRationalLine(ctx, outside, dir, o)
	checkJoinedLineMaterial(t, miss, err, nil)
	if len(miss.BoundaryEvents) != 0 || len(miss.BoundarySpans) != 0 {
		t.Fatal("proved whole-union miss reported boundary data", miss)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	stopped, err := chain.IntersectRationalLine(cancelled, origin, dir, o)
	checkJoinedLineAtomic(t, stopped, err, context.Canceled)
}

func TestVerifiedJoinedNormalOffsetSweepIntersectsRetainedOuterOwners(t *testing.T) {
	ctx := context.Background()
	sweeps, pair, chain := straightLineSweepChain(t)
	q := big.NewRat
	along := [3]*big.Rat{q(3, 1), q(0, 1), q(0, 1)}
	capEdge := [3]*big.Rat{q(-2, 1), q(0, 1), q(1, 10)}
	o := NormalOffsetSweepJoinedLineOptions{}
	got, err := chain.IntersectRationalLine(ctx, capEdge, along, o)
	checkJoinedLineMaterial(t, got, err, [][2]*big.Rat{{q(1, 3), q(4, 3)}})
	if len(got.BoundarySpans) != 3 || len(got.BoundaryEvents) != 4 {
		t.Fatal("retained edge ownership was lost across a materially continuous interval", got)
	}
	for i, bound := range []*big.Rat{q(1, 3), q(2, 3), q(1, 1), q(4, 3)} {
		event := got.BoundaryEvents[i]
		expected := [][]int{{0}, {0, 1}, {1, 2}, {2}}[i]
		if event.Parameter.Cmp(bound) != 0 || !slices.Equal(joinedLineOwners(event), expected) {
			t.Fatal("coincident outer event omitted an original owner", event, expected)
		}
	}
	for i, p := range got.BoundarySpans {
		if p.Owner != i || !slices.Equal(p.Span.BoundaryFaces, []NormalOffsetSweepSide{NormalOffsetSweepVMin, NormalOffsetSweepUpper}) {
			t.Fatal("original cap edge span lost its owner or included an internal wall face", got.BoundarySpans)
		}
	}
	checkJoinedLineRationalWitnesses(t, sweeps[:], chain.Boundary(), capEdge, along, got)
	got.BoundarySpans[0].Span.Start.Station[0].SetInt64(700)
	if got.BoundaryEvents[0].Witnesses[0].Station[0].Sign() != 0 {
		t.Fatal("span endpoint mutation altered an independent original event witness", got.BoundaryEvents[0])
	}

	// This whole line lies in the removed A/B wall. Its material
	// interval remains, and both original outer wall owners survive
	// only at the two true outer endpoints.
	wall := [3]*big.Rat{q(0, 1), q(-1, 1), q(0, 1)}
	parallel := [3]*big.Rat{q(0, 1), q(2, 1), q(0, 1)}
	wallHit, err := pair.IntersectRationalLine(ctx, wall, parallel, o)
	checkJoinedLineMaterial(t, wallHit, err, [][2]*big.Rat{{q(1, 2), q(1, 1)}})
	if len(wallHit.BoundarySpans) != 0 || len(wallHit.BoundaryEvents) != 2 ||
		!slices.Equal(joinedLineOwners(wallHit.BoundaryEvents[0]), []int{0, 1}) ||
		!slices.Equal(joinedLineOwners(wallHit.BoundaryEvents[1]), []int{0, 1}) {
		t.Fatal("coincident internal wall was not replaced by genuine outer endpoints", wallHit)
	}
	checkJoinedLineRationalWitnesses(t, sweeps[:2], pair.Boundary(), wall, parallel, wallHit)

	// The exact perimeter of the same wall lies in both retained cap
	// owners on the whole common line interval.
	wall[2] = q(1, 10)
	perimeter, err := pair.IntersectRationalLine(ctx, wall, parallel, o)
	checkJoinedLineMaterial(t, perimeter, err, [][2]*big.Rat{{q(1, 2), q(1, 1)}})
	if len(perimeter.BoundarySpans) != 2 || perimeter.BoundarySpans[0].Owner != 0 || perimeter.BoundarySpans[1].Owner != 1 {
		t.Fatal("coincident retained caps need both original positive-length spans", perimeter)
	}
	for _, p := range perimeter.BoundarySpans {
		if !slices.Equal(p.Span.BoundaryFaces, []NormalOffsetSweepSide{NormalOffsetSweepUpper}) {
			t.Fatal("coincident perimeter carries an omitted wall face", p)
		}
	}
	checkJoinedLineRationalWitnesses(t, sweeps[:2], pair.Boundary(), wall, parallel, perimeter)

	corner := [3]*big.Rat{q(0, 1), q(0, 1), q(1, 10)}
	tangent := [3]*big.Rat{q(0, 1), q(1, 1), q(1, 1)}
	point, err := pair.IntersectRationalLine(ctx, corner, tangent, o)
	checkJoinedLineMaterial(t, point, err, [][2]*big.Rat{{q(0, 1), q(0, 1)}})
	if len(point.BoundarySpans) != 0 || len(point.BoundaryEvents) != 1 ||
		!slices.Equal(joinedLineOwners(point.BoundaryEvents[0]), []int{0, 1}) {
		t.Fatal("isolated perimeter point lost either owner or acquired a positive-length span", point)
	}
	for _, owner := range point.BoundaryEvents[0].Witnesses {
		if !slices.Equal(owner.Faces, []NormalOffsetSweepSide{NormalOffsetSweepVMin, NormalOffsetSweepUpper}) {
			t.Fatal("wrong original outer faces for isolated perimeter point", owner)
		}
	}
	checkJoinedLineRationalWitnesses(t, sweeps[:2], pair.Boundary(), corner, tangent, point)
}

func TestVerifiedJoinedNormalOffsetSweepIntersectsDisjointLineComponents(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	var sweeps [3]*VerifiedNormalOffsetSweep
	for owner := range sweeps {
		// Outer charts have y in [1,2] at their far ends but all
		// shared walls have the original y range [0,1].
		shifts := [3][2]int64{{1, 0}, {0, 0}, {0, 1}}[owner]
		var net [][4]*big.Rat
		for xi, shift := range shifts {
			for yi := range 2 {
				net = append(net, [4]*big.Rat{q(int64(owner+xi-1), 1), q(int64(yi)+shift, 1), q(0, 1), q(1, 1)})
			}
		}
		p, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, net, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweeps[owner], _, err = NewVerifiedNormalOffsetSweep(ctx, p, q(-1, 10), q(1, 10),
			NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	join := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	_, chain := joinedSweepMomentFixture(t, sweeps, join, join)
	p, d := [3]*big.Rat{q(0, 1), q(3, 2), q(0, 1)}, [3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}
	got, err := chain.IntersectRationalLine(ctx, p, d, NormalOffsetSweepJoinedLineOptions{})
	checkJoinedLineMaterial(t, got, err, [][2]*big.Rat{{q(-1, 1), q(-1, 2)}, {q(3, 2), q(2, 1)}})
	if len(got.BoundaryEvents) != 4 || len(got.BoundarySpans) != 0 {
		t.Fatal("union bridged a material gap or lost the separate outer endpoints", got)
	}
	checkJoinedLineRationalWitnesses(t, sweeps[:], chain.Boundary(), p, d, got)
	t.Logf("proved two material intervals [-1,-1/2] and [3/2,2], %d exact operations", got.ExactOperations)
}

func TestVerifiedJoinedNormalOffsetSweepIntersectsPlacedCurvedRulings(t *testing.T) {
	ctx := context.Background()
	sweeps, joins := placedSweepDistanceCylinder(t)
	pair, chain := joinedSweepMomentFixture(t, sweeps, joins[0], joins[1])
	q := big.NewRat
	world := func(p [3]*big.Rat) [3]*big.Rat { return obliqueSweepTestTransform(p[0], p[1], p[2]) }
	ruling := [3]*big.Rat{q(2, 1), q(0, 1), q(0, 1)}
	cap := [3]*big.Rat{q(-11, 8), q(0, 1), q(2, 13)}
	p, d := placedSweepLine(world, cap, ruling)
	o := NormalOffsetSweepJoinedLineOptions{}
	capLine, err := chain.IntersectRationalLine(ctx, p, d, o)
	checkJoinedLineMaterial(t, capLine, err, [][2]*big.Rat{{q(1, 2), q(2, 1)}})
	if len(capLine.BoundarySpans) != 3 || len(capLine.BoundaryEvents) != 4 ||
		!slices.Equal(joinedLineOwners(capLine.BoundaryEvents[1]), []int{0, 1}) ||
		!slices.Equal(joinedLineOwners(capLine.BoundaryEvents[2]), []int{1, 2}) ||
		capLine.BoundarySpans[1].Span.Start.Station[0].Cmp(q(14, 5)) != 0 ||
		capLine.BoundarySpans[1].Span.Start.Station[2].Cmp(q(2, 13)) != 0 {
		t.Fatal("curved joined cap lost nonbinary middle original station or outer event owners", capLine)
	}
	checkJoinedLineRationalWitnesses(t, sweeps[:], chain.Boundary(), p, d, capLine)
	t.Logf("placed curved cap: three original boundary spans and four outer events, %d exact operations", capLine.ExactOperations)

	inside := [3]*big.Rat{q(-11, 8), q(1, 4), q(1, 20)}
	ip, id := placedSweepLine(world, inside, ruling)
	irrational, err := pair.IntersectRationalLine(ctx, ip, id, o)
	checkJoinedLineMaterial(t, irrational, err, [][2]*big.Rat{{q(1, 2), q(3, 2)}})
	if len(irrational.BoundaryEvents) != 2 || len(irrational.BoundarySpans) != 0 {
		t.Fatal("curved material through the removed wall retained an internal event", irrational)
	}
	for owner, event := range irrational.BoundaryEvents {
		if len(event.Witnesses) != 1 || event.Witnesses[0].Owner != owner || event.Witnesses[0].AlgebraicStation == nil {
			t.Fatal("curved joined endpoint lost its own original algebraic source", event)
		}
		original, err := sweeps[owner].IntersectRationalLine(ctx, ip, id, NormalOffsetSweepLineOptions{})
		if err != nil || !original.Complete || original.Span == nil || original.Span.AlgebraicStation == nil {
			t.Fatal("original algebraic owner proof was unavailable", original, err)
		}
		retained := event.Witnesses[0].AlgebraicStation
		want := original.Span.AlgebraicStation
		if retained.ProfileAxis != 1-owner || retained.ProfileParameter.Lower.Cmp(want.ProfileParameter.Lower) != 0 ||
			retained.SignedDisplacement.Upper.Cmp(want.SignedDisplacement.Upper) != 0 ||
			!slices.Equal(retained.ProfileRoot.Coefficients, want.ProfileRoot.Coefficients) {
			t.Fatal("composed source witness differs from direct original root proof", retained, want)
		}
		if _, err := ImportExactRoot(ctx, retained.ProfileRoot, ExactRootCodecOptions{}); err != nil {
			t.Fatal("retained original witness cannot be reopened", err)
		}
	}
	t.Logf("placed curved algebraic pair: two outer source witnesses, exact interval [1/2,3/2], %d operations", irrational.ExactOperations)

	// The first owner can complete, but an incorrect middle seed
	// leaves the original owner's curved interval unresolved.
	o.ProfileParameters[1] = q(3, 1)
	bad, err := chain.IntersectRationalLine(ctx, p, d, o)
	checkJoinedLineAtomic(t, bad, err, ErrConvergence)
}

func straightLineSweepChain(t *testing.T) ([3]*VerifiedNormalOffsetSweep, *VerifiedNormalOffsetSweepPair, *VerifiedNormalOffsetSweepChain) {
	t.Helper()
	sweeps := [3]*VerifiedNormalOffsetSweep{
		sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}}),
		sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {4, 4}}),
		sweepChainPlanarChart(t, [2][2]int64{{4, 4}, {8, 8}}),
	}
	join := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	pair, chain := joinedSweepMomentFixture(t, sweeps, join, join)
	return sweeps, pair, chain
}

func checkJoinedLineMaterial(t *testing.T, got NormalOffsetSweepJoinedLineIntersection, err error, want [][2]*big.Rat) {
	t.Helper()
	if err != nil || !got.Complete || len(got.Material) != len(want) {
		t.Fatal("expected complete original material line union", got, err, want)
	}
	for i, iv := range got.Material {
		if iv.Lower.Cmp(want[i][0]) != 0 || iv.Upper.Cmp(want[i][1]) != 0 {
			t.Fatal("wrong exact material interval", i, iv, want)
		}
	}
}

func checkJoinedLineAtomic(t *testing.T, got NormalOffsetSweepJoinedLineIntersection, err, expected error) {
	t.Helper()
	if !errors.Is(err, expected) || got.Complete || got.Material != nil || got.BoundaryEvents != nil || got.BoundarySpans != nil {
		t.Fatal("partial original proof published joined material or boundary data", got, err, expected)
	}
}

func joinedLineOwners(event NormalOffsetSweepJoinedLineEvent) (owners []int) {
	for _, w := range event.Witnesses {
		owners = append(owners, w.Owner)
	}
	return owners
}

func checkJoinedLineRationalWitnesses(t *testing.T, sweeps []*VerifiedNormalOffsetSweep, retained []VerifiedNormalOffsetSweepBoundaryFace,
	origin, direction [3]*big.Rat, got NormalOffsetSweepJoinedLineIntersection) {
	t.Helper()
	ctx := context.Background()
	for _, event := range got.BoundaryEvents {
		for _, w := range event.Witnesses {
			if w.Owner < 0 || w.Owner >= len(sweeps) || len(w.Faces) == 0 || w.AlgebraicStation != nil {
				t.Fatal("expected an original rational station for every retained witness", w)
			}
			for _, face := range w.Faces {
				if !slices.ContainsFunc(retained, func(f VerifiedNormalOffsetSweepBoundaryFace) bool {
					return f.Owner == w.Owner && f.Side == face
				}) {
					t.Fatal("boundary witness on an omitted original wall", w)
				}
			}
			point, err := sweeps[w.Owner].EvaluateRational(ctx, w.Station[0], w.Station[1], w.Station[2],
				SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for i := range origin {
				model := new(big.Rat).Add(origin[i], new(big.Rat).Mul(event.Parameter, direction[i]))
				if model.Cmp(event.Point[i]) != 0 || point.Lower[i].Cmp(model) != 0 || point.Upper[i].Cmp(model) != 0 {
					t.Fatal("retained original owner witness did not exactly replay at grouped model point", event, point)
				}
			}
		}
	}
}
