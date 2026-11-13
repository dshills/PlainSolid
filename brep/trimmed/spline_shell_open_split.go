package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineTubeFaceCut describes one open path crossing two distinct original
// sides of a rectangular spline face. Segments are in traversal order in a
// single recipe; Chart selects the face's chart on that recipe.
type SplineTubeFaceCut struct {
	Face, Chart int
	Segments    []intersect.TubeSegment
	Start, End  SplineTubeEndpoint
}

// SplineTubeWireSplit preserves the original edge and face ancestry while
// recording all cut sections in input order. Each original boundary edge
// crossed by the wire is split exactly once, even though two faces use it.
type SplineTubeWireSplit struct {
	OriginalEdges []int
	EdgeSplits    []EdgeSplit
	Faces         []FacePathSplit
}

// SplitSplineTubeWire applies one closed cycle of open spline tube paths on
// different faces of an existing spline shell. Each crossed original edge
// must have two exact endpoint proposals from its incident cut faces, and
// the face paths must form a single cycle. Each face is split once. This
// operation changes the boundary graph while preserving the original solid.
// Input branches must not cross other trims between the matched endpoints.
func (s Solid) SplitSplineTubeWire(ctx context.Context, cuts []SplineTubeFaceCut, o geom.TubeEndpointEdgeOptions) (Solid, SplineTubeWireSplit, error) {
	var out SplineTubeWireSplit
	if ctx == nil || len(cuts) < 2 || len(cuts) > 64 {
		return Solid{}, out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, out, e
	}
	type cutData struct {
		anchors [2]SplineTubeAnchor
		path    []CutSegment
	}
	data := make([]cutData, len(cuts))
	type edgeUse struct{ cut, end int }
	uses := map[int][]edgeUse{}
	seenFace := map[int]bool{}
	segmentCount := 0
	for i, c := range cuts {
		if e := ctx.Err(); e != nil {
			return Solid{}, out, e
		}
		if c.Face < 0 || c.Face >= len(s.faces) || c.Chart < 0 || c.Chart > 1 || seenFace[c.Face] || len(c.Segments) == 0 || len(c.Segments) > 64 {
			return Solid{}, out, geom.ErrInvalid
		}
		seenFace[c.Face] = true
		segmentCount += len(c.Segments)
		if segmentCount > 256 {
			return Solid{}, out, parametric.ErrBudget
		}
		var recipe intersect.TubeRecipe
		previous := -1
		for j, s := range c.Segments {
			p := s.TrimA()
			if c.Chart == 1 {
				p = s.TrimB()
			}
			key, index, _ := p.Recipe()
			if j > 0 && (key != recipe || index != previous+1) || j == 0 && key.Data().Closed {
				return Solid{}, out, geom.ErrInvalid
			}
			recipe, previous = key, index
			a, e := p.EvalUV(0)
			if e != nil {
				return Solid{}, out, e
			}
			b, e := p.EvalUV(1)
			if e != nil {
				return Solid{}, out, e
			}
			data[i].path = append(data[i].path, CutSegment{Start: a, End: b, Curve: p})
		}
		for end, endpoint := range [2]SplineTubeEndpoint{c.Start, c.End} {
			n := 0
			if end == 1 {
				n = len(c.Segments) - 1
			}
			a, e := s.CertifySplineTubeAnchor(ctx, c.Face, c.Segments[n], end, c.Chart, endpoint.Side, endpoint.Hint, o)
			if e != nil {
				return Solid{}, out, e
			}
			data[i].anchors[end] = a
			id := a.Anchor.Edge
			if len(uses[id]) == 0 {
				out.OriginalEdges = append(out.OriginalEdges, id)
			}
			uses[id] = append(uses[id], edgeUse{i, end})
		}
		if data[i].anchors[0].Anchor.Edge == data[i].anchors[1].Anchor.Edge {
			return Solid{}, SplineTubeWireSplit{}, geom.ErrInvalid
		}
	}
	adj := make([][]int, len(cuts))
	for _, id := range out.OriginalEdges {
		u := uses[id]
		if len(u) != 2 {
			return Solid{}, SplineTubeWireSplit{}, fmt.Errorf("trimmed: section wire does not have two uses at edge %d: %w", id, geom.ErrConvergence)
		}
		a, b := data[u[0].cut].anchors[u[0].end], data[u[1].cut].anchors[u[1].end]
		if a.Anchor != b.Anchor || !exactPointCoordinatesEqual(a.Proof.Point, b.Proof.Point) {
			return Solid{}, SplineTubeWireSplit{}, geom.ErrConvergence
		}
		adj[u[0].cut] = append(adj[u[0].cut], u[1].cut)
		adj[u[1].cut] = append(adj[u[1].cut], u[0].cut)
	}
	stack := []int{0}
	seen := make(map[int]bool, len(cuts))
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !seen[i] {
			seen[i] = true
			stack = append(stack, adj[i]...)
		}
	}
	if len(seen) != len(cuts) {
		return Solid{}, SplineTubeWireSplit{}, fmt.Errorf("trimmed: section paths do not form one cycle: %w", geom.ErrConvergence)
	}
	q := s
	vertices := map[int]int{}
	for _, id := range out.OriginalEdges {
		u := uses[id][0]
		a := data[u.cut].anchors[u.end].Anchor
		var split EdgeSplit
		var e error
		q, split, e = q.SplitSplineEdgeExact(ctx, id, a.Parameter, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
		if e != nil {
			return Solid{}, SplineTubeWireSplit{}, e
		}
		vertices[id] = split.Vertex
		out.EdgeSplits = append(out.EdgeSplits, split)
	}
	for i, c := range cuts {
		var indices [2]int
		for j, a := range data[i].anchors {
			indices[j] = q.faceVertexUse(c.Face, vertices[a.Anchor.Edge])
			if indices[j] < 0 {
				return Solid{}, SplineTubeWireSplit{}, geom.ErrConvergence
			}
		}
		var f FacePathSplit
		var e error
		q, f, e = q.SplitFacePath(ctx, c.Face, indices[0], indices[1], data[i].path)
		if e != nil {
			return Solid{}, SplineTubeWireSplit{}, e
		}
		out.Faces = append(out.Faces, f)
	}
	return q, out, nil
}

func (s Solid) faceVertexUse(face, vertex int) int {
	for i, u := range s.faces[face].Outer {
		e := s.edges[u.Edge]
		if !u.Reversed && e.Start == vertex || u.Reversed && e.End == vertex {
			return i
		}
	}
	return -1
}

func exactPointCoordinatesEqual(a, b [3]*big.Rat) bool {
	for i, x := range a {
		if x == nil || b[i] == nil || x.Cmp(b[i]) != 0 {
			return false
		}
	}
	return true
}
