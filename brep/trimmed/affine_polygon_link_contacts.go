package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func (r *ConvexAffinePolygonCrossing) polygonCoincidentVertices(ctx context.Context, s Solid, limit int) ([]ConvexAffinePolygonVertexContact, error) {
	points := map[geom.Vec3][]int{}
	for vi, p := range s.vertices {
		points[p] = append(points[p], vi)
	}
	out := []ConvexAffinePolygonVertexContact{}
	// First occurrence order makes certificates stable, independent of maps.
	for vi, p := range s.vertices {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		ids := points[p]
		if len(ids) == 1 || ids[0] != vi {
			continue
		}
		if len(out) >= limit {
			return nil, geom.ErrConvergence
		}
		if len(ids) != 2 {
			return nil, geom.ErrDomain
		}
		q := affineMaterialXYZ(p)
		node := -1
		for i, n := range r.nodes {
			if len(n.Owners) == 2 && n.Point[0].Cmp(q[0]) == 0 && n.Point[1].Cmp(q[1]) == 0 && n.Point[2].Cmp(q[2]) == 0 {
				degree := 0
				for _, e := range r.source.edges {
					if e.Vertices[0] == i || e.Vertices[1] == i {
						degree++
					}
				}
				if degree == 4 {
					node = i
				}
				break
			}
		}
		if node < 0 {
			return nil, geom.ErrDomain
		}
		proof := ConvexAffinePolygonVertexContact{SectionVertex: node, Point: q, Vertices: append([]int(nil), ids...), IncidentFaces: make([][]int, len(ids))}
		for at, id := range ids {
			for fi, f := range s.faces {
				for _, u := range f.Outer {
					e := s.edges[u.Edge]
					if e.Start == id || e.End == id {
						proof.IncidentFaces[at] = append(proof.IncidentFaces[at], fi)
						break
					}
				}
			}
			if len(proof.IncidentFaces[at]) < 3 {
				return nil, geom.ErrDomain
			}
		}
		out = append(out, proof)
	}
	return out, nil
}

func polygonNativeLoop(f Face) ([][2]*big.Rat, error) {
	out := make([][2]*big.Rat, len(f.Outer))
	for i, u := range f.Outer {
		out[i] = [2]*big.Rat{ratInset(u.UVStart.X), ratInset(u.UVStart.Y)}
		if u.Curve != nil {
			q, ok := u.Curve.(ExactQuadraticUV)
			if !ok {
				return nil, geom.ErrDomain
			}
			ends, _ := q.Endpoints()
			out[i] = ends[0]
		}
	}
	return out, nil
}
func polygonFaceVertexIDs(s Solid, f Face) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, u := range f.Outer {
		e := s.edges[u.Edge]
		for _, i := range []int{e.Start, e.End} {
			if !seen[i] {
				seen[i] = true
				out = append(out, i)
			}
		}
	}
	return out
}
func polygonModelEquals(p [3]*big.Rat, q geom.Vec3) bool {
	exact := affineMaterialXYZ(q)
	for k := range p {
		if p[k].Cmp(exact[k]) != 0 {
			return false
		}
	}
	return true
}

// Complete pairwise triangle contact establishes that indexed incidence is the
// whole boundary contact, except the declared isolated vertex identifications.
func (r *ConvexAffinePolygonCrossing) verifyPolygonContactTopology(ctx context.Context, b *nativePrismAffineBudget, s Solid, sources []AffineShellMaterialFace, contacts []ConvexAffinePolygonVertexContact) (int, error) {
	if len(sources) != len(s.faces) {
		return 0, geom.ErrDomain
	}
	expected, e := r.polygonCoincidentVertices(ctx, s, 64)
	if e != nil {
		return 0, e
	}
	if len(expected) != len(contacts) {
		return 0, geom.ErrDomain
	}
	for i, x := range expected {
		y := contacts[i]
		if x.SectionVertex != y.SectionVertex || len(x.Vertices) != len(y.Vertices) || len(x.IncidentFaces) != len(y.IncidentFaces) {
			return 0, geom.ErrDomain
		}
		for k := range x.Point {
			if y.Point[k] == nil || x.Point[k].Cmp(y.Point[k]) != 0 {
				return 0, geom.ErrDomain
			}
		}
		for j, v := range x.Vertices {
			if y.Vertices[j] != v || len(x.IncidentFaces[j]) != len(y.IncidentFaces[j]) {
				return 0, geom.ErrDomain
			}
			for k, f := range x.IncidentFaces[j] {
				if y.IncidentFaces[j][k] != f {
					return 0, geom.ErrDomain
				}
			}
		}
	}
	patches := make([]intersect.Patch, len(s.faces))
	loops := make([][][2]*big.Rat, len(s.faces))
	ids := make([][]int, len(s.faces))
	for fi, f := range s.faces {
		h := sources[fi]
		var e error
		patches[fi], e = algebraicShellTracePatch(f, r.shells[h.Owner].queries[h.Face].chart.Domain())
		if e != nil {
			return 0, e
		}
		loops[fi], e = polygonNativeLoop(f)
		if e != nil {
			return 0, e
		}
		ids[fi] = polygonFaceVertexIDs(s, f)
	}
	count := 0
	for i := range s.faces {
		for j := i + 1; j < len(s.faces); j++ {
			if b.work >= b.max {
				return 0, geom.ErrConvergence
			}
			pair, e := intersect.IntersectConvexAffinePatches(ctx, patches[i], patches[j], [2][][2]*big.Rat{loops[i], loops[j]}, geom.SurfacePatchOptions{}, intersect.AffinePolygonContactOptions{MaxEdges: 6, MaxOperations: b.max - b.work, MaxCoefficientBits: b.bits})
			if e != nil {
				return 0, e
			}
			c := pair.Contact()
			b.work += c.ExactOperations()
			kind, e := c.Kind()
			if e != nil {
				return 0, e
			}
			vertices, e := c.Vertices()
			if e != nil {
				return 0, e
			}
			shared := []int{}
			for _, a := range ids[i] {
				for _, d := range ids[j] {
					if a == d {
						shared = append(shared, a)
					}
				}
			}
			sharedEdges := []int{}
			for _, a := range s.faces[i].Outer {
				for _, d := range s.faces[j].Outer {
					if a.Edge == d.Edge {
						sharedEdges = append(sharedEdges, a.Edge)
					}
				}
			}
			switch kind {
			case intersect.AffinePatchContactEmpty:
				if len(shared) != 0 || len(sharedEdges) != 0 {
					return 0, geom.ErrDomain
				}
			case intersect.AffinePatchContactPoint:
				if len(vertices) != 1 || len(sharedEdges) != 0 || len(shared) > 1 {
					return 0, geom.ErrDomain
				}
				a, d := -1, -1
				for _, v := range ids[i] {
					if polygonModelEquals(vertices[0].Point, s.vertices[v]) {
						a = v
					}
				}
				for _, v := range ids[j] {
					if polygonModelEquals(vertices[0].Point, s.vertices[v]) {
						d = v
					}
				}
				if a < 0 || d < 0 {
					return 0, geom.ErrDomain
				}
				if a == d {
					if len(shared) != 1 || shared[0] != a {
						return 0, geom.ErrDomain
					}
				} else {
					if len(shared) != 0 {
						return 0, geom.ErrDomain
					}
					proved := false
					for _, contact := range contacts {
						if len(contact.Vertices) == 2 && ((contact.Vertices[0] == a && contact.Vertices[1] == d) || (contact.Vertices[0] == d && contact.Vertices[1] == a)) {
							proved = true
							break
						}
					}
					if !proved {
						return 0, geom.ErrDomain
					}
				}
			case intersect.AffinePatchContactSegment:
				if len(vertices) != 2 || len(sharedEdges) != 1 || len(shared) != 2 {
					return 0, geom.ErrDomain
				}
				edge := s.edges[sharedEdges[0]]
				if !((polygonModelEquals(vertices[0].Point, s.vertices[edge.Start]) && polygonModelEquals(vertices[1].Point, s.vertices[edge.End])) || (polygonModelEquals(vertices[1].Point, s.vertices[edge.Start]) && polygonModelEquals(vertices[0].Point, s.vertices[edge.End]))) {
					return 0, geom.ErrDomain
				}
			default:
				return 0, geom.ErrDomain
			}
			count++
		}
	}
	if e := ctx.Err(); e != nil {
		return 0, e
	}
	return count, b.err
}
