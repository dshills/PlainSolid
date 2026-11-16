package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSweepBoundaryDistanceExactRationalSlab(t *testing.T) {
	ctx := context.Background()
	sweep, world := sweepBoundarySlabFixture(t)
	newPoint := func(x, y, z *big.Rat) [3]*big.Rat { return [3]*big.Rat{x, y, z} }
	q := big.NewRat
	for _, tc := range []struct {
		name                 string
		query, station, foot [3]*big.Rat
		distance             *big.Rat
		side                 NormalOffsetSweepSide
	}{
		{"upper face", newPoint(q(2, 3), q(7, 5), q(1, 1)), newPoint(q(-2, 3), q(73, 15), q(2, 7)),
			newPoint(q(2, 3), q(7, 5), q(2, 7)), q(25, 49), NormalOffsetSweepUpper},
		{"wall at nonbinary original offset", newPoint(q(5, 2), q(7, 5), q(1, 13)), newPoint(q(2, 1), q(73, 15), q(1, 13)),
			newPoint(q(2, 1), q(7, 5), q(1, 13)), q(1, 4), NormalOffsetSweepUMax},
		{"upper edge", newPoint(q(5, 2), q(7, 5), q(9, 14)), newPoint(q(2, 1), q(73, 15), q(2, 7)),
			newPoint(q(2, 1), q(7, 5), q(2, 7)), q(37, 98), NormalOffsetSweepUMax},
		{"lower corner", newPoint(q(-1, 2), q(-1, 2), q(-2, 3)), newPoint(q(-2, 1), q(3, 1), q(-1, 3)),
			newPoint(q(0, 1), q(0, 1), q(-1, 3)), q(11, 18), NormalOffsetSweepUMin},
		{"interior volume", newPoint(q(1, 1), q(3, 2), q(0, 1)), newPoint(q(0, 1), q(5, 1), q(2, 7)),
			newPoint(q(1, 1), q(3, 2), q(2, 7)), q(4, 49), NormalOffsetSweepUpper},
		{"true boundary point", newPoint(q(0, 1), q(3, 2), q(1, 13)), newPoint(q(-2, 1), q(5, 1), q(1, 13)),
			newPoint(q(0, 1), q(3, 2), q(1, 13)), q(0, 1), NormalOffsetSweepUMin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := NormalOffsetSweepDistanceOptions{SquaredTolerance: new(big.Rat), MaxCells: 6}
			got, err := sweep.BoundPointBoundaryDistanceRational(ctx, world(tc.query), options)
			if err != nil || !got.Complete || got.Witness == nil || got.Witness.Side != tc.side ||
				got.Squared.Lower.Cmp(tc.distance) != 0 || got.Squared.Upper.Cmp(tc.distance) != 0 {
				t.Fatal("expected exact rational global boundary minimum", got, tc.distance, err)
			}
			foot := world(tc.foot)
			for axis, want := range tc.station {
				x := got.Witness.Position[axis]
				if got.Witness.Parameters[axis].Cmp(want) != 0 || x.Lower.Cmp(foot[axis]) != 0 || x.Upper.Cmp(foot[axis]) != 0 {
					t.Fatal("nearest station or physical witness lost original coordinates", axis, got.Witness, want, foot)
				}
			}
			checkSweepBoundaryDistanceWitness(t, sweep, world(tc.query), got)
			if tc.distance.Sign() == 0 && got.VisitedCells != 1 {
				t.Fatal("exact zero witness is a global proof with any remaining faces", got.VisitedCells)
			}
			t.Logf("face %d, exact squared distance %s, original station %v, visited %d", tc.side, tc.distance, tc.station, got.VisitedCells)
		})
	}
	query := world(newPoint(q(5, 2), q(7, 5), q(9, 14)))
	short, err := sweep.BoundPointBoundaryDistanceRational(ctx, query, NormalOffsetSweepDistanceOptions{MaxCells: 1})
	if !errors.Is(err, ErrConvergence) || short.Complete || short.VisitedCells != 1 ||
		short.Squared.Lower.Sign() != 0 || short.Squared.Upper.Cmp(q(37, 98)) < 0 {
		t.Fatal("early face budget lost a valid global enclosure", short, err)
	}
	checkSweepBoundaryDistanceWitness(t, sweep, query, short)
	work, err := sweep.BoundPointBoundaryDistanceRational(ctx, query,
		NormalOffsetSweepDistanceOptions{Patch: SurfacePatchOptions{MaxOperations: 1}})
	if !errors.Is(err, ErrConvergence) || work.Complete || work.Squared.Lower.Sign() != 0 || work.Squared.Upper != nil || work.Witness != nil {
		t.Fatal("short exact budget invented an unevaluated boundary station", work, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	empty, err := sweep.BoundPointBoundaryDistanceRational(cancelled, query, NormalOffsetSweepDistanceOptions{})
	if !errors.Is(err, context.Canceled) || empty.Complete || empty.Witness != nil {
		t.Fatal("cancelled exact search published a claim", empty, err)
	}
}

func TestVerifiedNormalOffsetSweepBoundaryDistanceRotatedNegativeJacobian(t *testing.T) {
	ctx := context.Background()
	p, err := normalOffsetParabolicCylinder(t, true).Trim(ParameterRectangle{.484375, .515625, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(7, 10), big.NewRat(4, 5),
		NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true, true}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete || sweep.Orientation() != -1 {
		t.Fatal("the original placed, beyond-focal strip must be a verified sweep", proof, err)
	}
	point := [3]*big.Rat{big.NewRat(-1, 2), new(big.Rat), big.NewRat(9, 10)}
	opts := NormalOffsetSweepDistanceOptions{}
	// The upright coordinate on this strip is x²+t/sqrt(1+4x²).
	// For |x|<=1/32 and t>=7/10 it decreases away from x=0.
	// The unique closest source station is the upper-layer midpoint.
	goal := big.NewRat(1, 100)
	limited := opts
	limited.MaxCells = 6
	coarse, err := sweep.BoundPointBoundaryDistanceRational(ctx, point, limited)
	if !errors.Is(err, ErrConvergence) || coarse.Complete || coarse.VisitedCells != 6 ||
		coarse.Squared.Lower.Sign() <= 0 || coarse.Squared.Lower.Cmp(goal) > 0 || coarse.Squared.Upper.Cmp(goal) != 0 {
		t.Fatal("short curved search lost its proved positive directed distance bounds", coarse, err)
	}
	checkSweepBoundaryDistanceWitness(t, sweep, point, coarse)
	work := opts
	work.Patch.MaxOperations = coarse.ExactOperations + 10
	partial, err := sweep.BoundPointBoundaryDistanceRational(ctx, point, work)
	if !errors.Is(err, ErrConvergence) || partial.Complete || partial.Squared.Lower.Cmp(coarse.Squared.Lower) < 0 ||
		partial.Squared.Lower.Cmp(goal) > 0 || partial.Squared.Upper.Cmp(goal) != 0 {
		t.Fatal("interrupted exact subdivision lost the prior full boundary cover", partial, err)
	}
	checkSweepBoundaryDistanceWitness(t, sweep, point, partial)
	got, err := sweep.BoundPointBoundaryDistanceRational(ctx, point, opts)
	if err != nil || !got.Complete || got.Squared.Lower.Cmp(goal) > 0 || got.Squared.Upper.Cmp(goal) != 0 ||
		new(big.Rat).Sub(got.Squared.Upper, got.Squared.Lower).Cmp(big.NewRat(1, 100_000_000)) > 0 ||
		got.Witness.Side != NormalOffsetSweepUpper ||
		got.Witness.Parameters[0].Cmp(big.NewRat(1, 2)) != 0 || got.Witness.Parameters[1].Cmp(big.NewRat(1, 2)) != 0 ||
		got.Witness.Parameters[2].Cmp(big.NewRat(4, 5)) != 0 {
		t.Fatal("curved cap distance failed its independent exact minimum", got, err)
	}
	checkSweepBoundaryDistanceWitness(t, sweep, point, got)
	lower, _ := got.Squared.Lower.Float64()
	t.Logf("rotated negative-Jacobian cap squared distance [%.13g,0.01], %d cells, %d exact operations",
		lower, got.VisitedCells, got.ExactOperations)

	// The rotated free cylinder axis is world -X. A query outside
	// that range lies exactly 1/5 from the VMin wall at its midpoint.
	wallQuery := [3]*big.Rat{big.NewRat(1, 5), new(big.Rat), big.NewRat(3, 4)}
	wall, err := sweep.BoundPointBoundaryDistanceRational(ctx, wallQuery,
		NormalOffsetSweepDistanceOptions{SquaredTolerance: new(big.Rat)})
	if err != nil || !wall.Complete || wall.Witness.Side != NormalOffsetSweepVMin ||
		wall.Squared.Lower.Cmp(big.NewRat(1, 25)) != 0 || wall.Squared.Upper.Cmp(big.NewRat(1, 25)) != 0 {
		t.Fatal("curved wall did not prove its exact original-axis minimum", wall, err)
	}
	checkSweepBoundaryDistanceWitness(t, sweep, wallQuery, wall)
	t.Logf("rotated cylindrical wall squared distance exactly 1/25, %d cells, %d exact operations", wall.VisitedCells, wall.ExactOperations)

	on := point
	on[2] = big.NewRat(4, 5)
	zero, err := sweep.BoundPointBoundaryDistanceRational(ctx, on,
		NormalOffsetSweepDistanceOptions{SquaredTolerance: new(big.Rat), MaxCells: 6})
	if err != nil || !zero.Complete || zero.Squared.Upper.Sign() != 0 || zero.Squared.Lower.Sign() != 0 {
		t.Fatal("exact curved boundary hit should finish with a zero witness", zero, err)
	}
	checkSweepBoundaryDistanceWitness(t, sweep, on, zero)
}

func checkSweepBoundaryDistanceWitness(t *testing.T, sweep *VerifiedNormalOffsetSweep, query [3]*big.Rat,
	result NormalOffsetSweepDistance) {
	t.Helper()
	if result.Witness == nil {
		t.Fatal("a finite upper distance needs an original station")
	}
	w := result.Witness
	face, err := sweep.Boundary(w.Side)
	if err != nil {
		t.Fatal(err)
	}
	axis := int(w.Side) / 2
	free := [3][2]int{{1, 2}, {0, 2}, {0, 1}}[axis]
	scope := face.ParameterDomain()
	for j, i := range free {
		if w.Parameters[i].Cmp(scope[j].Lower) < 0 || w.Parameters[i].Cmp(scope[j].Upper) > 0 {
			t.Fatal("upper witness left the original analytic face", w)
		}
	}
	uv, tSpan := sweep.Domain()
	fixed := normalSweepParameterIntervals(uv, exactInterval{tSpan.Lower, tSpan.Upper})[axis]
	at := fixed.lo
	if w.Side%2 == 1 {
		at = fixed.hi
	}
	if w.Parameters[axis].Cmp(at) != 0 {
		t.Fatal("upper witness was not located on its stated original face", w)
	}
	p, err := face.EvaluateRational(context.Background(), w.Parameters[free[0]], w.Parameters[free[1]],
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	squared := new(big.Rat)
	for i, e := range w.Position {
		if e.Lower.Cmp(p.Lower[i]) != 0 || e.Upper.Cmp(p.Upper[i]) != 0 {
			t.Fatal("upper witness cannot be replayed at the exact original station", i, e, p)
		}
		lo := new(big.Rat).Sub(e.Lower, query[i])
		hi := new(big.Rat).Sub(e.Upper, query[i])
		lo.Mul(lo, lo)
		hi.Mul(hi, hi)
		if hi.Cmp(lo) > 0 {
			lo = hi
		}
		squared.Add(squared, lo)
	}
	if squared.Cmp(result.Squared.Upper) != 0 {
		t.Fatal("upper squared distance is not supported by the witness enclosure", squared, result.Squared)
	}
}

func sweepBoundarySlabFixture(t *testing.T) (*VerifiedNormalOffsetSweep, func([3]*big.Rat) [3]*big.Rat) {
	t.Helper()
	ctx := context.Background()
	world := func(local [3]*big.Rat) (p [3]*big.Rat) {
		x, y, z := local[0], local[1], local[2]
		p[0] = new(big.Rat).Add(big.NewRat(5, 4), new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 5), x),
			new(big.Rat).Mul(big.NewRat(3, 5), y)))
		p[1] = new(big.Rat).Add(big.NewRat(-7, 5), new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(3, 5), x),
			new(big.Rat).Mul(big.NewRat(4, 5), y)))
		p[2] = new(big.Rat).Add(big.NewRat(11, 9), z)
		return p
	}
	recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 1, Domain: ParameterRectangle{-2, 2, 3, 7}}
	for _, x := range []int64{0, 2} {
		for _, y := range []int64{0, 3} {
			p := world([3]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), new(big.Rat)})
			recipe.Controls = append(recipe.Controls, [4]string{p[0].RatString(), p[1].RatString(), p[2].RatString(), "1"})
		}
	}
	p, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(-1, 3), big.NewRat(2, 7),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("independently placed slab must verify", proof, err)
	}
	return sweep, world
}

func TestVerifiedNormalOffsetSweepBoundaryDistanceRationalSphere(t *testing.T) {
	ctx := context.Background()
	// Exact stereographic source of the unit sphere in the square
	// [-1/32,1/32]^2. Its positive homogeneous weights vary.
	a := big.NewRat(1, 32)
	a2 := new(big.Rat).Mul(a, a)
	linear := [3]*big.Rat{new(big.Rat).Neg(a), new(big.Rat), a}
	square := [3]*big.Rat{a2, new(big.Rat).Neg(a2), a2}
	var poles [][4]*big.Rat
	for i, x := range linear {
		for j, y := range linear {
			r2 := new(big.Rat).Add(square[i], square[j])
			poles = append(poles, [4]*big.Rat{new(big.Rat).Add(x, x), new(big.Rat).Add(y, y),
				new(big.Rat).Sub(big.NewRat(1, 1), r2), new(big.Rat).Add(big.NewRat(1, 1), r2)})
		}
	}
	source, err := NewExactHomogeneousBezierSurface(ctx, 2, 2, poles, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, big.NewRat(-1, 11), big.NewRat(2, 13),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("exact weighted stereographic sweep must verify independently", proof, err)
	}
	// The full swept volume has radius at most 15/13. The upper
	// source face includes the north pole and attains the triangle
	// inequality bound from the original point (0,0,2).
	goal := big.NewRat(121, 169)
	query := [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(2, 1)}
	o := NormalOffsetSweepDistanceOptions{SquaredTolerance: new(big.Rat)}
	got, err := sweep.BoundPointBoundaryDistanceRational(ctx, query, o)
	if err != nil || !got.Complete || got.Squared.Lower.Cmp(goal) > 0 || got.Squared.Upper.Cmp(goal) != 0 ||
		new(big.Rat).Sub(got.Squared.Upper, got.Squared.Lower).Cmp(o.SquaredTolerance) > 0 ||
		got.Witness.Side != NormalOffsetSweepUpper || got.Witness.Parameters[2].Cmp(big.NewRat(2, 13)) != 0 {
		t.Fatal("weighted sphere missed the independently exact radial minimum", got, err)
	}
	checkSweepBoundaryDistanceWitness(t, sweep, query, got)
	lower, _ := got.Squared.Lower.Float64()
	t.Logf("rational source spherical distance enclosure starts %.13g, exact upper %s, %d cells, %d exact operations",
		lower, goal, got.VisitedCells, got.ExactOperations)
}
