package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedJoinedNormalOffsetSweepBoundaryAreaExactSlabs(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	charts := [3]*VerifiedNormalOffsetSweep{
		sweepChainPlanarChart(t, [2][2]int64{{-4, -4}, {0, 0}}),
		sweepChainPlanarChart(t, [2][2]int64{{0, 0}, {4, 4}}),
		sweepChainPlanarChart(t, [2][2]int64{{4, 4}, {8, 8}}),
	}
	join := VerifiedNormalOffsetSweepJoin{Edges: [2]SurfaceEdge{SurfaceEdgeUMax, SurfaceEdgeUMin}, Axis: 0, AInteriorSign: -1}
	pair, chain := joinedSweepMomentFixture(t, charts, join, join)
	single, err := charts[0].BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{AbsoluteError: q(0, 1)})
	if err != nil || single.Bounds.Lower.Cmp(q(14, 5)) != 0 {
		t.Fatal("independent original slab boundary area must include both full U walls", single, err)
	}
	for _, test := range []struct {
		name     string
		target   *big.Rat
		boundary []VerifiedNormalOffsetSweepBoundaryFace
		measure  func(context.Context, NormalOffsetSweepAreaOptions) (NormalOffsetSweepJoinedBoundaryArea, error)
	}{
		{"pair", q(26, 5), pair.Boundary(), pair.BoundaryAreaRational},
		{"chain", q(38, 5), chain.Boundary(), chain.BoundaryAreaRational},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := NormalOffsetSweepAreaOptions{AbsoluteError: q(0, 1), MaxCells: len(test.boundary)}
			r, err := test.measure(ctx, options)
			if err != nil || !r.Complete || r.Bounds.Lower.Cmp(test.target) != 0 || r.Bounds.Upper.Cmp(test.target) != 0 {
				t.Fatal("external box formula disagrees with joined retained area", r, test.target, err)
			}
			checkJoinedSweepAreaOwners(t, r, test.boundary)
			for _, f := range r.Faces {
				expected := q(1, 5)
				if f.Side >= NormalOffsetSweepLower {
					expected = q(1, 1)
				}
				if f.Bounds.Lower.Cmp(expected) != 0 || f.Bounds.Upper.Cmp(expected) != 0 ||
					f.SquaredArea.Cmp(new(big.Rat).Mul(expected, expected)) != 0 {
					t.Fatal("an original face lost its independent exact area", f, expected)
				}
			}
			t.Logf("retained %d original faces, exact area %s, %d exact operations", r.VisitedCells, r.Bounds.Lower, r.ExactOperations)
			options.MaxCells--
			short, err := test.measure(ctx, options)
			if !errors.Is(err, ErrConvergence) || short.Complete || short.VisitedCells != options.MaxCells ||
				len(short.Faces) != options.MaxCells || short.Bounds.Lower != nil || short.Bounds.Upper != nil {
				t.Fatal("incomplete cross-owner area cover published a total", short, err)
			}
		})
	}
}

func TestVerifiedJoinedNormalOffsetSweepBoundaryAreaPlacedCircularChain(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	sweeps, widths := placedCircularSweepAreaChain(t)
	first, second := sweepChainPlacedJoins()
	first.Covector = &[3]*big.Rat{q(12, 7), q(5, 7)}
	second.Covector = &[3]*big.Rat{q(-18, 7), q(-15, 14)}
	_, chain := joinedSweepMomentFixture(t, sweeps, first, second)
	o := NormalOffsetSweepAreaOptions{AbsoluteError: q(1, 1_000_000_000_000), MaxCells: 14}
	got, err := chain.BoundaryAreaRational(ctx, o)
	if err != nil || !got.Complete ||
		new(big.Rat).Sub(got.Bounds.Upper, got.Bounds.Lower).Cmp(o.AbsoluteError) > 0 {
		t.Fatal("placed circular chain area should be proved on only the fourteen outer faces", got, err)
	}
	checkJoinedSweepAreaOwners(t, got, chain.Boundary())
	angle := referenceSweepSectorAngle()
	shift := q(35, 143)
	lowerRadius, upperRadius := q(21, 11), q(28, 13)
	radiiSum := new(big.Rat).Add(lowerRadius, upperRadius)
	endFactor := new(big.Rat).Mul(shift, new(big.Rat).Quo(radiiSum, q(2, 1)))
	for _, f := range got.Faces {
		if f.Side >= NormalOffsetSweepLower {
			radius := lowerRadius
			if f.Side == NormalOffsetSweepUpper {
				radius = upperRadius
			}
			factor := new(big.Rat).Mul(radius, widths[f.Owner])
			checkCircularJoinedFaceEnvelope(t, f, angle, factor)
			continue
		}
		rulingAxis := 0
		if f.Owner == 1 {
			rulingAxis = 1
		}
		if int(f.Side)/2 == rulingAxis {
			if f.Owner == 1 || (f.Owner == 0 && f.Side != NormalOffsetSweepUMin) ||
				(f.Owner == 2 && f.Side != NormalOffsetSweepUMax) {
				t.Fatal("original joined annular wall was integrated", f)
			}
			checkCircularJoinedFaceEnvelope(t, f, angle, endFactor)
			continue
		}
		rectangle := new(big.Rat).Mul(shift, widths[f.Owner])
		if f.Bounds.Lower.Cmp(rectangle) != 0 || f.Bounds.Upper.Cmp(rectangle) != 0 ||
			f.SquaredArea == nil || f.SquaredArea.Cmp(new(big.Rat).Mul(rectangle, rectangle)) != 0 {
			t.Fatal("the original radial side should have its exact rectangle area", f, rectangle)
		}
	}
	// The independent continuous geometry is one sector of total
	// length 7/2 with only two physical annular end areas.
	length := q(7, 2)
	radial := new(big.Rat).Mul(q(2, 1), new(big.Rat).Mul(length, shift))
	circularFactor := new(big.Rat).Mul(radiiSum, new(big.Rat).Add(length, shift))
	for i, bound := range []*big.Rat{angle.lo, angle.hi} {
		exact := new(big.Rat).Add(radial, new(big.Rat).Mul(circularFactor, bound))
		if (i == 0 && got.Bounds.Lower.Cmp(exact) > 0) || (i == 1 && got.Bounds.Upper.Cmp(exact) < 0) {
			t.Fatal("joined directed area escaped the independently integrated outer boundary", got.Bounds, exact)
		}
	}
	t.Logf("rotated circular retained area [%s,%s], %d original outer faces, %d exact operations",
		got.Bounds.Lower.FloatString(18), got.Bounds.Upper.FloatString(18), got.VisitedCells, got.ExactOperations)
	zero := o
	zero.AbsoluteError = q(0, 1)
	coarse, err := chain.BoundaryAreaRational(ctx, zero)
	if !errors.Is(err, ErrConvergence) || coarse.Complete || len(coarse.Faces) != 14 ||
		coarse.Bounds.Lower == nil || coarse.Bounds.Upper == nil ||
		coarse.Bounds.Lower.Cmp(got.Bounds.Lower) > 0 || coarse.Bounds.Upper.Cmp(got.Bounds.Upper) < 0 {
		t.Fatal("coarse original owner proofs need a complete directed outer sum", coarse, err)
	}
	short := o
	short.Patch.MaxOperations = got.ExactOperations / 2
	limited, err := chain.BoundaryAreaRational(ctx, short)
	if !errors.Is(err, ErrConvergence) || limited.Complete || len(limited.Faces) == 0 || len(limited.Faces) >= 14 ||
		limited.Bounds.Lower != nil {
		t.Fatal("shared exact work budget published an incomplete total or discarded proved faces", limited, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	stopped, err := chain.BoundaryAreaRational(cancelled, o)
	if !errors.Is(err, context.Canceled) || stopped.Complete || len(stopped.Faces) != 0 || stopped.Bounds.Lower != nil {
		t.Fatal("cancelled joined measure published a result", stopped, err)
	}
}

func checkJoinedSweepAreaOwners(t *testing.T, r NormalOffsetSweepJoinedBoundaryArea, boundary []VerifiedNormalOffsetSweepBoundaryFace) {
	t.Helper()
	if r.VisitedCells != len(boundary) || len(r.Faces) != len(boundary) {
		t.Fatal("only original outer faces should count toward visited work", r.VisitedCells, len(r.Faces), len(boundary))
	}
	for i, f := range r.Faces {
		if f.Owner != boundary[i].Owner || f.Side != boundary[i].Side {
			t.Fatal("joined area order drifted from original owner and face identities", i, f, boundary[i])
		}
	}
}

func checkCircularJoinedFaceEnvelope(t *testing.T, f NormalOffsetSweepJoinedFaceArea, angle exactInterval, factor *big.Rat) {
	t.Helper()
	if f.SquaredArea != nil || f.Bounds.Lower.Cmp(new(big.Rat).Mul(angle.lo, factor)) > 0 ||
		f.Bounds.Upper.Cmp(new(big.Rat).Mul(angle.hi, factor)) < 0 {
		t.Fatal("original circular face area does not contain the independent result", f, factor)
	}
}

func placedCircularSweepAreaChain(t *testing.T) (sweeps [3]*VerifiedNormalOffsetSweep, widths [3]*big.Rat) {
	t.Helper()
	ctx := context.Background()
	q := big.NewRat
	widths = [3]*big.Rat{q(5, 4), q(3, 4), q(3, 2)}
	starts := [3]*big.Rat{q(0, 1), q(5, 4), q(2, 1)}
	for owner := range sweeps {
		recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 2, Domain: ParameterRectangle{2, 3, -1, 0}}
		projection := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
		if owner == 1 {
			recipe.DegreeU, recipe.DegreeV = 2, 1
			recipe.Domain = ParameterRectangle{2, 4, -1, 0}
			projection = NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true}}
		}
		for u := 0; u <= recipe.DegreeU; u++ {
			for v := 0; v <= recipe.DegreeV; v++ {
				axial, profile := u, v
				factor := q(1, 1)
				if owner == 1 {
					axial, profile = v, 2-u
					factor = [3]*big.Rat{q(1, 1), q(3, 2), q(9, 4)}[u]
				}
				w := [3]*big.Rat{q(1, 1), q(1, 1), q(26, 25)}[profile]
				x := new(big.Rat).Add(starts[owner], new(big.Rat).Mul(widths[owner], q(int64(axial), 1)))
				y := new(big.Rat).Quo(q(int64(2*profile), 5), w)
				z := new(big.Rat).Quo([3]*big.Rat{q(2, 1), q(2, 1), q(48, 25)}[profile], w)
				world := [3]*big.Rat{q(1, 8), q(-1, 4), q(5, 16)}
				world[0].Add(world[0], new(big.Rat).Mul(x, q(12, 13)))
				world[0].Sub(world[0], new(big.Rat).Mul(y, q(5, 13)))
				world[1].Add(world[1], new(big.Rat).Mul(x, q(5, 13)))
				world[1].Add(world[1], new(big.Rat).Mul(y, q(12, 13)))
				world[2].Add(world[2], z)
				w.Mul(w, factor)
				recipe.Controls = append(recipe.Controls, [4]string{
					world[0].Mul(world[0], w).RatString(),
					world[1].Mul(world[1], w).RatString(),
					world[2].Mul(world[2], w).RatString(), w.RatString(),
				})
			}
		}
		source, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, q(-1, 11), q(2, 13), projection, NormalOffsetCoverageOptions{})
		if err != nil || !proof.Complete {
			t.Fatal("original placed circle sweep must verify independently", owner,
				len(proof.Unresolved), proof.ExactOperations, err)
		}
		sweeps[owner] = sweep
	}
	return sweeps, widths
}
