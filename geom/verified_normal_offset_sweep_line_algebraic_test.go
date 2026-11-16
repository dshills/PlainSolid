package geom

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"
)

func checkIndependentAlgebraicRuling(t *testing.T, s *VerifiedNormalOffsetSweep, world func([3]*big.Rat) [3]*big.Rat,
	ruling [3]*big.Rat) {
	t.Helper()
	ctx := context.Background()
	q := big.NewRat
	for _, c := range []struct {
		z, sign int64
		cubic   [4]int64
	}{
		{1, 1, [4]int64{4528, -3816, -3324, -4030}},
		{-1, -1, [4]int64{4720, -4104, -3756, -4150}},
	} {
		p, d := placedSweepLine(world, [3]*big.Rat{q(17, 8), q(1, 5), q(c.z, 20)}, ruling)
		got, err := s.IntersectRationalLine(ctx, p, d, NormalOffsetSweepLineOptions{})
		if err != nil || !got.Complete || got.Span == nil || got.Span.AlgebraicStation == nil {
			t.Fatal("the exact unseeded profile root should preserve its algebraic source witness", got, err)
		}
		span, station := got.Span, got.Span.AlgebraicStation
		if span.Start.Parameter.Cmp(q(1, 4)) != 0 || span.End.Parameter.Cmp(q(3, 4)) != 0 ||
			station.ProfileAxis != 1 || station.OriginalProfile.Lower.Cmp(q(-1, 2)) != 0 ||
			station.OriginalProfile.Upper.Cmp(q(1, 2)) != 0 || span.Start.Station[1] != nil || span.End.Station[1] != nil ||
			span.Start.Station[2] != nil || span.End.Station[2] != nil || len(span.BoundaryFaces) != 0 ||
			!slices.Equal(span.Start.Faces, []NormalOffsetSweepSide{NormalOffsetSweepUMax}) ||
			!slices.Equal(span.End.Faces, []NormalOffsetSweepSide{NormalOffsetSweepUMin}) {
			t.Fatal("irrational profile or displacement was rounded into a rational station", span)
		}
		root, err := ImportExactRoot(ctx, station.ProfileRoot, ExactRootCodecOptions{})
		if err != nil {
			t.Fatal("algebraic witness recipe could not reprove its isolated root", err)
		}
		shift := station.SignedDisplacement
		if shift.Lower.Sign() != int(c.sign) || shift.Upper.Sign() != int(c.sign) ||
			new(big.Rat).Sub(shift.Upper, shift.Lower).Cmp(q(1, 1_000_000_000_000)) > 0 {
			t.Fatal("original signed algebraic displacement lost its valid directed enclosure", shift)
		}
		// In local coordinates the exact normal-foot equation is
		// 20-(100-3*zSign)*y-18*y³=0. The source Mobius clock
		// gives y=(5s-2)/(2s+4) in normalized profile parameter s.
		// Each cubic above is the independent integer numerator.
		var poly []*big.Rat
		for _, coefficient := range c.cubic {
			poly = append(poly, q(coefficient, 1))
		}
		contact, err := root.SignPolynomial(ctx, poly, ExactCurvePlaneEventOptions{})
		if err != nil || contact.Sign != 0 {
			t.Fatal("retained primitive has a different original station from the independent foot equation", contact, err)
		}
		normalized := root.Interval()
		if station.ProfileParameter.Lower.Cmp(new(big.Rat).Sub(normalized.Lower, q(1, 2))) != 0 ||
			station.ProfileParameter.Upper.Cmp(new(big.Rat).Sub(normalized.Upper, q(1, 2))) != 0 {
			t.Fatal("retained normalized root no longer maps to the original raw domain", station, normalized)
		}
		for _, end := range [2]NormalOffsetSweepLineEndpoint{span.Start, span.End} {
			for i, actual := range end.Point {
				want := new(big.Rat).Add(p[i], new(big.Rat).Mul(end.Parameter, d[i]))
				if actual.Cmp(want) != 0 {
					t.Fatal("algebraic proof changed the exact rational model line endpoint", end)
				}
			}
		}
		coarse, err := s.IntersectRationalLine(ctx, p, d,
			NormalOffsetSweepLineOptions{Patch: SurfacePatchOptions{MaxOperations: got.ExactOperations - 64}})
		if err != nil || !coarse.Complete || coarse.Span == nil || coarse.Span.AlgebraicStation == nil ||
			coarse.Span.Start.Parameter.Cmp(span.Start.Parameter) != 0 || coarse.Span.End.Parameter.Cmp(span.End.Parameter) != 0 {
			t.Fatal("shortening optional witness narrowing lost the exact proved material span", coarse, err)
		}
		coarseShift := coarse.Span.AlgebraicStation.SignedDisplacement
		if coarseShift.Lower.Cmp(shift.Lower) > 0 || coarseShift.Upper.Cmp(shift.Upper) < 0 ||
			new(big.Rat).Sub(coarseShift.Upper, coarseShift.Lower).Cmp(new(big.Rat).Sub(shift.Upper, shift.Lower)) <= 0 {
			t.Fatal("coarse certified signed shift did not retain directed bounds", coarseShift, shift)
		}
		short, err := s.IntersectRationalLine(ctx, p, d,
			NormalOffsetSweepLineOptions{Patch: SurfacePatchOptions{MaxOperations: 200}})
		if !errors.Is(err, ErrConvergence) || short.Complete || short.Span != nil {
			t.Fatal("interrupted algebraic proof published a material span", short, err)
		}
		t.Logf("unseeded algebraic profile for signed case %d: raw [%s,%s], displacement [%s,%s], %d exact operations",
			c.sign, station.ProfileParameter.Lower.FloatString(14), station.ProfileParameter.Upper.FloatString(14),
			shift.Lower.FloatString(14), shift.Upper.FloatString(14), got.ExactOperations)
	}

	// This transverse point is above the allowed cap at y=0, but is
	// inside the conservative expanded hull of the original chart.
	// Its normal-foot cubic is strictly monotone and y=0 is its only
	// root. A known root above the cap may therefore prove a miss.
	p, d := placedSweepLine(world, [3]*big.Rat{q(17, 8), q(0, 1), q(17, 100)}, ruling)
	result, err := s.IntersectRationalLine(ctx, p, d, NormalOffsetSweepLineOptions{})
	if err != nil || !result.Complete || result.Span != nil {
		t.Fatal("full normal-foot ledger did not prove the near-cap miss", result, err)
	}
	t.Logf("unseeded near-cap algebraic miss, exact work %d", result.ExactOperations)
}

func TestVerifiedNormalOffsetSweepIntersectsAlgebraicCapRuling(t *testing.T) {
	ctx := context.Background()
	p, err := normalOffsetParabolicCylinder(t, false).Trim(ParameterRectangle{21.0 / 32, 11.0 / 16, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	q := big.NewRat
	s, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, q(11, 20), q(3, 5),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("independent original curved cap sweep did not verify", proof, err)
	}
	// For x²=11/100, sqrt(1+4x²)=6/5. At t=3/5 the original
	// curved normal sends physical x to zero and z to 61/100.
	// The original profile parameter is irrational, but the displaced
	// ruling and both endpoints in model coordinates are rational.
	line := [3]*big.Rat{q(0, 1), q(-1, 3), q(61, 100)}
	d := [3]*big.Rat{q(0, 1), q(2, 1), q(0, 1)}
	got, err := s.IntersectRationalLine(ctx, line, d, NormalOffsetSweepLineOptions{})
	if err != nil || !got.Complete || got.Span == nil || got.Span.AlgebraicStation == nil {
		t.Fatal("algebraic cap contact needed an exact equality proof", got, err)
	}
	span, station := got.Span, got.Span.AlgebraicStation
	if span.Start.Parameter.Cmp(q(1, 6)) != 0 || span.End.Parameter.Cmp(q(2, 3)) != 0 || station.ProfileAxis != 0 ||
		!slices.Equal(span.BoundaryFaces, []NormalOffsetSweepSide{NormalOffsetSweepUpper}) ||
		!slices.Equal(span.Start.Faces, []NormalOffsetSweepSide{NormalOffsetSweepVMin, NormalOffsetSweepUpper}) ||
		!slices.Equal(span.End.Faces, []NormalOffsetSweepSide{NormalOffsetSweepVMax, NormalOffsetSweepUpper}) ||
		station.SignedDisplacement.Lower.Cmp(q(3, 5)) != 0 || station.SignedDisplacement.Upper.Cmp(q(3, 5)) != 0 ||
		span.Start.Station[2].Cmp(q(3, 5)) != 0 || span.End.Station[2].Cmp(q(3, 5)) != 0 ||
		span.Start.Station[0] != nil || span.End.Station[0] != nil {
		t.Fatal("algebraic cap equality lost original boundary face occurrences", span)
	}
	root, err := ImportExactRoot(ctx, station.ProfileRoot, ExactRootCodecOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Physical original x = 5/16+s/16. Clear denominators
	// in x²=11/100 to get 25s²+250s-79=0.
	exact, err := root.SignPolynomial(ctx, []*big.Rat{q(-79, 1), q(250, 1), q(25, 1)}, ExactCurvePlaneEventOptions{})
	if err != nil || exact.Sign != 0 {
		t.Fatal("cap root differed from independently derived irrational profile location", exact, err)
	}
	if !slices.Equal([]string{span.Start.Point[0].RatString(), span.Start.Point[1].RatString(), span.Start.Point[2].RatString()},
		[]string{"0", "0", "61/100"}) ||
		span.End.Point[0].Sign() != 0 || span.End.Point[1].Cmp(q(1, 1)) != 0 || span.End.Point[2].Cmp(q(61, 100)) != 0 {
		t.Fatal("wrong exact physical cap-ruling endpoints", span)
	}
	t.Logf("algebraic cap equality at t=3/5, original line interval [1/6,2/3], exact work %d", got.ExactOperations)
}

func TestVerifiedNormalOffsetSweepIntersectsWeightedCubicRuling(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	weights := [4]*big.Rat{q(1, 1), q(5, 4), q(25, 16), q(125, 64)}
	recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 3, Domain: ParameterRectangle{0, 1, 2, 3}}
	for u := range 2 {
		for v, weight := range weights {
			p := [3]*big.Rat{q(int64(u), 1), q(int64(v), 3), q(0, 1)}
			if v == 3 {
				p[2] = q(1, 8)
			}
			recipe.Controls = append(recipe.Controls, [4]string{
				p[0].Mul(p[0], weight).RatString(), p[1].Mul(p[1], weight).RatString(),
				p[2].Mul(p[2], weight).RatString(), weight.RatString(),
			})
		}
	}
	source, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, q(-1, 11), q(2, 13),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("the original cubic rational extrusion must verify", proof, err)
	}
	p := [3]*big.Rat{q(-1, 3), q(1, 2), q(1, 8)}
	d := [3]*big.Rat{q(2, 1), q(0, 1), q(0, 1)}
	got, err := sweep.IntersectRationalLine(ctx, p, d, NormalOffsetSweepLineOptions{})
	if err != nil || !got.Complete || got.Span == nil || got.Span.AlgebraicStation == nil ||
		got.Span.Start.Parameter.Cmp(q(1, 6)) != 0 || got.Span.End.Parameter.Cmp(q(2, 3)) != 0 {
		t.Fatal("degree-bounded rational cubic failed the complete original ruling proof", got, err)
	}
	c := got.Span.AlgebraicStation
	root, err := ImportExactRoot(ctx, c.ProfileRoot, ExactRootCodecOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// The physical foot satisfies 32-64*y+3*y²-3*y⁵=0 for
	// z=y³/8. Substitute y=5s/(4+s) and clear the denominator.
	var independent []*big.Rat
	for _, coefficient := range []int64{32768, -40960, -56640, -22000, -3580, -9588} {
		independent = append(independent, q(coefficient, 1))
	}
	sign, err := root.SignPolynomial(ctx, independent, ExactCurvePlaneEventOptions{})
	if err != nil || sign.Sign != 0 || c.SignedDisplacement.Lower.Sign() <= 0 {
		t.Fatal("original weighted cubic station differs from the independent degree-five foot", sign, c, err)
	}
	t.Logf("weighted cubic foot retained degree-%d source equation, %d exact operations",
		len(c.ProfileRoot.Coefficients)-1, got.ExactOperations)
}

func TestVerifiedNormalOffsetSweepIntersectsRefusesNonisolatedFoot(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	var net [][4]*big.Rat
	for x := int64(0); x < 2; x++ {
		for j := int64(0); j < 3; j++ {
			weight := [3]*big.Rat{q(257, 256), q(255, 256), q(257, 256)}[j]
			net = append(net, [4]*big.Rat{
				new(big.Rat).Mul(q(x, 1), weight), q(j-1, 8), [3]*big.Rat{q(255, 256), q(257, 256), q(255, 256)}[j], weight,
			})
		}
	}
	source, err := NewExactHomogeneousBezierSurface(ctx, 1, 2, net, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, source, q(1, 1), q(2, 1),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete {
		t.Fatal("original rational circular extrusion should verify", proof, err)
	}
	// The line through the profile center is normal to the tangent
	// at every station. All signed offsets would be -1, outside the
	// positive interval. The expanded projection hull includes this
	// line and an isolated-root proof must refuse the coincident family.
	p, d := [3]*big.Rat{q(0, 1), q(0, 1), q(0, 1)}, [3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}
	r, err := sweep.IntersectRationalLine(ctx, p, d, NormalOffsetSweepLineOptions{})
	if !errors.Is(err, ErrConvergence) || r.Complete || r.Span != nil {
		t.Fatal("nonisolated normal-foot family should remain unresolved", r, err)
	}
}
