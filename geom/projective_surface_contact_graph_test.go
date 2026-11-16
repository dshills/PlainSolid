package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestTraceProjectiveSurfaceContactsJoinsTheOriginalNonbinaryKnot(t *testing.T) {
	ctx := context.Background()
	a, b := projectiveTinyLinearHeightCharts(t)
	r, err := TraceProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{})
	if err != nil || !r.Complete || len(r.Contours) != 1 || r.Contours[0].Closed || len(r.Contours[0].Arcs) != 2 ||
		len(r.Vertices) != 3 || len(r.Arcs) != 2 {
		t.Fatal("global original branch was not joined across its source knot", r, err)
	}
	interior := 0
	for _, v := range r.Vertices {
		assertProjectiveBoundaryRoot(t, v.Common[0], big.NewRat(1, 2))
		assertProjectiveBoundaryRoot(t, v.OriginalUV[1][1], big.NewRat(32, 5))
		if !v.OnOverlapBoundary {
			interior++
			assertProjectiveBoundaryRoot(t, v.Common[1], big.NewRat(9, 10))
			assertProjectiveBoundaryRoot(t, v.OriginalUV[0][1], big.NewRat(19, 20))
			assertProjectiveBoundaryRoot(t, v.OriginalUV[1][0], new(big.Rat))
		}
	}
	if interior != 1 {
		t.Fatal("both original cells must share one interior vertex", r.Vertices)
	}
	assertProjectiveContactChain(t, r, r.Contours[0])
	short, err := TraceProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{MaxJoinComparisons: 1})
	if !errors.Is(err, ErrConvergence) || short.Complete || !short.Cover.Complete || len(short.Arcs)+len(short.Vertices)+len(short.Contours) != 0 {
		t.Fatal("failed join must retain exact local proof without a partial global graph", short, err)
	}
}

func TestTraceProjectiveSurfaceContactsFindsAClosedOriginalContour(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	b, err := NewSurfacePatch(quadraticContactSurface(t, -.125), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := TraceProjectiveSurfaceContacts(ctx, a, b, [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{})
	if err != nil || !r.Complete || len(r.Contours) != 1 || !r.Contours[0].Closed || len(r.Contours[0].Arcs) != len(r.Arcs) || len(r.Arcs) < 4 {
		t.Fatalf("circular contour not preserved: complete=%v local=%v vertices=%d arcs=%d contours=%d: %v",
			r.Complete, r.Cover.Complete, len(r.Vertices), len(r.Arcs), len(r.Contours), err)
	}
	for _, v := range r.Vertices {
		if v.OnOverlapBoundary {
			t.Fatal("interior loop acquired a false projected boundary crossing", v)
		}
	}
	assertProjectiveContactChain(t, r, r.Contours[0])
	point, err := NewSurfacePatch(quadraticContactSurface(t, 0), SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err = TraceProjectiveSurfaceContacts(ctx, a, point, [2]int{0, 1}, ProjectiveSurfaceContactGraphOptions{
		Cover: ProjectiveSurfaceContactCoverOptions{Arcs: SurfacePlaneArcOptions{IsolateQuadraticTangencies: true}}})
	if !errors.Is(err, ErrConvergence) || r.Complete || len(r.Contours) != 0 || len(r.Cover.Isolations) == 0 {
		t.Fatal("an isolated singular contact must not be assembled as a regular contour", r, err)
	}
}

func assertProjectiveContactChain(t *testing.T, g ProjectiveSurfaceContactGraph, c ProjectiveContactContour) {
	t.Helper()
	have := map[int]bool{}
	get := func(use ProjectiveContactArcUse) (start, end int) {
		x := g.Arcs[use.Arc].Vertices
		if use.Reversed {
			return x[1], x[0]
		}
		return x[0], x[1]
	}
	var first, last int
	for i, u := range c.Arcs {
		start, end := get(u)
		if have[u.Arc] || i > 0 && start != last {
			t.Fatal("contact arc was duplicated or its exact vertex is wrong", i, u, c)
		}
		if i == 0 {
			first = start
		}
		last = end
		have[u.Arc] = true
	}
	if c.Closed != (first == last) {
		t.Fatal("wrong contact chain closure", c)
	}
}
