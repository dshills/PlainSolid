package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestTraceProjectiveSurfaceContactsJoinsQuadraticSaddleAcrossOriginalKnots(t *testing.T) {
	ctx := context.Background()
	for _, rotated := range []bool{false, true} {
		a, b, axes := projectiveQuadraticSaddleCharts(t, rotated)
		opts := ProjectiveSurfaceContactGraphOptions{
			Cover: ProjectiveSurfaceContactCoverOptions{IsolateQuadraticSaddles: true, Arcs: SurfacePlaneArcOptions{MaxDepth: 4}},
		}
		g, err := TraceProjectiveSurfaceContacts(ctx, a, b, axes, opts)
		if err != nil || !g.Complete || len(g.SaddleVertices) != 1 || len(g.Contours) != 4 || len(g.Vertices) != 7 || len(g.Arcs) != 6 {
			t.Fatalf("original contact network did not join: complete=%v cells=%v nodes=%d paths=%d vertices=%d arcs=%d; %v",
				g.Complete, g.Cover.IncompleteCells, len(g.SaddleVertices), len(g.Contours), len(g.Vertices), len(g.Arcs), err)
		}
		junction := g.SaddleVertices[0]
		center := g.Vertices[junction]
		assertProjectiveBoundaryRoot(t, center.OriginalUV[1][0], big.NewRat(-2, 5))
		assertProjectiveBoundaryRoot(t, center.OriginalUV[1][1], big.NewRat(32, 5))
		arcSeen := make(map[int]bool)
		usedSaddleRays := 0
		pathEnds := make(map[int]bool)
		for _, c := range g.Contours {
			if c.Closed {
				t.Fatal("each of the four rays must independently reach the overlap boundary", c)
			}
			assertProjectiveContactChain(t, g, c)
			start, finish := c.Arcs[0], c.Arcs[len(c.Arcs)-1]
			p0, p1 := g.Arcs[start.Arc].Vertices[0], g.Arcs[finish.Arc].Vertices[1]
			if start.Reversed {
				p0 = g.Arcs[start.Arc].Vertices[1]
			}
			if finish.Reversed {
				p1 = g.Arcs[finish.Arc].Vertices[0]
			}
			if p1 == junction {
				p0, p1 = p1, p0
			}
			if p0 != junction || pathEnds[p1] || !g.Vertices[p1].OnOverlapBoundary {
				t.Fatal("four distinct original ends must link to one proved saddle", c, p0, p1)
			}
			pathEnds[p1] = true
			for _, use := range c.Arcs {
				if arcSeen[use.Arc] {
					t.Fatal("exact branch overcounted across an original knot", use)
				}
				arcSeen[use.Arc] = true
				arc := g.Arcs[use.Arc]
				if !arc.Saddle {
					continue
				}
				usedSaddleRays++
				axis, err := g.ArcFreeAxis(use.Arc)
				if err != nil {
					t.Fatal(err)
				}
				end := g.Vertices[arc.Vertices[1]]
				local := new(big.Rat).Quo(new(big.Rat).Add(center.Common[axis].Interval().Lower, end.Common[axis].Interval().Lower), big.NewRat(2, 1))
				point, _, err := g.EvaluateArcRational(ctx, use.Arc, local, SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				assertProjectiveBoundaryRoot(t, point.Common[axis], local)
				assertProjectiveBoundaryRoot(t, point.Common[1-axis], center.Common[1-axis].Interval().Lower)
				var model [2][3]*big.Rat
				for i, src := range [2]*SurfacePatch{a, b} {
					u, v := point.OriginalUV[i][0].Interval(), point.OriginalUV[i][1].Interval()
					if u.Lower.Cmp(u.Upper) != 0 || v.Lower.Cmp(v.Upper) != 0 {
						t.Fatal("axis-aligned saddle made rational original positions inexact", u, v)
					}
					value, err := src.EvaluateRational(ctx, u.Lower, v.Lower, SurfacePatchEvaluationOptions{})
					if err != nil {
						t.Fatal(err)
					}
					model[i] = value.Point
				}
				for k := range model[0] {
					if model[0][k].Cmp(model[1][k]) != 0 {
						t.Fatal("saddle station evaluates to different original model points", model)
					}
				}
			}
		}
		if len(arcSeen) != len(g.Arcs) || usedSaddleRays != 4 {
			t.Fatal("lost a ray or regular continuation", arcSeen, usedSaddleRays)
		}
		opts.MaxArcs = 3
		short, err := TraceProjectiveSurfaceContacts(ctx, a, b, axes, opts)
		if !errors.Is(err, ErrConvergence) || !short.Cover.Complete || short.Complete || len(short.Arcs)+len(short.Vertices)+len(short.SaddleVertices) != 0 {
			t.Fatal("short join must retain exact cell proof but publish no incomplete network", short, err)
		}
		opts.MaxArcs = 0
		opts.Cover.IsolateQuadraticSaddles = false
		regular, err := TraceProjectiveSurfaceContacts(ctx, a, b, axes, opts)
		if !errors.Is(err, ErrConvergence) || regular.Complete || len(regular.Cover.IncompleteCells) == 0 {
			t.Fatal("ordinary contact tracing should not silently promote the saddle", regular, err)
		}
	}
}
