package step

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
)

// Private immutable proof owns exact vertices, directed source identities and
// outward halfspaces; both point and line queries consume it without mutation.
type hierarchyConvexPlanarProof struct {
	vertices                    [][3]*big.Rat
	edges                       []brep.Edge
	planes                      []hierarchyPlanarFace
	sourceVertices, sourceEdges []int64
}

func prepareHierarchyConvexPlanar(ctx context.Context, p IndependentExactMaterial, faceIDs []int64, a *hierarchyPointArithmetic, tick func() error) (*hierarchyConvexPlanarProof, string, error) {
	unsupported := func() (*hierarchyConvexPlanarProof, string, error) {
		if a.err != nil {
			return nil, "", a.err
		}
		return nil, "planar leaf requires exact convex hole-free outward faces", nil
	}
	vertices, edges, faces := p.Planar.Solid.Vertices(), p.Planar.Solid.Edges(), p.Planar.Solid.Faces()
	if len(faces) != len(faceIDs) {
		return nil, "", geom.ErrInvalid
	}
	v := make([][3]*big.Rat, len(vertices))
	for i, q := range vertices {
		v[i] = mappedRatVec(q)
	}
	vs := make([]int64, len(v))
	es := make([]int64, len(edges))
	var planes []hierarchyPlanarFace
	dot, sub, cross, equal := a.planarDot, a.planarSub, a.planarCross, planarEqual
	cache := map[int64]linearEdge{}
	for fi, face := range faces {
		if e := tick(); e != nil {
			return nil, "", e
		}
		if len(face.Holes) > 0 || len(face.Boundary) < 3 {
			return unsupported()
		}
		fid := faceIDs[fi]
		fr, e := simple(p.Source, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return nil, "", e
		}
		bounds, e := listRefs(fr.Args[1], 1)
		if e != nil || len(bounds) != 1 {
			return unsupported()
		}
		en := p.Source.Entities[bounds[0]]
		if len(en.Records) != 1 {
			return nil, "", geom.ErrInvalid
		}
		br := en.Records[0]
		if len(br.Args) != 3 {
			return nil, "", geom.ErrInvalid
		}
		oriented, e := boolean(br.Args[2])
		if e != nil {
			return nil, "", e
		}
		loop, e := edgeLoop(p.Source, br.Args[1].Ref, cache)
		if e != nil {
			return nil, "", e
		}
		if len(loop.points) != len(face.Boundary) {
			return nil, "", geom.ErrInvalid
		}
		indices := make([]int, len(loop.points))
		for j, pid := range loop.points {
			pr, e := simple(p.Source, pid, "CARTESIAN_POINT", 2)
			if e != nil || len(pr.Args[1].Items) != 3 {
				return nil, "", geom.ErrInvalid
			}
			var exact [3]*big.Rat
			for axis := range exact {
				exact[axis], e = liftSourceRat(pr.Args[1].Items[axis])
				if e != nil {
					return nil, "", e
				}
			}
			found := -1
			for i, q := range v {
				if e = tick(); e != nil {
					return nil, "", e
				}
				if equal(exact, q) {
					if found >= 0 {
						return nil, "", geom.ErrInvalid
					}
					found = i
				}
			}
			if found < 0 {
				return unsupported()
			}
			indices[j] = found
			if vs[found] != 0 && vs[found] != loop.vertices[j] {
				return nil, "", geom.ErrInvalid
			}
			vs[found] = loop.vertices[j]
		}
		directed := map[[2]int]int64{}
		for j, start := range indices {
			end := indices[(j+1)%len(indices)]
			if !oriented {
				start, end = end, start
			}
			directed[[2]int{start, end}] = loop.edges[j]
		}
		ids := make([]int, len(face.Boundary))
		for j, use := range face.Boundary {
			ed := edges[use.Edge]
			start, end := ed.Start, ed.End
			if use.Reversed {
				start, end = end, start
			}
			source := directed[[2]int{start, end}]
			if source == 0 {
				return nil, "", geom.ErrInvalid
			}
			if es[use.Edge] != 0 && es[use.Edge] != source {
				return nil, "", geom.ErrInvalid
			}
			es[use.Edge] = source
			ids[j] = start
		}
		n := cross(sub(v[ids[1]], v[ids[0]]), sub(v[ids[2]], v[ids[1]]))
		if dot(n, n).Sign() == 0 {
			return unsupported()
		}

		support, e := simple(p.Source, fr.Args[2].Ref, "PLANE", 2)
		if e != nil {
			return nil, "", e
		}
		sourceFrame, e := mappedExactFrame(p.Source, support.Args[1].Ref)
		if e != nil {
			return nil, "", e
		}
		sourceNormal := mappedRatVec(sourceFrame.ZAxis())
		sourceOrigin := mappedRatVec(sourceFrame.Origin())
		sense, e := boolean(fr.Args[3])
		if e != nil {
			return nil, "", e
		}
		if !sense {
			for i := range sourceNormal {
				sourceNormal[i] = new(big.Rat).Neg(sourceNormal[i])
			}
		}
		parallel := cross(n, sourceNormal)
		if dot(parallel, parallel).Sign() != 0 || dot(n, sourceNormal).Sign() <= 0 {
			return unsupported()
		}
		for _, id := range ids {
			if dot(sourceNormal, sub(v[id], sourceOrigin)).Sign() != 0 {
				return unsupported()
			}
		}
		c := new(big.Rat).Neg(dot(n, v[ids[0]]))
		negative := false
		for _, q := range v {
			if e = tick(); e != nil {
				return nil, "", e
			}
			sign := a.add(dot(n, q), c).Sign()
			if sign > 0 {
				return unsupported()
			}
			negative = negative || sign < 0
		}
		if !negative {
			return unsupported()
		}
		for j, id := range ids {
			if a.add(dot(n, v[id]), c).Sign() != 0 {
				return unsupported()
			}
			turn := cross(sub(v[ids[(j+1)%len(ids)]], v[id]), sub(v[ids[(j+2)%len(ids)]], v[ids[(j+1)%len(ids)]]))
			if dot(turn, n).Sign() <= 0 {
				return unsupported()
			}
		}
		planes = append(planes, hierarchyPlanarFace{n, c, fid})
	}
	if a.err != nil {
		return nil, "", a.err
	}
	return &hierarchyConvexPlanarProof{v, edges, planes, vs, es}, "", nil
}
func (a *hierarchyPointArithmetic) planarDot(x, y [3]*big.Rat) *big.Rat {
	q := new(big.Rat)
	for i := range x {
		q = a.add(q, a.mul(x[i], y[i]))
	}
	return q
}
func (a *hierarchyPointArithmetic) planarSub(x, y [3]*big.Rat) (q [3]*big.Rat) {
	for i := range q {
		q[i] = a.sub(x[i], y[i])
	}
	return
}
func (a *hierarchyPointArithmetic) planarCross(x, y [3]*big.Rat) (q [3]*big.Rat) {
	for i := range q {
		q[i] = a.sub(a.mul(x[(i+1)%3], y[(i+2)%3]), a.mul(x[(i+2)%3], y[(i+1)%3]))
	}
	return
}
func planarEqual(x, y [3]*big.Rat) bool {
	for i := range x {
		if x[i].Cmp(y[i]) != 0 {
			return false
		}
	}
	return true
}
func (p *hierarchyConvexPlanarProof) classify(point [3]*big.Rat, a *hierarchyPointArithmetic, tick func() error) (AnalyticPointLocation, error) {
	boundary := false
	for _, plane := range p.planes {
		if e := tick(); e != nil {
			return "", e
		}
		s := a.add(a.planarDot(plane.n, point), plane.c).Sign()
		if a.err != nil {
			return "", a.err
		}
		if s > 0 {
			return AnalyticPointOutside, nil
		}
		boundary = boundary || s == 0
	}
	if boundary {
		return AnalyticPointBoundary, nil
	}
	return AnalyticPointInside, nil
}
func (p *hierarchyConvexPlanarProof) contacts(point [3]*big.Rat, a *hierarchyPointArithmetic, tick func() error) (faces, edges, vertices []int64, err error) {
	for _, plane := range p.planes {
		if err = tick(); err != nil {
			return
		}
		if a.add(a.planarDot(plane.n, point), plane.c).Sign() == 0 {
			faces = append(faces, plane.id)
		}
	}
	for i, q := range p.vertices {
		if err = tick(); err != nil {
			return
		}
		if planarEqual(point, q) {
			vertices = append(vertices, p.sourceVertices[i])
		}
	}
	for i, ed := range p.edges {
		if err = tick(); err != nil {
			return
		}
		ab := a.planarSub(p.vertices[ed.End], p.vertices[ed.Start])
		ap := a.planarSub(point, p.vertices[ed.Start])
		cross := a.planarCross(ab, ap)
		if a.planarDot(cross, cross).Sign() != 0 {
			continue
		}
		z := a.planarDot(ap, ab)
		if z.Sign() >= 0 && z.Cmp(a.planarDot(ab, ab)) <= 0 {
			edges = append(edges, p.sourceEdges[i])
		}
	}
	err = a.err
	return
}
