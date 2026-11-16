package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// Check the display triangulation's entire native owner ledger independently
// of the lifting and stitching process, including through-hole edges.
func auditParabolaTwoCirclesMesh(ctx context.Context, s Solid, out CertifiedParabolaTwoCirclesComponentMesh) error {
	type owners struct {
		faces [2]int
		count int
	}
	uses := make(map[[2]int]owners)
	next := make([][]int, len(out.Mesh.Vertices))
	for i, t := range out.Mesh.Triangles {
		if err := ctx.Err(); err != nil {
			return err
		}
		for j, a := range t {
			b := t[(j+1)%3]
			key := parabolaCircleMeshEdgeKey(a, b)
			current := uses[key]
			if current.count == 2 {
				return geom.ErrConvergence
			}
			current.faces[current.count] = out.OriginalFace[i]
			current.count++
			uses[key] = current
			next[a] = append(next[a], b)
			next[b] = append(next[b], a)
		}
	}
	seen := make([]bool, len(next))
	queue := []int{0}
	seen[0] = true
	for i := 0; i < len(queue); i++ {
		for _, neighbor := range next[queue[i]] {
			if !seen[neighbor] {
				seen[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
	}
	if len(queue) != len(next) || len(next)-len(uses)+len(out.Mesh.Triangles) != s.Topology().EulerCharacteristic {
		return geom.ErrConvergence
	}
	nativeOwners := make([]owners, len(s.edges))
	for i, f := range s.faces {
		for _, loop := range append([][]Use{f.Outer}, f.Holes...) {
			for _, u := range loop {
				row := &nativeOwners[u.Edge]
				if row.count == 2 {
					return geom.ErrConvergence
				}
				row.faces[row.count] = i
				row.count++
			}
		}
	}
	represented := make(map[[2]int]bool)
	for i, edge := range out.OriginalEdges {
		native := s.edges[i]
		if edge.Edge != i || len(edge.Vertices) < 2 || len(edge.Vertices) != len(edge.Parameters) ||
			edge.Vertices[0] != out.OriginalVertices[native.Start] ||
			edge.Vertices[len(edge.Vertices)-1] != out.OriginalVertices[native.End] ||
			edge.Parameters[0].Cmp(ratInset(native.Range.Min)) != 0 ||
			edge.Parameters[len(edge.Parameters)-1].Cmp(ratInset(native.Range.Max)) != 0 {
			return geom.ErrConvergence
		}
		want := nativeOwners[i]
		if want.count != 2 || want.faces[0] == want.faces[1] {
			return geom.ErrConvergence
		}
		for k := 1; k < len(edge.Vertices); k++ {
			if edge.Parameters[k].Cmp(edge.Parameters[k-1]) <= 0 {
				return geom.ErrConvergence
			}
			key := parabolaCircleMeshEdgeKey(edge.Vertices[k-1], edge.Vertices[k])
			have := uses[key]
			if represented[key] || have.count != 2 ||
				parabolaCircleMeshEdgeKey(have.faces[0], have.faces[1]) != parabolaCircleMeshEdgeKey(want.faces[0], want.faces[1]) {
				return geom.ErrConvergence
			}
			represented[key] = true
		}
	}
	for key, have := range uses {
		if have.count != 2 || (have.faces[0] != have.faces[1]) != represented[key] {
			return geom.ErrConvergence
		}
	}
	return nil
}

func ownParabolaCircleMeshParameters(parameters []*big.Rat) []*big.Rat {
	owned := make([]*big.Rat, len(parameters))
	for i, q := range parameters {
		owned[i] = new(big.Rat).Set(q)
	}
	return owned
}
