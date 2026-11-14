package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestGraphObliqueLineRetainedDisjoint(t *testing.T) {
	ctx := context.Background()
	axes := [2]int{0, 2}
	s := graphMaterialBooleanFixture(t, false, true)
	q, e := s.PrepareGraphMaterialQuery(ctx, AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	zero := new(big.Rat)
	o := geom.ToroidLineOptions{ParameterTolerance: 1e-10}
	// At height 9/4096 above the signed XZ stock plane, each quadratic
	// roof closes on a circle of radius 4/64. With Z fixed on the projected
	// centerline, the roots in original local X are 1/4,3/8,5/8,3/4.
	origin := [3]*big.Rat{big.NewRat(2, 1), big.NewRat(-12279, 4096), big.NewRat(9, 2)}
	direction := [3]*big.Rat{big.NewRat(-3, 5), zero, zero}
	limits := [2]*big.Rat{big.NewRat(1, 7), big.NewRat(9, 7)}
	r, e := q.IntersectObliqueLineRational(ctx, origin, direction, limits, o)
	if e != nil || !r.Complete || len(r.Intervals) != 2 || len(r.Hits) != 4 {
		t.Fatal("both retained material intervals must survive the signed projection", r, e)
	}
	want := [4]*big.Rat{big.NewRat(5, 12), big.NewRat(5, 8), big.NewRat(25, 24), big.NewRat(5, 4)}
	for i, hit := range r.Hits {
		if hit.Parameter.Cmp(want[i]) != 0 || hit.Before != (i%2 != 0) || hit.After != (i%2 == 0) || len(hit.Faces) != 1 || len(hit.RimEdges) != 0 {
			t.Fatal("original retained roof transition", i, hit, want[i])
		}
		f, e := s.OriginalFace(hit.Faces[0])
		if e != nil || f.Surface != q.faces[1].Surface {
			t.Fatal("a non-roof support owned the observed transition", hit.Faces, e)
		}
	}
	for i, span := range r.Intervals {
		if span.Lower.Cmp(want[2*i]) != 0 || span.Upper.Cmp(want[2*i+1]) != 0 {
			t.Fatal("wrong closed retained intervals", span)
		}
	}
	// Half peak height has irrational roof contacts at projected radius
	// sqrt(25/8192). The small closed brackets must retain their original
	// roof owners and enclose the independent exact squared radii.
	irrational := origin
	irrational[1] = big.NewRat(-24551, 8192)
	refined, _, e := q.WithRefinedWindowTubes(ctx, AlgebraicGraphTubeWindowOptions{SubdivisionDepth: 2})
	if e != nil {
		t.Fatal(e)
	}
	bounded, e := refined.BoundObliqueLineRational(ctx, irrational, direction, limits, o)
	if e != nil || !bounded.Complete || len(bounded.Intervals) != 2 || len(bounded.Hits) != 4 {
		t.Fatal("properly enclosed irrational roof events", bounded, e)
	}
	for i, hit := range bounded.Hits {
		center := [2]*big.Rat{big.NewRat(27, 16), big.NewRat(21, 16)}[i/2]
		squared := func(x *big.Rat) *big.Rat {
			d := new(big.Rat).Sub(x, center)
			return d.Mul(d, d)
		}
		xyz := hit.PointBounds[0]
		low, high := squared(xyz.Lower), squared(xyz.Upper)
		if low.Cmp(high) > 0 {
			low, high = high, low
		}
		r2 := big.NewRat(25, 8192)
		if hit.Point[0] != nil || hit.Parameter.Lower.Cmp(hit.Parameter.Upper) >= 0 ||
			new(big.Rat).Sub(hit.Parameter.Upper, hit.Parameter.Lower).Cmp(ratInset(o.ParameterTolerance)) > 0 ||
			low.Cmp(r2) > 0 || high.Cmp(r2) < 0 || len(hit.Faces) != 1 || hit.Faces[0] != r.Hits[i].Faces[0] || len(hit.RimEdges) != 0 ||
			hit.Before != r.Hits[i].Before || hit.After != r.Hits[i].After {
			t.Fatal("irrational event ownership or certified squared radius", i, hit, r2)
		}
		left, right := "0", "1"
		if i%2 != 0 {
			left, right = "-1", "0"
		}
		// Compare the stored original line root to an independently
		// composed polynomial identity, not just its numeric bracket.
		radius, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"-25", "0", "8192"}, Lower: left, Upper: right}, geom.ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		mid := new(big.Rat).Quo(new(big.Rat).Sub(big.NewRat(2, 1), center), big.NewRat(3, 5))
		wantRoot, _, e := radius.AffineMap(ctx, mid, big.NewRat(-5, 3), geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		relation, e := hit.Root.Compare(ctx, wantRoot, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !relation.Proven || relation.Order != 0 {
			t.Fatal("lost exact root identity in original line units", i, relation, e)
		}

	}
	if bad, e := q.IntersectObliqueLineRational(ctx, irrational, direction, limits, o); !errors.Is(e, geom.ErrConvergence) || bad.Complete || len(bad.Hits)+len(bad.Intervals) != 0 {
		t.Fatal("exact point API must reject the nonrational request atomically", bad, e)
	}
	// A finite segment strictly inside a component must not invent new
	// boundary contacts at its clipping endpoints.
	clip := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(3, 5)}
	inside, e := q.IntersectObliqueLineRational(ctx, origin, direction, clip, o)
	if e != nil || !inside.Complete || len(inside.Intervals) != 1 || len(inside.Hits) != 0 ||
		inside.Intervals[0].Lower.Cmp(clip[0]) != 0 || inside.Intervals[0].Upper.Cmp(clip[1]) != 0 {
		t.Fatal("strictly interior clipped segment", inside, e)
	}
	// The plane crossing in the gap between lobes belongs to neither cap.
	gap := [3]*big.Rat{big.NewRat(3, 2), big.NewRat(-3, 1), big.NewRat(9, 2)}
	inclined := [3]*big.Rat{big.NewRat(3, 5), big.NewRat(1, 64), zero}
	miss, e := q.IntersectObliqueLineRational(ctx, gap, inclined, [2]*big.Rat{big.NewRat(-1, 64), big.NewRat(1, 64)}, o)
	if e != nil || !miss.Complete || len(miss.Hits)+len(miss.Intervals) != 0 {
		t.Fatal("spurious original cap contact between lobes", miss, e)
	}
	// The line in the original cap plane retains two face intervals.
	// Four exact rim sections delimit it; neither buried space nor a
	// candidate on the untrimmed support can create an overlap.
	coincident := origin
	coincident[1] = big.NewRat(-3, 1)
	overlaps, e := q.IntersectObliqueLineRational(ctx, coincident, direction, [2]*big.Rat{big.NewRat(1, 7), big.NewRat(7, 5)}, o)
	if e != nil || !overlaps.Complete || len(overlaps.Intervals) != 2 || len(overlaps.Hits) != 4 || len(overlaps.BoundarySpans) != 2 {
		t.Fatal("original retained cap-plane intervals", overlaps, e)
	}
	capRoots := [4]*big.Rat{big.NewRat(25, 64), big.NewRat(125, 192), big.NewRat(65, 64), big.NewRat(245, 192)}
	for i, hit := range overlaps.Hits {
		if hit.Parameter.Cmp(capRoots[i]) != 0 || hit.Before != (i%2 != 0) || hit.After != (i%2 == 0) || len(hit.Faces) != 2 || len(hit.RimEdges) == 0 {
			t.Fatal("shared original cap/roof rim contact", i, hit, capRoots[i])
		}
		for _, edge := range hit.RimEdges {
			if _, _, e := s.SectionSegment(edge); e != nil {
				t.Fatal(e)
			}
		}
	}
	for i, span := range overlaps.BoundarySpans {
		f, e := s.OriginalFace(span.Face)
		if e != nil || f.Surface != q.faces[0].Surface || span.Start.Lower.Cmp(capRoots[2*i]) != 0 || span.End.Upper.Cmp(capRoots[2*i+1]) != 0 {
			t.Fatal("correct retained cap owner/effective range", span, e)
		}
	}
	clipOverlap, e := q.IntersectObliqueLineRational(ctx, coincident, direction, clip, o)
	if e != nil || !clipOverlap.Complete || len(clipOverlap.Hits) != 0 || len(clipOverlap.BoundarySpans) != 1 || len(clipOverlap.Intervals) != 1 {
		t.Fatal("finite clipping cannot invent rim hits", clipOverlap, e)
	}

	// Offset the projected axis by 1/32 in the cap plane: retained
	// crossings are irrational with model radius sqrt(21)/64.
	obliqueCap := coincident
	obliqueCap[2] = big.NewRat(145, 32)
	capAlgebraic, e := refined.BoundObliqueLineRational(ctx, obliqueCap, direction, [2]*big.Rat{big.NewRat(1, 7), big.NewRat(7, 5)}, o)
	if e != nil || !capAlgebraic.Complete || len(capAlgebraic.Hits) != 4 || len(capAlgebraic.BoundarySpans) != 2 {
		t.Fatal("irrational shared cap/roof rims", capAlgebraic, e)
	}
	for i, h := range capAlgebraic.Hits {
		if h.Root == nil || h.Point[0] != nil || h.Parameter.Lower.Cmp(h.Parameter.Upper) >= 0 || len(h.Faces) != 2 || len(h.RimEdges) != 1 || h.Before != (i%2 != 0) || h.After != (i%2 == 0) {
			t.Fatal("original irrational rim ownership", i, h)
		}
		if _, _, e := s.SectionSegment(h.RimEdges[0]); e != nil {
			t.Fatal(e)
		}
		center := [2]*big.Rat{big.NewRat(27, 16), big.NewRat(21, 16)}[i/2]
		for j, x := range []*big.Rat{h.PointBounds[0].Lower, h.PointBounds[0].Upper} {
			d := new(big.Rat).Sub(x, center)
			cmp := d.Mul(d, d).Cmp(big.NewRat(21, 4096))
			if (i+j)%2 == 0 && cmp > 0 || (i+j)%2 != 0 && cmp < 0 {
				t.Fatal("irrational rim bracket misses the independent radius", i, h)
			}
		}

	}

	// At peak height each quadratic contributes an isolated tangent
	// point in the closed boundary with no material on either side.
	peak := origin
	peak[1] = big.NewRat(-12263, 4096)
	peaks, e := q.IntersectObliqueLineRational(ctx, peak, direction, limits, o)
	if e != nil || !peaks.Complete || len(peaks.Hits) != 2 || len(peaks.Intervals) != 2 || len(peaks.BoundarySpans) != 0 {
		t.Fatal("two isolated regularized roof tangencies", peaks, e)
	}
	for i, h := range peaks.Hits {
		want := [2]*big.Rat{big.NewRat(25, 48), big.NewRat(55, 48)}[i]
		if h.Before || h.After || h.Parameter.Cmp(want) != 0 || peaks.Intervals[i].Lower.Cmp(want) != 0 || peaks.Intervals[i].Upper.Cmp(want) != 0 {
			t.Fatal(h, peaks.Intervals[i])
		}
	}

	// Distinct roots can be far closer than the requested tolerance
	// on either side of an exact tangent. They must never be merged.
	epsilon2 := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 199))
	nearPeak := peak
	nearPeak[1] = new(big.Rat).Sub(peak[1], epsilon2)
	closeRoots, e := refined.BoundObliqueLineRational(ctx, nearPeak, direction, [2]*big.Rat{big.NewRat(1, 2), big.NewRat(3, 5)}, o)
	if e != nil || !closeRoots.Complete || len(closeRoots.Hits) != 2 || len(closeRoots.Intervals) != 1 ||
		closeRoots.Hits[0].Parameter.Upper.Cmp(closeRoots.Hits[1].Parameter.Lower) >= 0 {
		t.Fatal("distinct roots separated by less than one ULP", closeRoots, e)
	}

	limited := o
	limited.Surface.MaxOperations = 20
	if bad, e := q.IntersectObliqueLineRational(ctx, origin, direction, limits, limited); !errors.Is(e, geom.ErrConvergence) || bad.Complete || len(bad.Hits)+len(bad.Intervals) != 0 {
		t.Fatal("budget must reject atomically", bad, e)
	}
}
