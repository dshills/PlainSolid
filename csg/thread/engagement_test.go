package thread

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestVerifyEngagementClipsFiniteNeighboringTurns(t *testing.T) {
	ctx := context.Background()
	for _, hand := range []geom.HelixHandedness{geom.RightHanded, geom.LeftHanded} {
		p, e := BuildPair(ctx, pairFixture(t, hand), geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		// Shaft cutters index3/4 straddle a turn wrap. Their source turns
		// are 1+phase and 2+phase; finite UMax=5/2 clips phase to 1/2.
		r := big.NewRat(15, 8)
		phase := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
		proof, e := p.VerifyEngagement(ctx, r, 3, phase, geom.SurfacePatchEvaluationOptions{})
		if e != nil || !proof.Complete || !proof.Present || proof.WholeWindow || proof.Phase[1].Cmp(big.NewRat(1, 2)) != 0 ||
			proof.WitnessPhase.Cmp(big.NewRat(1, 4)) != 0 || proof.LowerGap.Cmp(big.NewRat(1, 8)) != 0 || proof.UpperGap.Cmp(big.NewRat(1, 8)) != 0 {
			t.Fatal(hand, proof, e)
		}
		if proof.Ridge.Copies[0].Start != 1 || proof.Ridge.Copies[1].Start != 0 ||
			proof.WitnessShaft[0].Cmp(big.NewRat(33, 8)) != 0 || proof.WitnessShaft[1].Cmp(big.NewRat(35, 8)) != 0 {
			t.Fatal("wrong axial ridge witness", proof.Ridge, proof.WitnessShaft)
		}
		// Mutating the public report must not affect a later proof.
		p.Fit.RadialBand[0].SetInt64(0)
		phase[0] = big.NewRat(3, 4)
		out, e := p.VerifyEngagement(ctx, r, 3, phase, geom.SurfacePatchEvaluationOptions{})
		if e != nil || !out.Complete || out.Present {
			t.Fatal("finite turn absent", out, e)
		}
	}
}

func TestCompareRunoutQuarterDistinguishesEndInterference(t *testing.T) {
	ctx := context.Background()
	p, e := BuildPair(ctx, pairFixture(t, geom.RightHanded), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat(15, 8)
	phase := new(big.Rat)
	out, e := p.CompareRunoutQuarter(ctx, r, phase, new(big.Rat), geom.HelicalStartsLineOptions{})
	if e != nil || !out.Complete || !out.Overlap || len(out.Contacts) == 0 {
		t.Fatal("nominal flank fit must not hide finite end interference", out, e)
	}
	out, e = p.CompareRunoutQuarter(ctx, r, phase, big.NewRat(8, 1), geom.HelicalStartsLineOptions{})
	if e != nil || !out.Complete || out.Overlap || !out.Touch {
		t.Fatal("translated stocks should only touch at a cap", out, e)
	}
}

func TestCompareRunoutPhaseWindow(t *testing.T) {
	ctx := context.Background()
	p, e := BuildPair(ctx, pairFixture(t, geom.RightHanded), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat(15, 8)
	phase := [2]*big.Rat{big.NewRat(1, 7), big.NewRat(6, 7)}
	out, e := p.CompareRunoutPhaseWindow(ctx, r, new(big.Rat), phase, geom.HelicalStartsLineOptions{})
	if e != nil || !out.Complete || out.NoPositiveOverlap || out.WitnessPhase == nil || !out.Witness.Overlap ||
		out.WitnessPhase.Cmp(phase[0]) < 0 || out.WitnessPhase.Cmp(phase[1]) > 0 {
		t.Fatal("missing finite runout overlap witness", out, e)
	}
	out, e = p.CompareRunoutPhaseWindow(ctx, r, big.NewRat(8, 1), phase, geom.HelicalStartsLineOptions{})
	if e != nil || !out.Complete || !out.NoPositiveOverlap || out.WitnessPhase != nil || out.Probes < 2 {
		t.Fatal("cap-only contact must have no positive overlap", out, e)
	}
}
