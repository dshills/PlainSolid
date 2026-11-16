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

func TestTraceProjectiveSurfaceContactsUsesExactFreeProjectionDerivative(t *testing.T) {
	ctx := context.Background()
	for _, axes := range [][2]int{{0, 1}, {1, 2}, {2, 0}} {
		var patch [2]*SurfacePatch
		for owner := range patch {
			var poles []Vec3
			var weights []float64
			for i := range 2 {
				for j := range 2 {
					x, y, w := float64(i), float64(j), 1.
					z := 1 + x*y
					if owner == 0 {
						w += y
					} else {
						z += x - .5
					}
					var p [3]float64
					p[axes[0]], p[axes[1]], p[3-axes[0]-axes[1]] = x, y, z
					poles = append(poles, Vec3{p[0], p[1], p[2]})
					weights = append(weights, w)
				}
			}
			knots := []float64{0, 0, 1, 1}
			surface, err := NewBSplineSurface(1, 1, 2, 2, poles, knots, knots, weights)
			if err != nil {
				t.Fatal(err)
			}
			patch[owner], err = NewSurfacePatch(surface, SurfacePatchOptions{})
			if err != nil {
				t.Fatal(err)
			}
		}
		g, err := TraceProjectiveSurfaceContacts(ctx, patch[0], patch[1], axes, ProjectiveSurfaceContactGraphOptions{})
		if err != nil || !g.Complete || len(g.Arcs) != 1 || len(g.Cover.Isolations) != 1 {
			t.Fatal("the original fractional chart must retain one unbroken arc", axes, err)
		}
		arc := g.Cover.Isolations[0].Arcs[0]
		d := arc.Certificate.WorldDerivative
		if arc.Depth != 0 || arc.OnBoundary || arc.Certificate.WorldAxis != 1 ||
			d.Sign != 1 || d.Min != .5 || d.Max != 2 {
			t.Fatal("the exact derivative of 2v/(1+v) is 2/(1+v)^2", axes, arc.Certificate)
		}
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
