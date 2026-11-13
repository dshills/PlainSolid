package trimmed

import (
	"context"
	"fmt"

	"plainsolid/geom"
)

// EdgeAnchor selects an edge on the original face and a strictly interior
// parameter in the original model curve's parameter units.
type EdgeAnchor struct {
	Edge      int
	Parameter float64
}

// AnchoredFaceSplit records both shared edge splits and the new face path.
type AnchoredFaceSplit struct {
	Start, End EdgeSplit
	Path       FacePathSplit
}

// SplitFaceAcrossEdges first reconciles the two cut endpoints with all
// incident faces, then creates the open cut on the selected face. Endpoints
// from makePath are in that face's UV chart. It must return a connected path
// from the first to second endpoint. The receiver is unchanged on any error.
// Only distinct straight outer edge uses of a hole-free face are supported.
func (s Solid) SplitFaceAcrossEdges(ctx context.Context, face int, start, end EdgeAnchor, makePath func(geom.Vec2, geom.Vec2) ([]CutSegment, error)) (Solid, AnchoredFaceSplit, error) {
	return s.splitFaceAcrossEdges(ctx, face, start, end, makePath, Solid.SplitEdge)
}

func (s Solid) splitFaceAcrossEdges(ctx context.Context, face int, start, end EdgeAnchor,
	makePath func(geom.Vec2, geom.Vec2) ([]CutSegment, error), split func(Solid, int, float64) (Solid, EdgeSplit, error)) (Solid, AnchoredFaceSplit, error) {
	var empty AnchoredFaceSplit
	if ctx == nil || makePath == nil || face < 0 || face >= len(s.faces) || start.Edge == end.Edge {
		return Solid{}, empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, empty, e
	}
	f := s.faces[face]
	if len(f.Holes) > 0 {
		return Solid{}, empty, fmt.Errorf("trimmed: anchored split requires a hole-free face")
	}
	count := func(id int) int {
		n := 0
		for _, u := range f.Outer {
			if u.Edge == id && u.Curve == nil {
				n++
			}
		}
		return n
	}
	if count(start.Edge) != 1 || count(end.Edge) != 1 {
		return Solid{}, empty, fmt.Errorf("trimmed: anchors must belong to distinct single-use straight outer edges")
	}
	q, a, e := split(s, start.Edge, start.Parameter)
	if e != nil {
		return Solid{}, empty, e
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, empty, e
	}
	q, b, e := split(q, end.Edge, end.Parameter)
	if e != nil {
		return Solid{}, empty, e
	}
	f = q.faces[face]
	which := func(id int) (int, error) {
		n := -1
		for i, u := range f.Outer {
			v := q.edges[u.Edge].Start
			if u.Reversed {
				v = q.edges[u.Edge].End
			}
			if v == id {
				if n >= 0 {
					return -1, fmt.Errorf("trimmed: ambiguous anchored vertex on face")
				}
				n = i
			}
		}
		if n < 0 {
			return -1, fmt.Errorf("trimmed: missing anchored vertex on face")
		}
		return n, nil
	}
	i, e := which(a.Vertex)
	if e != nil {
		return Solid{}, empty, e
	}
	j, e := which(b.Vertex)
	if e != nil {
		return Solid{}, empty, e
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, empty, e
	}
	p, e := makePath(f.Outer[i].UVStart, f.Outer[j].UVStart)
	if e != nil {
		return Solid{}, empty, e
	}
	r, h, e := q.SplitFacePath(ctx, face, i, j, p)
	if e != nil {
		return Solid{}, empty, e
	}
	return r, AnchoredFaceSplit{a, b, h}, nil
}
