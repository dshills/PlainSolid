//go:build research

package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestIntersectSplineEdgesAndApplyAnchors(t *testing.T) {
	ctx := context.Background()
	a, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 1}, 0)
	b, _ := makeOpenSplineBox(t, [3]float64{.5, -.5, 0}, [3]float64{1.5, .5, 1}, 0)
	find := func(s Solid, a, b geom.Vec3) int {
		for i, e := range s.edges {
			p, q := s.vertices[e.Start], s.vertices[e.End]
			if p == a && q == b || p == b && q == a {
				return i
			}
		}
		t.Fatal("expected original edge missing", a, b)
		return -1
	}
	x := find(a, geom.Vec3{}, geom.Vec3{X: 1})
	y := find(b, geom.Vec3{X: .5, Y: -.5}, geom.Vec3{X: .5, Y: .5})
	r, e := IntersectSplineEdges(ctx, a, x, b, y, geom.ExactCurveSegmentOptions{})
	if e != nil || !r.Complete || len(r.Contacts) != 1 {
		t.Fatal(r, e)
	}
	p := r.Contacts[0]
	if !p.Transverse || !p.Splittable || p.AtEnd != ([2]bool{}) {
		t.Fatal(p)
	}
	for i, s := range [2]Solid{a, b} {
		anchor := p.Anchors[i]
		out, h, e := s.SplitSplineEdgeExact(ctx, anchor.Edge, anchor.Parameter, SplineRectangleEmbeddingOptions{}.EdgeAgreement)
		if e != nil || out.vertices[h.Vertex] != (geom.Vec3{X: .5}) {
			t.Fatal(i, h, e)
		}
	}
}

func TestIntersectSplineEdgesRationalAndReversed(t *testing.T) {
	ctx := context.Background()
	// A(t)=(3t,0,9t²), B(s)=(1,s,1). The point is exactly dyadic,
	// but the original first edge parameter t=1/3 cannot be split.
	curve, e := geom.NewBSpline(2, []geom.Vec3{{}, {X: 1.5}, {X: 3, Z: 9}}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	line, e := geom.NewBSpline(1, []geom.Vec3{{X: 1, Y: -1, Z: 1}, {X: 1, Y: 1, Z: 1}}, []float64{2, 2, 4, 4}, nil)
	if e != nil {
		t.Fatal(e)
	}
	a := Solid{edges: []parametric.Edge{{Curve: curve, Range: parametric.Interval{Max: 1}}}}
	b := Solid{edges: []parametric.Edge{{Curve: line, Range: parametric.Interval{Min: 2, Max: 4}}}}
	for i, pair := range [2][2]Solid{{a, b}, {b, a}} {
		r, e := IntersectSplineEdges(ctx, pair[0], 0, pair[1], 0, geom.ExactCurveSegmentOptions{})
		if e != nil || !r.Complete || len(r.Contacts) != 1 {
			t.Fatal(r, e)
		}
		p := r.Contacts[0]
		if p.Splittable || !p.Transverse || p.Parameters[i].Cmp(big.NewRat(1, 3)) != 0 || p.Parameters[i^1].Cmp(big.NewRat(3, 1)) != 0 {
			t.Fatal(p)
		}
	}
}
