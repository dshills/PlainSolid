package trimmed

import (
	"fmt"
	"math/big"
	"plainsolid/geom"
)

// ConvexAffinePolygonPinchError identifies multiple circular links at one exact
// model point within an edge-connected material component. It does not assert
// that duplicating the vertex would produce an embedded manifold. No solid is
// published. SectionVertex is -1 unless an original paired junction owns it.
type ConvexAffinePolygonPinchError struct {
	Component     int
	SectionVertex int
	Point         [3]*big.Rat
	ClosedFans    int
}

func (e *ConvexAffinePolygonPinchError) Error() string {
	return fmt.Sprintf("trimmed: polygon material component %d has %d closed fans at original section vertex %d: %v", e.Component, e.ClosedFans, e.SectionVertex, geom.ErrDomain)
}
func (e *ConvexAffinePolygonPinchError) Unwrap() error { return geom.ErrDomain }

// This combinatorial test uses exact lossless source endpoints, before Sew can
// merge anything by tolerance. Each incident face has exactly two neighbors at
// the point. More than one connected cycle is a pinched, not circular, link.
func polygonComponentLinks(b *nativePrismAffineBudget, r *ConvexAffinePolygonCrossing, faces []SewFace, component int) error {
	type key struct{ a, c geom.Vec3 }
	edges := map[key][]int{}
	vertices := []geom.Vec3{}
	incident := map[geom.Vec3][]int{}
	less := func(a, c geom.Vec3) bool {
		if a.X != c.X {
			return a.X < c.X
		}
		if a.Y != c.Y {
			return a.Y < c.Y
		}
		return a.Z < c.Z
	}
	for fi, f := range faces {
		seen := map[geom.Vec3]bool{}
		for _, u := range f.Face.Outer {
			edge := f.Edges[u.Edge]
			a, c := f.Vertices[edge.Start], f.Vertices[edge.End]
			if !less(a, c) {
				a, c = c, a
			}
			edges[key{a, c}] = append(edges[key{a, c}], fi)
			for _, p := range []geom.Vec3{a, c} {
				if !seen[p] {
					seen[p] = true
					if _, ok := incident[p]; !ok {
						vertices = append(vertices, p)
					}
					incident[p] = append(incident[p], fi)
				}
			}
		}
	}
	links := map[geom.Vec3]map[int][]int{}
	for _, p := range vertices {
		links[p] = map[int][]int{}
	}
	for edge, owners := range edges {
		if len(owners) != 2 || owners[0] == owners[1] {
			return geom.ErrDomain
		}
		for _, p := range []geom.Vec3{edge.a, edge.c} {
			links[p][owners[0]] = append(links[p][owners[0]], owners[1])
			links[p][owners[1]] = append(links[p][owners[1]], owners[0])
		}
	}
	for _, p := range vertices {
		if e := b.ctx.Err(); e != nil {
			return e
		}
		seen := map[int]bool{}
		fans := 0
		for _, fi := range incident[p] {
			if len(links[p][fi]) != 2 {
				return geom.ErrDomain
			}
			if seen[fi] {
				continue
			}
			fans++
			queue := []int{fi}
			seen[fi] = true
			for at := 0; at < len(queue); at++ {
				for _, next := range links[p][queue[at]] {
					if !seen[next] {
						seen[next] = true
						queue = append(queue, next)
					}
				}
			}
		}
		if fans > 1 {
			q := affineMaterialXYZ(p)
			node := -1
			for i, n := range r.nodes {
				if len(n.Owners) == 2 && n.Point[0].Cmp(q[0]) == 0 && n.Point[1].Cmp(q[1]) == 0 && n.Point[2].Cmp(q[2]) == 0 {
					node = i
					break
				}
			}
			return &ConvexAffinePolygonPinchError{Component: component, SectionVertex: node, Point: q, ClosedFans: fans}
		}
	}
	return nil
}
