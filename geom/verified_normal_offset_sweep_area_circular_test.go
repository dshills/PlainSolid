package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestVerifiedNormalOffsetSweepBoundaryAreaCircularSector(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	lower, upper, width, radius := q(-1, 11), q(2, 13), q(5, 4), q(2, 1)
	thickness := new(big.Rat).Sub(upper, lower)
	angle := referenceSweepSectorAngle()
	tolerance := q(1, 1_000_000_000_000)
	for _, swap := range []bool{false, true} {
		source := sweepAreaCircularSource(t, swap, false)
		r, err := source.BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{AbsoluteError: tolerance})
		if err != nil || !r.Complete || len(r.Faces) != 6 || r.VisitedCells != 6 ||
			new(big.Rat).Sub(r.Bounds.Upper, r.Bounds.Lower).Cmp(tolerance) > 0 {
			t.Fatal("the independently authored placed sector needs a directed original-face cover", swap, r, err)
		}
		sign := q(1, 1)
		if swap {
			sign.Neg(sign)
		}
		bot := new(big.Rat).Add(radius, new(big.Rat).Mul(sign, lower))
		top := new(big.Rat).Add(radius, new(big.Rat).Mul(sign, upper))
		annular := new(big.Rat).Mul(thickness, new(big.Rat).Quo(new(big.Rat).Add(bot, top), q(2, 1)))
		radial := new(big.Rat).Mul(width, thickness)
		norm := [6]*big.Rat{annular, annular, radial, radial, new(big.Rat).Mul(width, bot), new(big.Rat).Mul(width, top)}
		exactWalls := [6]bool{false, false, true, true}
		if swap {
			norm[0], norm[1], norm[2], norm[3] = radial, radial, annular, annular
			exactWalls = [6]bool{true, true}
		}
		wantTotal := exactInterval{new(big.Rat), new(big.Rat)}
		for i, face := range r.Faces {
			if face.Side != NormalOffsetSweepSide(i) {
				t.Fatal("physical area lost original side identity", i, face)
			}
			exact := norm[i]
			expect := exactInterval{exact, exact}
			if exactWalls[i] {
				if face.SquaredArea == nil || face.SquaredArea.Cmp(new(big.Rat).Mul(exact, exact)) != 0 ||
					face.Bounds.Lower.Cmp(exact) != 0 || face.Bounds.Upper.Cmp(exact) != 0 {
					t.Fatal("planar radial wall requires an exact rational square and area", i, face, exact)
				}
			} else {
				expect = exactInterval{new(big.Rat).Mul(exact, angle.lo), new(big.Rat).Mul(exact, angle.hi)}
				if face.SquaredArea != nil || face.Bounds.Lower.Cmp(expect.lo) > 0 || face.Bounds.Upper.Cmp(expect.hi) < 0 {
					t.Fatal("original curved face missed the independently integrated angle", i, face, expect)
				}
			}
			wantTotal.lo.Add(wantTotal.lo, expect.lo)
			wantTotal.hi.Add(wantTotal.hi, expect.hi)
		}
		if r.Bounds.Lower.Cmp(wantTotal.lo) > 0 || r.Bounds.Upper.Cmp(wantTotal.hi) < 0 {
			t.Fatal("directed total missed the independent sum of original physical areas", r.Bounds, wantTotal)
		}
		t.Logf("original normal sign %s, physical face cover total [%s,%s], exact operations %d",
			sign, r.Bounds.Lower.FloatString(18), r.Bounds.Upper.FloatString(18), r.ExactOperations)
	}
	source := sweepAreaCircularSource(t, false, false)
	partial, err := source.BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{MaxCells: 5})
	if !errors.Is(err, ErrConvergence) || partial.Complete || len(partial.Faces) != 5 || partial.Bounds.Upper != nil {
		t.Fatal("a missing original face cannot be counted in the total", partial, err)
	}
	coarse, err := source.BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{AbsoluteError: new(big.Rat)})
	if !errors.Is(err, ErrConvergence) || coarse.Complete || len(coarse.Faces) != 6 ||
		coarse.Bounds.Lower == nil || coarse.Bounds.Upper == nil {
		t.Fatal("zero-width request needs a valid enclosure and explicit refusal", coarse, err)
	}
	noncircular := sweepAreaCircularSource(t, false, true)
	refused, err := noncircular.BoundaryAreaRational(ctx, NormalOffsetSweepAreaOptions{})
	if !errors.Is(err, ErrDomain) || refused.Complete || len(refused.Faces) != 0 || refused.Bounds.Lower != nil {
		t.Fatal("changed original profile pole must not masquerade as a circular sector", refused, err)
	}
}

// The fixture uses y=4s/(1+s²) and z=2(1-s²)/(1+s²), with
// s varying from 0 to 1/5. Swapping U,V reverses the signed unit normal.
func sweepAreaCircularSource(t *testing.T, swap, alter bool) *VerifiedNormalOffsetSweep {
	t.Helper()
	ctx := context.Background()
	q := big.NewRat
	recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 2, Domain: ParameterRectangle{2, 3, -1, 0}}
	projection := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
	if swap {
		recipe.DegreeU, recipe.DegreeV = 2, 1
		recipe.Domain = ParameterRectangle{-1, 0, 2, 3}
		projection = NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{false, false, true}}
	}
	for u := 0; u <= recipe.DegreeU; u++ {
		for v := 0; v <= recipe.DegreeV; v++ {
			axial, profile := u, v
			if swap {
				axial, profile = v, u
			}
			w := [3]*big.Rat{q(1, 1), q(1, 1), q(26, 25)}[profile]
			y := new(big.Rat).Quo(q(int64(2*profile), 5), w)
			z := [3]*big.Rat{q(2, 1), q(2, 1), q(48, 25)}[profile]
			if alter && profile == 1 {
				z.Add(z, q(1, 64))
			}
			z.Quo(z, w)
			world := obliqueSweepTestTransform(q(int64(5*axial), 4), y, z)
			recipe.Controls = append(recipe.Controls, [4]string{
				world[0].Mul(world[0], w).RatString(),
				world[1].Mul(world[1], w).RatString(),
				world[2].Mul(world[2], w).RatString(), w.RatString(),
			})
		}
	}
	patch, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	source, proof, err := NewVerifiedNormalOffsetSweep(ctx, patch, q(-1, 11), q(2, 13), projection, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("the independently authored circular or altered source must be a verified sweep", swap, alter, proof, err)
	}
	return source
}

// The independent sector angle is twice atan(1/5), enclosed by twenty
// alternating terms and the next bound. The production proof uses atan(5/12)
// recovered from the physical endpoint vectors.
func referenceSweepSectorAngle() exactInterval {
	sum := new(big.Rat)
	denominator := big.NewInt(5)
	for i := int64(0); i < 20; i++ {
		numerator := big.NewInt(2)
		if i%2 == 1 {
			numerator.Neg(numerator)
		}
		term := new(big.Rat).SetFrac(numerator, new(big.Int).Mul(denominator, big.NewInt(2*i+1)))
		sum.Add(sum, term)
		denominator.Mul(denominator, big.NewInt(25))
	}
	next := new(big.Rat).SetFrac(big.NewInt(2), new(big.Int).Mul(denominator, big.NewInt(41)))
	return exactInterval{sum, new(big.Rat).Add(sum, next)}
}
