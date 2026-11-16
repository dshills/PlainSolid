package trimmed

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"

	"plainsolid/geom"
)

func projectiveClosingAxisFixture(t *testing.T) ExactQuadraticUV {
	t.Helper()
	// Elevate a line from (0,0) to (0,1) with endpoint weights 1 and 2.
	q, err := NewExactQuadraticUV([3][3]*big.Rat{
		{new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
		{new(big.Rat), big.NewRat(1, 1), big.NewRat(3, 2)},
		{new(big.Rat), big.NewRat(2, 1), big.NewRat(2, 1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestMappedClosingCoedgePhysicalClockInverse(t *testing.T) {
	raw := projectiveClosingAxisFixture(t)
	for _, tc := range []struct {
		trim ExactQuadraticUV
		want *big.Rat
	}{
		{raw, big.NewRat(1, 3)},
		{raw.reversed(), big.NewRat(2, 3)},
	} {
		clock, ok := mappedClosingClock(tc.trim)
		if !ok || clock.axis != 1 {
			t.Fatal("original degree-elevated isoparametric clock must be recognized")
		}
		rt, err := clock.parameter(big.NewRat(1, 2))
		if err != nil || rt.Cmp(tc.want) != 0 {
			t.Fatal("physical half-station must retain its non-dyadic original clock parameter", rt, err)
		}
		if outside, err := clock.parameter(big.NewRat(5, 4)); outside != nil || !errors.Is(err, geom.ErrDomain) {
			t.Fatal("source clock accepted an extrapolated physical station", outside, err)
		}
	}
	h := raw.HomogeneousControls()
	h[1][1] = big.NewRat(7, 8)
	nonlinear, err := NewExactQuadraticUV(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := mappedClosingClock(nonlinear); ok {
		t.Fatal("a straight image alone cannot establish the original fractional-linear clock")
	}
}

func TestMappedClosingTurnSubdividesOnlyProvenObstruction(t *testing.T) {
	ctx := context.Background()
	p, err := geom.NewSurfacePatch(mappedChainCreasedSupport(t, false, .5, .5), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	raw := projectiveClosingAxisFixture(t)
	quiet := mappedChainFixtureLine(t, geom.Vec2{X: 1}, geom.Vec2{X: 1, Y: .0625})
	prescribed := [][]*big.Rat{nil, {new(big.Rat), big.NewRat(1, 7), big.NewRat(1, 1)}}
	cuts, err := mappedClosingTurnCuts(ctx, p, []PCurve{raw, quiet}, prescribed, .35)
	if err != nil || cuts[0] == nil || len(cuts[0]) < 4 || !slices.Equal(cuts[1], prescribed[1]) || prescribed[0] != nil {
		t.Fatal("subdivision must allocate only the obstructing original closing edge", cuts, err)
	}
	found := false
	for _, t := range cuts[0] {
		if t.Cmp(big.NewRat(1, 3)) == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("exact physical half-station was lost", cuts)
	}
	replay, err := mappedClosingTurnCuts(ctx, p, []PCurve{raw, quiet}, cuts, .35)
	if err != nil || !slices.EqualFunc(replay[0], cuts[0], func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) ||
		!slices.Equal(replay[1], cuts[1]) {
		t.Fatal("subdivision must be idempotent for a subsequent closing seam alignment pass", replay, err)
	}
}

func TestMappedClosingTurnSharesProjectiveCoedgeCuts(t *testing.T) {
	ctx := context.Background()
	r, closing, planar, seams := mappedChainClosedFixtureWithKnot(t, true)
	var seam [2]MappedChainClosedUse
	seen := false
	for _, pair := range seams {
		a := pair[0]
		if a.Face != 0 {
			continue
		}
		clock, ok := mappedClosingClock(closing[0][a.Trim])
		if ok && clock.axis == 1 && clock.start[0].Sign() == 0 {
			seam, seen = pair, true
			break
		}
	}
	if !seen {
		t.Fatal("fixture must contain the original west closing edge")
	}
	replace := func(trim PCurve, reverseWeights bool) ExactQuadraticUV {
		w0, w1 := int64(1), int64(2)
		if reverseWeights {
			w0, w1 = w1, w0
		}
		h := [3][3]*big.Rat{}
		for i, w := range [2]int64{w0, w1} {
			q, err := mixedExactEvaluate(trim, big.NewRat(int64(i), 1))
			if err != nil {
				t.Fatal(err)
			}
			weight := big.NewRat(w, 1)
			h[2*i] = [3]*big.Rat{new(big.Rat).Mul(q[0], weight), new(big.Rat).Mul(q[1], weight), weight}
		}
		for k := range h[1] {
			h[1][k] = new(big.Rat).Quo(new(big.Rat).Add(h[0][k], h[2][k]), big.NewRat(2, 1))
		}
		q, err := NewExactQuadraticUV(h)
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	a, b := seam[0], seam[1]
	closing[0][a.Trim] = replace(closing[0][a.Trim], false)
	wall := &planar[b.Face-2]
	reverse := !wall.Reversed
	wall.Outer[b.Trim] = replace(wall.Outer[b.Trim], reverse)
	wall.Mesh.Display.Deflection = .005
	fill := MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{
		BoundaryModelBudget: .005,
		Interior:            CurvedQuadraticTrimMeshOptions{Deflection: .08},
	}}
	out, err := r.TessellateClosedCertified(ctx, closing, [2]bool{false, true}, planar, seams, MappedChainClosedMeshOptions{
		AlignSupportKnots: true, MaxKnotCellFlips: 10000, NormalTurn: .35,
		Boundary: MappedChainPolylineOptions{Deflection: .005},
		Curved:   [2]MixedExactMultiSpanMeshOptions{fill, fill},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.SquaredHalfNormalTurnTangent == nil || out.SquaredHalfNormalTurnTangent.Cmp(mappedAngleTangentLimit(.35/2)) > 0 ||
		!out.DisplayCollision.Complete || len(out.DisplayCollision.Contacts) != 0 || out.Mesh.ValidateSolid() != nil {
		t.Fatal("joint projective coedges must produce a closed shell with a continuous normal-turn bound")
	}
	curvedTrim := len(r.cuts) - 1 + a.Trim
	samples := [2]MixedExactTrimSamples{out.Curved[0].UVRegion.Samples[curvedTrim], out.Planar[b.Face-2].Samples[b.Trim]}
	want := [2]*big.Rat{big.NewRat(1, 3), big.NewRat(1, 3)}
	if reverse {
		want[1] = big.NewRat(2, 3)
	}
	var shared [2]int
	for owner, s := range samples {
		shared[owner] = -1
		for i, station := range s.Parameters {
			if station.Cmp(want[owner]) == 0 {
				shared[owner] = out.FaceVertices[[2]int{0, b.Face}[owner]][s.Vertices[i]]
			}
		}
	}
	if shared[0] < 0 || shared[0] != shared[1] {
		t.Fatal("physical midpoint must use its original projective stations and one shared display vertex", shared)
	}
}
