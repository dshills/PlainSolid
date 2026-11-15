package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CircularSweepCapJoin retains the output identity of every original vertex,
// edge and face. Each input's coincident cap maps to -1. Matching original
// rim edges share one output identity; lower and upper wall faces stay owned
// by their separate original sweeps.
type CircularSweepCapJoin struct {
	Solid                  Solid
	Faces, Edges, Vertices [2][]int
	Caps                   [2]int
	SharedRimEdges         []int
}

// JoinExactCircularSweepCaps joins two exact model-Z sweeps with complete cap
// contact. Their directed profiles must match, allowing cyclic ring starts
// and different hole ordering. It does not infer new segmentation or snap
// near geometry. The complete original cap boundaries prove equal material.
func JoinExactCircularSweepCaps(ctx context.Context, a, b ExactCircularSweep, o ExactCircularLoopOptions) (CircularSweepCapJoin, error) {
	var zero CircularSweepCapJoin
	if ctx == nil {
		return zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	caps := [2]int{}
	switch {
	case a.Top == b.Bottom:
		caps[0] = 1
	case a.Bottom == b.Top:
		caps[1] = 1
	default:
		return zero, fmt.Errorf("trimmed: complete sweep cap join needs exactly abutting caps: %w", geom.ErrDomain)
	}
	h, err := ExtrudeExactDirectedCircularLoops(ctx, a.Outer, a.Holes, a.Bottom, a.Top, o)
	if err != nil {
		return zero, err
	}
	g, err := ExtrudeExactDirectedCircularLoops(ctx, b.Outer, b.Holes, b.Bottom, b.Top, o)
	if err != nil {
		return zero, err
	}
	n := len(h.vertices) / 2
	if len(g.vertices) != 2*n || len(a.Holes) != len(b.Holes) {
		return zero, fmt.Errorf("trimmed: original sweep cap segmentation differs: %w", geom.ErrDomain)
	}
	flat := func(q ExactCircularSweep) []ExactCircularLoopSegment {
		v := append([]ExactCircularLoopSegment(nil), q.Outer...)
		for _, ring := range q.Holes {
			v = append(v, ring...)
		}
		return v
	}
	ar, br := flat(a), flat(b)
	mapped := CircularSweepCapJoin{Caps: caps}
	for operand, s := range []Solid{h, g} {
		mapped.Faces[operand] = make([]int, len(s.faces))
		mapped.Edges[operand] = make([]int, len(s.edges))
		mapped.Vertices[operand] = make([]int, len(s.vertices))
	}
	vertices := h.Vertices()
	source := make(map[geom.Vec3]int, n)
	for i, p := range vertices {
		mapped.Vertices[0][i] = i
		if i/n == caps[0] {
			source[p] = i
		}
	}
	seen := map[int]bool{}
	for i, p := range g.vertices {
		if i/n != caps[1] {
			mapped.Vertices[1][i] = len(vertices)
			vertices = append(vertices, p)
			continue
		}
		id, ok := source[p]
		if !ok || seen[id] {
			return zero, fmt.Errorf("trimmed: original cap vertex sets differ: %w", geom.ErrDomain)
		}
		seen[id] = true
		mapped.Vertices[1][i] = id
	}
	edges := h.Edges()
	originalRims := make(map[[2]int]int, n)
	for i, e := range edges {
		mapped.Edges[0][i] = i
		if i/n == caps[0] {
			originalRims[[2]int{e.Start, e.End}] = i
		}
	}
	seen = map[int]bool{}
	for i, e := range g.edges {
		pair := [2]int{mapped.Vertices[1][e.Start], mapped.Vertices[1][e.End]}
		if i/n == caps[1] {
			j, ok := originalRims[pair]
			if !ok || seen[j] {
				return zero, fmt.Errorf("trimmed: original finite rim edges differ: %w", geom.ErrDomain)
			}
			x, y := ar[j%n], br[i%n]
			if x.Radius != y.Radius || x.Radius > 0 && x.Center != y.Center {
				return zero, fmt.Errorf("trimmed: original rim support differs: %w", geom.ErrDomain)
			}
			seen[j] = true
			mapped.Edges[1][i] = j
			mapped.SharedRimEdges = append(mapped.SharedRimEdges, j)
			continue
		}
		mapped.Edges[1][i] = len(edges)
		edges = append(edges, parametric.Edge{Start: pair[0], End: pair[1], Curve: e.Curve, Range: e.Range})
	}
	var faces []Face
	for operand, s := range []Solid{h, g} {
		for i, f := range s.Faces() {
			if i == caps[operand] {
				mapped.Faces[operand][i] = -1
				continue
			}
			for _, ring := range append([][]Use{f.Outer}, f.Holes...) {
				for j := range ring {
					ring[j].Edge = mapped.Edges[operand][ring[j].Edge]
				}
			}
			mapped.Faces[operand][i] = len(faces)
			faces = append(faces, f)
		}
	}
	out, err := NewSolidContext(ctx, vertices, edges, faces, h.tolerance)
	if err != nil {
		return zero, err
	}
	proof, err := out.ValidateEdgeAgreement(ctx, o.Edges)
	if err != nil {
		return zero, err
	}
	if !proof.Complete {
		return zero, fmt.Errorf("trimmed: cannot certify both original seam owners: %w", geom.ErrConvergence)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	mapped.Solid = out
	return mapped, nil
}
