package trimmed

import (
	"context"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// QuadraticTrimSeamEndpoint identifies endpoint0 or1 in the canonical FaceA
// direction of an original seam declaration. Interior parameters are excluded.
type QuadraticTrimSeamEndpoint struct{ Seam, End int }

// StitchQuadraticTrimCollectionWithJunctionsCertified adds explicit endpoint
// equivalence groups to collection stitching. Each group must have exactly
// equal rational model points, no repeated seam, and no endpoint reused in a
// second group. No coordinate proximity or interior crossing implies identity.
// After welding, every junction's triangle link must be one connected simple
// path (boundary vertex) or cycle (interior vertex), with consistent edge uses.
// This proves a manifold topological fan, not geometric sector nonintersection
// or global embedding. Existing collection stitching still rejects junctions.
func StitchQuadraticTrimCollectionWithJunctionsCertified(ctx context.Context, faces []QuadraticTrimStitchFace, seams []QuadraticTrimStitchSeam, junctions [][]QuadraticTrimSeamEndpoint, o QuadraticTrimFaceJoinOptions) (CertifiedQuadraticTrimCollection, error) {
	return stitchQuadraticTrimCollection(ctx, faces, seams, junctions, o)
}

func validateQuadraticJunctionFans(m mesh.Mesh, vertices []int) error {
	type pair [2]int
	links := map[int]map[int][]int{}
	edges := map[int]map[pair]bool{}
	for _, id := range vertices {
		links[id] = map[int][]int{}
		edges[id] = map[pair]bool{}
	}
	for _, triangle := range m.Triangles {
		for j, id := range triangle {
			link, ok := links[id]
			if !ok {
				continue
			}
			a, b := triangle[(j+1)%3], triangle[(j+2)%3]
			key := pair{a, b}
			if a > b {
				key = pair{b, a}
			}
			if edges[id][key] {
				return geom.ErrConvergence
			}
			edges[id][key] = true
			link[a] = append(link[a], b)
			link[b] = append(link[b], a)
		}
	}
	for _, link := range links {
		if len(link) < 2 {
			return geom.ErrConvergence
		}
		ends, start := 0, -1
		for id, neighbors := range link {
			start = id
			if len(neighbors) == 1 {
				ends++
			} else if len(neighbors) != 2 {
				return geom.ErrConvergence
			}
		}
		if ends != 0 && ends != 2 {
			return geom.ErrConvergence
		}
		seen := map[int]bool{start: true}
		stack := []int{start}
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, next := range link[id] {
				if !seen[next] {
					seen[next] = true
					stack = append(stack, next)
				}
			}
		}
		if len(seen) != len(link) {
			return geom.ErrConvergence
		}
	}
	return nil
}
