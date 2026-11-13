package thread

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestBuildRelievedPairRunoutProbeUsesFinishedBands(t *testing.T) {
	ctx := context.Background()
	p := pairFixture(t, geom.RightHanded)
	relief := PairReliefs{
		ShaftTip: trimmed.HelicalReliefSpec{Radius: 1.5, Depth: .5},
		NutLower: trimmed.HelicalReliefSpec{Radius: 2.125, Depth: .5},
		NutUpper: trimmed.HelicalReliefSpec{Radius: 2.25, Depth: .75},
	}
	got, e := BuildRelievedPair(ctx, p, relief, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.ShaftTipRemovedPi.Cmp(big.NewRat(7, 8)) != 0 || got.NutReliefsRemovedPi.Cmp(big.NewRat(285, 128)) != 0 {
		t.Fatal(got.ShaftTipRemovedPi, got.NutReliefsRemovedPi)
	}
	for i, s := range []trimmed.Solid{got.Shaft, got.Nut} {
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6, MaxSubdivisions: 128, MaxSamples: 16000000})
		want := []float64{(26.75 - .875) * math.Pi, (43.5 - 285./128) * math.Pi}[i]
		if e != nil || math.Abs(m.Volume-want) > 1e-6 {
			t.Fatal(i, m.Volume, want, e)
		}
	}
	r := big.NewRat(15, 8)
	out, e := got.CompareRunoutQuarter(ctx, r, new(big.Rat), new(big.Rat), geom.HelicalStartsLineOptions{})
	if e != nil || !out.Complete || !out.Overlap || out.A[0].Bounds[0].Cmp(big.NewRat(1, 2)) != 0 ||
		out.B[len(out.B)-1].Bounds[1].Cmp(big.NewRat(29, 4)) != 0 {
		t.Fatal("wrong relieved radial stock slice", out, e)
	}
	relief.NutLower.Radius = 1.875
	x, e := BuildRelievedPair(ctx, p, relief, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := x.CompareRunoutQuarter(ctx, r, new(big.Rat), new(big.Rat), geom.HelicalStartsLineOptions{}); !errors.Is(e, geom.ErrDomain) {
		t.Fatal("radial boundary must be handled explicitly", e)
	}
	relief.ShaftTip.Depth = 1
	if out, e := BuildRelievedPair(ctx, p, relief, geom.SurfacePatchEvaluationOptions{}); !errors.Is(e, geom.ErrConvergence) || len(out.Nut.Faces()) != 0 {
		t.Fatal("overlapping relief returned partial stock", e)
	}
}

func TestRelievedPairFullTurnChecksReliefTransition(t *testing.T) {
	ctx := context.Background()
	p := pairFixture(t, geom.RightHanded)
	radius := big.NewRat(15, 8)
	spec := PairReliefs{
		ShaftTip: trimmed.HelicalReliefSpec{Radius: 1.875, Depth: .5},
		NutLower: trimmed.HelicalReliefSpec{Radius: 2.125, Depth: .5},
		NutUpper: trimmed.HelicalReliefSpec{Radius: 2.25, Depth: .75},
	}
	got, e := BuildRelievedPair(ctx, p, spec, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r := [2]*big.Rat{big.NewRat(29, 16), big.NewRat(31, 16)}
	out, e := got.CompareRunoutFullTurn(ctx, big.NewRat(8, 1), r, geom.HelicalStartsLineOptions{})
	if e != nil || !out.Complete || !out.NoPositiveOverlap || out.CheckedSlabs != 3 {
		t.Fatal("complete split/full-turn proof", out, e)
	}
	r[0] = radius
	out, e = got.CompareRunoutFullTurn(ctx, new(big.Rat), r, geom.HelicalStartsLineOptions{})
	if e != nil || !out.Complete || out.NoPositiveOverlap || !out.WitnessOnReliefWall || out.Witness.WitnessRadius.Cmp(radius) != 0 {
		t.Fatal("relief boundary witness", out, e)
	}
	if out, e := got.CompareRunoutFullTurn(ctx, new(big.Rat), r, geom.HelicalStartsLineOptions{
		HelicalPolygonLineOptions: geom.HelicalPolygonLineOptions{MaxOperations: 1},
	}); !errors.Is(e, geom.ErrConvergence) || out.Complete {
		t.Fatal("partial proof on budget", out, e)
	}
}
