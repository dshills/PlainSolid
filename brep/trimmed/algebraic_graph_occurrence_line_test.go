package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestAlgebraicGraphOccurrenceWorldLines(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source := graphMaterialBooleanFixture(t, false, false)
	var pose [3][4]*big.Rat
	for i := range pose {
		for j := range pose[i] {
			pose[i][j] = new(big.Rat)
		}
	}
	pose[0][0] = big.NewRat(2, 1)
	pose[0][1] = big.NewRat(1, 3)
	pose[0][3] = big.NewRat(1, 7)
	pose[1][0] = big.NewRat(1, 5)
	pose[1][2] = big.NewRat(-3, 1)
	pose[1][3] = big.NewRat(2, 7)
	pose[2][1] = big.NewRat(1, 1)
	pose[2][3] = big.NewRat(-1, 11)
	occurrence, e := NewAlgebraicGraphOccurrence(ctx, source, pose, [2]int{0, 2}, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	image := func(p [3]*big.Rat, point bool) (r [3]*big.Rat) {
		for i := range r {
			r[i] = new(big.Rat)
			if point {
				r[i].Set(pose[i][3])
			}
			for j := 0; j < 3; j++ {
				r[i].Add(r[i], new(big.Rat).Mul(pose[i][j], p[j]))
			}
		}
		return r
	}
	center := queryPoint(big.NewRat(5, 16), big.NewRat(1, 2), new(big.Rat))
	normalOrigin := image(center, true)
	normal, e := occurrence.IntersectNormalLineRational(ctx, normalOrigin, big.NewRat(2, 3), [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !normal.Complete || !normal.Present || len(normal.Hits) != 2 || normal.Parameters[0].Cmp(big.NewRat(-225, 8192)) != 0 || normal.Parameters[1].Sign() != 0 {
		t.Fatal("signed world normal interval", normal, e)
	}
	for i, h := range normal.Hits {
		for axis, p := range h.Point {
			want := new(big.Rat).Set(normalOrigin[axis])
			if axis == 1 {
				want.Add(want, new(big.Rat).Mul(big.NewRat(2, 3), h.Parameter))
			}
			if p.Cmp(want) != 0 {
				t.Fatal("world normal witness", i, axis)
			}
		}
		if h.Before != (i == 1) || h.After != (i == 0) || len(h.Faces) != 1 {
			t.Fatal("original t transition", h)
		}
	}
	originalDirection := queryPoint(big.NewRat(-3, 5), new(big.Rat), new(big.Rat))
	worldDirection := image(originalDirection, false)
	origin := image(queryPoint(big.NewRat(5, 16), big.NewRat(1, 2), big.NewRat(175, 32768)), true)
	limits := [2]*big.Rat{big.NewRat(-1, 16), big.NewRat(1, 16)}
	options := geom.ToroidLineOptions{ParameterTolerance: 1e-10}
	line, e := occurrence.BoundObliqueLineRational(ctx, origin, worldDirection, limits, options)
	if e != nil || !line.Complete || len(line.Intervals) != 1 || len(line.Hits) != 2 {
		t.Fatal("world algebraic line", line, e)
	}
	for i, h := range line.Hits {
		lower, upper := "-1", "0"
		if i == 1 {
			lower, upper = "0", "1"
		}
		independent, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"-625", "0", "294912"}, Lower: lower, Upper: upper}, geom.ExactRootCodecOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if h.Root == nil || h.Point[0] != nil || len(h.Faces) != 1 || h.Before != (i == 1) || h.After != (i == 0) {
			t.Fatal("retained exact algebraic event", i, h)
		}
		relation, e := h.Root.Compare(ctx, independent, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !relation.Proven || relation.Order != 0 {
			t.Fatal("original t identity", i, relation, e)
		}
		for axis, bounds := range h.PointBounds {
			lo := new(big.Rat).Add(origin[axis], new(big.Rat).Mul(worldDirection[axis], h.Parameter.Lower))
			hi := new(big.Rat).Add(origin[axis], new(big.Rat).Mul(worldDirection[axis], h.Parameter.Upper))
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			if bounds.Lower.Cmp(lo) > 0 || bounds.Upper.Cmp(hi) < 0 {
				t.Fatal("world XYZ enclosure", axis)
			}
		}
	}
	rationalOrigin := image(queryPoint(big.NewRat(5, 16), big.NewRat(1, 2), big.NewRat(21, 4096)), true)
	rational, e := occurrence.BoundObliqueLineRational(ctx, rationalOrigin, worldDirection, limits, options)
	if e != nil || len(rational.Hits) != 2 {
		t.Fatal("rational world oblique", e)
	}
	for i, h := range rational.Hits {
		want := big.NewRat(-5, 96)
		if i == 1 {
			want.Neg(want)
		}
		if h.Parameter.Lower.Cmp(want) != 0 || h.Parameter.Upper.Cmp(want) != 0 || h.Point[0] == nil {
			t.Fatal("rational t", h)
		}
		for axis, p := range h.Point {
			expected := new(big.Rat).Add(rationalOrigin[axis], new(big.Rat).Mul(worldDirection[axis], want))
			if p.Cmp(expected) != 0 || h.PointBounds[axis].Lower.Cmp(p) != 0 || h.PointBounds[axis].Upper.Cmp(p) != 0 {
				t.Fatal("exact world witness", axis)
			}
		}
	}
	capLine, e := occurrence.BoundObliqueLineRational(ctx, normalOrigin, worldDirection, [2]*big.Rat{big.NewRat(-1, 4), big.NewRat(1, 4)}, options)
	if e != nil || len(capLine.BoundarySpans) != 1 || len(capLine.Hits) != 2 {
		t.Fatal("world cap overlap", capLine, e)
	}
	span := capLine.BoundarySpans[0]
	if span.Start.Lower.Cmp(big.NewRat(-25, 192)) != 0 || span.End.Upper.Cmp(big.NewRat(25, 192)) != 0 || span.Face != normal.Hits[1].Faces[0] {
		t.Fatal("original cap owner/units", span)
	}
	for _, h := range capLine.Hits {
		if len(h.RimEdges) == 0 {
			t.Fatal("cap overlap lost native rim", h)
		}
	}
	var encoded bytes.Buffer
	if e = WriteAlgebraicGraphOccurrenceJSON(&encoded, occurrence); e != nil {
		t.Fatal(e)
	}
	section := AlgebraicShellSectionOptions{Trace: nestedTubeSectionOptions().Trace}
	section.Trace.Coverage.OverlappingAffineXYFallback = true
	boolean := AlgebraicShellBooleanOptions{}
	boolean.Imprint.Exact.MaxOperations = 512000000
	back, e := ReadAlgebraicGraphOccurrenceJSON(ctx, &encoded, JSONLimits{}, section, boolean, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal("native replay", e)
	}
	reloadedNormal, e := back.IntersectNormalLineRational(ctx, normalOrigin, big.NewRat(2, 3), [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !reflect.DeepEqual(reloadedNormal, normal) {
		t.Fatal("reloaded signed normal", e)
	}
	again, e := back.BoundObliqueLineRational(ctx, origin, worldDirection, limits, options)
	if e != nil || len(again.Hits) != len(line.Hits) || !reflect.DeepEqual(again.Intervals, line.Intervals) {
		t.Fatal("reloaded line", e)
	}
	for i, h := range again.Hits {
		relation, e := h.Root.Compare(ctx, line.Hits[i].Root, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !relation.Proven || relation.Order != 0 || !reflect.DeepEqual(h.PointBounds, line.Hits[i].PointBounds) || !reflect.DeepEqual(h.Faces, line.Hits[i].Faces) || !reflect.DeepEqual(h.RimEdges, line.Hits[i].RimEdges) {
			t.Fatal("reloaded exact root/owner", i, e)
		}
	}
	for _, limit := range []int{1, line.ExactOperations - 1} {
		badOptions := options
		badOptions.Surface.MaxOperations = limit
		bad, e := occurrence.BoundObliqueLineRational(ctx, origin, worldDirection, limits, badOptions)
		if e == nil || bad.Complete || len(bad.Hits)+len(bad.Intervals) > 0 {
			t.Fatal("atomic oblique budget", e)
		}
	}
	badNormal, e := occurrence.IntersectNormalLineRational(ctx, normalOrigin, big.NewRat(2, 3), limits, geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || badNormal.Complete || len(badNormal.Hits) > 0 {
		t.Fatal("normal budget", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	bad, e := occurrence.BoundObliqueLineRational(cancelled, origin, worldDirection, limits, options)
	if !errors.Is(e, context.Canceled) || bad.Complete {
		t.Fatal("cancel", e)
	}
	t.Logf("world XZ normal%d operations, oblique%d operations, exact irrational roots of294912t²−625", normal.ExactOperations, line.ExactOperations)
}
