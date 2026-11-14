package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestGraphNormalLineNestedExact(t *testing.T) {
	if testing.Short() {
		t.Skip("complete nested native source")
	}
	ctx := context.Background()
	s := graphMaterialBooleanFixture(t, true, false)
	q, e := s.PrepareGraphMaterialQuery(ctx, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	x, y := big.NewRat(2, 3), big.NewRat(7, 13)
	h := nestedQueryHeight(x, y)
	origin := queryPoint(x, y, big.NewRat(1, 7))
	scale := big.NewRat(3, 5)
	limits := [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 1)}
	low := big.NewRat(-5, 21)
	high := new(big.Rat).Quo(new(big.Rat).Sub(h, origin[2]), scale)
	got, e := q.IntersectNormalLineRational(ctx, origin, scale, limits, geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !got.Complete || !got.Present || got.Parameters[0].Cmp(low) != 0 || got.Parameters[1].Cmp(high) != 0 || got.EndpointOnBoundary != [2]bool{true, true} || len(got.Hits) != 2 {
		t.Fatal("exact original parameter units", got)
	}
	for i, event := range got.Hits {
		if event.Before != (i == 1) || event.After != (i == 0) || len(event.Faces) != 1 || event.Faces[0] != i || len(event.RimEdges) != 0 {
			t.Fatal("native boundary transition", event)
		}
		want := new(big.Rat)
		if i == 1 {
			want = h
		}
		if event.Point[2].Cmp(want) != 0 || event.Point[0].Cmp(x) != 0 || event.Point[1].Cmp(y) != 0 {
			t.Fatal("exact physical event point", event)
		}
	}
	reversed, e := q.IntersectNormalLineRational(ctx, origin, new(big.Rat).Neg(scale), limits, geom.ExactCurvePlaneEventOptions{})
	if e != nil || len(reversed.Hits) != 2 || reversed.Hits[0].Faces[0] != 1 || reversed.Hits[1].Faces[0] != 0 || reversed.Parameters[0].Cmp(new(big.Rat).Neg(high)) != 0 || reversed.Parameters[1].Cmp(new(big.Rat).Neg(low)) != 0 {
		t.Fatal("reversed units/ancestry", reversed, e)
	}
	middle := new(big.Rat).Quo(new(big.Rat).Add(low, high), big.NewRat(2, 1))
	clipped, e := q.IntersectNormalLineRational(ctx, origin, scale, [2]*big.Rat{middle, big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !clipped.Present || clipped.EndpointOnBoundary != [2]bool{false, true} || len(clipped.Hits) != 1 || clipped.Hits[0].Faces[0] != 1 {
		t.Fatal("start inside retained material", clipped, e)
	}
	singleton, e := q.IntersectNormalLineRational(ctx, origin, scale, [2]*big.Rat{middle, middle}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !singleton.Present || len(singleton.Hits) != 0 || singleton.EndpointOnBoundary != [2]bool{} {
		t.Fatal("interior singleton must not invent boundary", singleton, e)
	}
	missed, e := q.IntersectNormalLineRational(ctx, origin, scale, [2]*big.Rat{big.NewRat(5, 1), big.NewRat(6, 1)}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !missed.Complete || missed.Present || len(missed.Hits) != 0 {
		t.Fatal("finite exclusion", missed, e)
	}
	hole, e := q.IntersectNormalLineRational(ctx, queryPoint(big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 7)), scale, limits, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !hole.Complete || hole.Present {
		t.Fatal("unselected nested band", hole, e)
	}
	for _, rim := range []*big.Rat{big.NewRat(13, 16), big.NewRat(21, 32)} {
		touch, e := q.IntersectNormalLineRational(ctx, queryPoint(rim, big.NewRat(1, 2), big.NewRat(1, 7)), scale, limits, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !touch.Present || touch.Parameters[0].Cmp(low) != 0 || touch.Parameters[1].Cmp(low) != 0 || len(touch.Hits) != 1 {
			t.Fatal("rim singleton", touch, e)
		}
		event := touch.Hits[0]
		if event.Before || event.After || len(event.Faces) != 2 || len(event.RimEdges) < 1 || len(event.RimEdges) > 2 {
			t.Fatal("regularized rim closure and native ancestry", event)
		}
	}
	native, e := s.IntersectGraphNormalLineRational(ctx, origin, scale, limits, AlgebraicGraphNormalLineOptions{})
	if e != nil || native.PreparationOperations < 1 || native.Parameters[0].Cmp(low) != 0 || native.Hits[1].Parameter.Cmp(high) != 0 {
		t.Fatal("native front door", native, e)
	}
	checkGraphNormalLineFailures(t, q, origin)
	t.Log("query work", got.ExactOperations, "native preparation", native.PreparationOperations)
}
func TestGraphNormalLinePlacedDisjoint(t *testing.T) {
	ctx := context.Background()
	s := graphMaterialBooleanFixture(t, false, true)
	axes := [2]int{0, 2}
	q, e := s.PrepareGraphMaterialQuery(ctx, AlgebraicGraphMaterialOptions{Projection: &axes})
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []*big.Rat{big.NewRat(5, 16), big.NewRat(11, 16)} {
		origin := queryPoint(new(big.Rat).Sub(big.NewRat(2, 1), x), new(big.Rat).Add(big.NewRat(-3, 1), big.NewRat(1, 7)), big.NewRat(9, 2))
		scale := big.NewRat(-2, 3)
		result, e := q.IntersectNormalLineRational(ctx, origin, scale, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !result.Present || len(result.Hits) != 2 {
			t.Fatal("placed component fiber", result, e)
		}
		for _, hit := range result.Hits {
			at, e := q.LocateRational(ctx, hit.Point, geom.ExactCurvePlaneEventOptions{})
			if e != nil || at.Location != parametric.Boundary || len(at.BoundaryFaces) != 1 || at.BoundaryFaces[0] != hit.Faces[0] {
				t.Fatal("independent native boundary classification", hit, at, e)
			}
			exact := new(big.Rat).Add(origin[1], new(big.Rat).Mul(scale, hit.Parameter))
			if exact.Cmp(hit.Point[1]) != 0 || hit.Point[0].Cmp(origin[0]) != 0 || hit.Point[2].Cmp(origin[2]) != 0 {
				t.Fatal("original XZ/world line units")
			}
		}
		result.Hits[0].Point[0].SetInt64(0)
		result.Parameters[0].SetInt64(0)
		again, e := q.IntersectNormalLineRational(ctx, origin, scale, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.ExactCurvePlaneEventOptions{})
		if e != nil || again.Hits[0].Point[0].Cmp(origin[0]) != 0 || again.Parameters[0].Sign() <= 0 {
			t.Fatal("caller-owned line witnesses", again, e)
		}
		checkGraphNormalLineFailures(t, q, origin)
	}
}
func checkGraphNormalLineFailures(t *testing.T, q *AlgebraicGraphMaterialQuery, origin [3]*big.Rat) {
	t.Helper()
	ctx := context.Background()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	limits := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
	for _, c := range []struct {
		ctx    context.Context
		scale  *big.Rat
		limits [2]*big.Rat
		o      geom.ExactCurvePlaneEventOptions
		want   error
	}{
		{ctx, new(big.Rat), limits, geom.ExactCurvePlaneEventOptions{}, geom.ErrInvalid},
		{ctx, big.NewRat(1, 1), [2]*big.Rat{limits[1], limits[0]}, geom.ExactCurvePlaneEventOptions{}, geom.ErrInvalid},
		{ctx, big.NewRat(1, 1), limits, geom.ExactCurvePlaneEventOptions{MaxOperations: 64}, geom.ErrConvergence},
		{cancelled, big.NewRat(1, 1), limits, geom.ExactCurvePlaneEventOptions{}, context.Canceled},
	} {
		out, e := q.IntersectNormalLineRational(c.ctx, origin, c.scale, c.limits, c.o)
		if !errors.Is(e, c.want) || out.Complete || out.Present || out.Parameters[0] != nil || len(out.Hits) != 0 {
			t.Fatal("atomic line failure", out, e)
		}
	}
}
