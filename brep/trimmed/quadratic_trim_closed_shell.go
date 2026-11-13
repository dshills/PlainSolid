package trimmed

import (
	"context"
	"plainsolid/geom"
)

// CertifiedClosedQuadraticShell adds complete boundary ownership, connected
// closed opposite edge incidence and manifold vertex links to the collection
// distance certificate. Closed is a topological statement: self-intersection,
// geometric embedding and a positive material volume remain separate questions.
type CertifiedClosedQuadraticShell struct {
	CertifiedQuadraticTrimCollection
}

// StitchClosedQuadraticShellCertified requires every original trim exactly once
// among the declared seam pairs and explicit endpoint junction evidence. The
// assembled display mesh must be connected, closed and consistently oriented,
// and each vertex must have one manifold cyclic triangle link. It preserves
// all collection budgets/ancestry and returns no partial shell on failure.
// No rounded mesh or smooth source embedding is inferred from topology alone.
func StitchClosedQuadraticShellCertified(ctx context.Context, faces []QuadraticTrimStitchFace, seams []QuadraticTrimStitchSeam, junctions [][]QuadraticTrimSeamEndpoint, o QuadraticTrimFaceJoinOptions) (CertifiedClosedQuadraticShell, error) {
	empty := CertifiedClosedQuadraticShell{}
	if ctx == nil || len(faces) < 2 || len(faces) > 16 || len(seams) > 32 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	sourceOptions, e := o.Face.Boundary.Source.WithDefaults()
	if e != nil {
		return empty, e
	}
	ownership := make([][]int, len(faces))
	for i, f := range faces {
		if len(f.Outer) < 2 || len(f.Outer) > sourceOptions.MaxEdges {
			return empty, geom.ErrConvergence
		}
		ownership[i] = make([]int, len(f.Outer))
	}
	for _, s := range seams {
		for _, use := range [][2]int{{s.FaceA, s.TrimA}, {s.FaceB, s.TrimB}} {
			if use[0] < 0 || use[0] >= len(faces) || use[1] < 0 || use[1] >= len(ownership[use[0]]) {
				return empty, geom.ErrInvalid
			}
			ownership[use[0]][use[1]]++
		}
	}
	for _, face := range ownership {
		for _, n := range face {
			if n != 1 {
				return empty, geom.ErrInvalid
			}
		}
	}
	result, e := StitchQuadraticTrimCollectionWithJunctionsCertified(ctx, faces, seams, junctions, o)
	if e != nil {
		return empty, e
	}
	if e := result.Mesh.ValidateSolid(); e != nil {
		return empty, e
	}
	vertices := make([]int, len(result.Mesh.Vertices))
	for i := range vertices {
		vertices[i] = i
	}
	if e := validateQuadraticJunctionFans(result.Mesh, vertices); e != nil {
		return empty, e
	}
	if len(vertices) == 0 {
		return empty, geom.ErrConvergence
	}
	adjacency := make([][]int, len(vertices))
	for _, t := range result.Mesh.Triangles {
		for j, a := range t {
			b := t[(j+1)%3]
			adjacency[a] = append(adjacency[a], b)
			adjacency[b] = append(adjacency[b], a)
		}
	}
	visited := make([]bool, len(vertices))
	visited[0] = true
	stack := []int{0}
	count := 0
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		a := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		for _, b := range adjacency[a] {
			if !visited[b] {
				visited[b] = true
				stack = append(stack, b)
			}
		}
	}
	if count != len(vertices) {
		return empty, geom.ErrConvergence
	}
	return CertifiedClosedQuadraticShell{result}, nil
}
