// Package frommesh converts closed indexed triangle solids to exact same-domain
// planar BReps. It never welds coordinates or approximates curved surfaces.
package frommesh

import (
	"context"
	"errors"
	"fmt"
	"plainsolid/brep"
	"plainsolid/geom"
	"plainsolid/heal/unify"
	"plainsolid/mesh"
)

var ErrInvalid = errors.New("frommesh: invalid indexed solid or options")
var ErrLimit = errors.New("frommesh: input size limit exceeded")

// Options bounds input allocation independently of unification and its exact
// embedding/orientation validation. Defaults: 16384 vertices,8192 triangles.
// Unify.MaxFaces is defaulted to MaxTriangles because every triangle initially
// forms one face; other unification defaults remain unchanged.
type Options struct {
	MaxVertices, MaxTriangles int
	Unify                     unify.Options
}

// Result maps every input triangle to one unified polygon face. FaceTriangles
// lists contributing original triangles in ascending order. VertexMap is -1
// for unused or removed collinear vertices. Geometry uses only input coordinates.
type Result struct {
	Solid         brep.Solid
	TriangleFaces []int
	FaceTriangles [][]int
	VertexMap     []int
	RemovedEdges  int
}

// Convert requires closed oriented embedded manifold triangles, including inward
// cavity shells where appropriate. Open meshes, self intersections, inconsistent
// normals and ambiguous same-domain junctions fail explicitly. Empty input is
// accepted. Unreferenced vertices are dropped but never geometrically welded.
func Convert(ctx context.Context, m mesh.Mesh, o Options) (Result, error) {
	empty := Result{}
	if ctx == nil || o.MaxVertices < 0 || o.MaxTriangles < 0 {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxVertices == 0 {
		o.MaxVertices = 16384
	}
	if o.MaxTriangles == 0 {
		o.MaxTriangles = 8192
	}
	if len(m.Vertices) > o.MaxVertices || len(m.Triangles) > o.MaxTriangles {
		return empty, ErrLimit
	}
	if e := m.ValidateSolid(); e != nil {
		return empty, fmt.Errorf("%w: %v", ErrInvalid, e)
	}
	if o.Unify.MaxFaces == 0 {
		o.Unify.MaxFaces = o.MaxTriangles
	}
	if o.Unify.MaxTriangles == 0 {
		o.Unify.MaxTriangles = o.MaxTriangles
	}
	remap := make([]int, len(m.Vertices))
	for i := range remap {
		remap[i] = -1
	}
	var vertices []geom.Vec3
	for _, t := range m.Triangles {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		for _, i := range t {
			if remap[i] < 0 {
				remap[i] = len(vertices)
				vertices = append(vertices, m.Vertices[i])
			}
		}
	}
	var edges []brep.Edge
	faces := make([]brep.Face, len(m.Triangles))
	lookup := map[[2]int]int{}
	for i, t := range m.Triangles {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		for j, a := range t {
			b := t[(j+1)%3]
			a, b = remap[a], remap[b]
			reverse := a > b
			if reverse {
				a, b = b, a
			}
			key := [2]int{a, b}
			index, ok := lookup[key]
			if !ok {
				index = len(edges)
				lookup[key] = index
				edges = append(edges, brep.Edge{Start: a, End: b})
			}
			faces[i].Boundary = append(faces[i].Boundary, brep.EdgeUse{Edge: index, Reversed: reverse})
		}
	}
	s, e := brep.NewSolid(vertices, edges, faces)
	if e != nil {
		return empty, fmt.Errorf("%w: %v", ErrInvalid, e)
	}
	r, e := unify.Unify(ctx, s, o.Unify)
	if e != nil {
		return empty, e
	}
	for i, x := range remap {
		if x >= 0 {
			remap[i] = r.VertexMap[x]
		}
	}
	return Result{r.Solid, r.OldToNew, r.NewToOld, remap, r.RemovedEdges}, nil
}
