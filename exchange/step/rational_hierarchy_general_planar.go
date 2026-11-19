package step

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"runtime"
)

// General proof retains finite original face loops. It is never substituted for
// a convex proof in existing halfspace-based APIs.
type hierarchyGeneralPlanarFace struct {
	plane    hierarchyPlanarFace
	loops    [][]int
	boundIDs []int64
	drop     int
}
type hierarchyGeneralPlanarProof struct {
	base  *hierarchyConvexPlanarProof
	faces []hierarchyGeneralPlanarFace
}

func prepareHierarchyGeneralPlanar(ctx context.Context, p IndependentExactMaterial, faceIDs []int64, a *hierarchyPointArithmetic, tick func() error) (*hierarchyGeneralPlanarProof, error) {
	vertices, edges, faces := p.Planar.Solid.Vertices(), p.Planar.Solid.Edges(), p.Planar.Solid.Faces()
	if len(faces) != len(faceIDs) {
		return nil, geom.ErrInvalid
	}
	base := &hierarchyConvexPlanarProof{vertices: make([][3]*big.Rat, len(vertices)), edges: edges, sourceVertices: make([]int64, len(vertices)), sourceEdges: make([]int64, len(edges))}
	for i, v := range vertices {
		base.vertices[i] = mappedRatVec(v)
	}
	out := &hierarchyGeneralPlanarProof{base: base}
	cache := map[int64]linearEdge{}
	for fi, face := range faces {
		if e := tick(); e != nil {
			return nil, e
		}
		fr, e := simple(p.Source, faceIDs[fi], "ADVANCED_FACE", 4)
		if e != nil {
			return nil, e
		}
		bounds, e := listRefs(fr.Args[1], 1)
		if e != nil {
			return nil, e
		}
		nativeLoops := append([][]brep.EdgeUse{face.Boundary}, face.Holes...)
		if len(bounds) != len(nativeLoops) {
			return nil, geom.ErrInvalid
		}
		sourcePairs := map[[2]int]int64{}
		var sourceLoops [][]int
		for _, bid := range bounds {
			en := p.Source.Entities[bid]
			if len(en.Records) != 1 || len(en.Records[0].Args) != 3 {
				return nil, geom.ErrInvalid
			}
			br := en.Records[0]
			sense, e := boolean(br.Args[2])
			if e != nil {
				return nil, e
			}
			loop, e := edgeLoop(p.Source, br.Args[1].Ref, cache)
			if e != nil {
				return nil, e
			}
			indices := make([]int, len(loop.points))
			for j, pid := range loop.points {
				pr, e := simple(p.Source, pid, "CARTESIAN_POINT", 2)
				if e != nil || len(pr.Args[1].Items) != 3 {
					return nil, geom.ErrInvalid
				}
				var exact [3]*big.Rat
				for k := range exact {
					exact[k], e = liftSourceRat(pr.Args[1].Items[k])
					if e != nil {
						return nil, e
					}
				}
				found := -1
				for i, v := range base.vertices {
					if e = tick(); e != nil {
						return nil, e
					}
					if planarEqual(exact, v) {
						if found >= 0 {
							return nil, geom.ErrInvalid
						}
						found = i
					}
				}
				if found < 0 {
					return nil, geom.ErrDomain
				}
				indices[j] = found
				if base.sourceVertices[found] != 0 && base.sourceVertices[found] != loop.vertices[j] {
					return nil, geom.ErrInvalid
				}
				base.sourceVertices[found] = loop.vertices[j]
			}
			for j, v := range indices {
				w := indices[(j+1)%len(indices)]
				if !sense {
					v, w = w, v
				}
				key := [2]int{v, w}
				if sourcePairs[key] != 0 {
					return nil, geom.ErrInvalid
				}
				sourcePairs[key] = loop.edges[j]
			}
			if !sense {
				for i, j := 0, len(indices)-1; i < j; i, j = i+1, j-1 {
					indices[i], indices[j] = indices[j], indices[i]
				}
			}
			sourceLoops = append(sourceLoops, indices)
		}
		support, e := simple(p.Source, fr.Args[2].Ref, "PLANE", 2)
		if e != nil {
			return nil, e
		}
		frame, e := mappedExactFrame(p.Source, support.Args[1].Ref)
		if e != nil {
			return nil, e
		}
		normal := mappedRatVec(frame.ZAxis())
		sense, e := boolean(fr.Args[3])
		if e != nil {
			return nil, e
		}
		if !sense {
			for i := range normal {
				normal[i] = new(big.Rat).Neg(normal[i])
			}
		}
		origin := mappedRatVec(frame.Origin())
		plane := hierarchyPlanarFace{normal, new(big.Rat).Neg(a.planarDot(normal, origin)), faceIDs[fi]}
		drop := 0
		for drop < 3 && normal[drop].Sign() == 0 {
			drop++
		}
		if drop == 3 {
			return nil, geom.ErrInvalid
		}
		gf := hierarchyGeneralPlanarFace{plane: plane, drop: drop}
		used := map[int]bool{}
		for li, loop := range nativeLoops {
			ids := make([]int, len(loop))
			for j, use := range loop {
				ed := edges[use.Edge]
				v, w := ed.Start, ed.End
				if use.Reversed {
					v, w = w, v
				}
				sid := sourcePairs[[2]int{v, w}]
				if sid == 0 {
					return nil, geom.ErrInvalid
				}
				if base.sourceEdges[use.Edge] != 0 && base.sourceEdges[use.Edge] != sid {
					return nil, geom.ErrInvalid
				}
				base.sourceEdges[use.Edge] = sid
				ids[j] = v
				if a.add(a.planarDot(normal, base.vertices[v]), plane.c).Sign() != 0 {
					return nil, geom.ErrDomain
				}
			}
			matched := -1
			for j, source := range sourceLoops {
				if len(source) != len(ids) {
					continue
				}
				for shift := range source {
					ok := true
					for k, id := range ids {
						if id != source[(k+shift)%len(source)] {
							ok = false
							break
						}
					}
					if ok {
						matched = j
						break
					}
				}
			}
			if matched < 0 || used[matched] {
				return nil, geom.ErrInvalid
			}
			used[matched] = true
			area := new(big.Rat)
			i, j := (drop+1)%3, (drop+2)%3
			for k, v := range ids {
				w := ids[(k+1)%len(ids)]
				area = a.add(area, a.sub(a.mul(base.vertices[v][i], base.vertices[w][j]), a.mul(base.vertices[v][j], base.vertices[w][i])))
			}
			orientation := area.Sign() * normal[drop].Sign()
			if li == 0 && orientation <= 0 || li > 0 && orientation >= 0 {
				return nil, geom.ErrInvalid
			}
			gf.loops = append(gf.loops, ids)
			gf.boundIDs = append(gf.boundIDs, bounds[matched])
		}
		base.planes = append(base.planes, plane)
		out.faces = append(out.faces, gf)
	}
	if a.err != nil {
		return nil, a.err
	}
	return out, nil
}

// finiteFace returns -1 outside, 0 on a loop boundary, +1 in the face region.
func (p *hierarchyGeneralPlanarProof) finiteFace(face hierarchyGeneralPlanarFace, point [3]*big.Rat, a *hierarchyPointArithmetic, tick func() error) (int, error) {
	i, j := (face.drop+1)%3, (face.drop+2)%3
	winding := 0
	for _, loop := range face.loops {
		for k, index := range loop {
			if e := tick(); e != nil {
				return 0, e
			}
			v, w := p.base.vertices[index], p.base.vertices[loop[(k+1)%len(loop)]]
			cross := a.sub(a.mul(a.sub(w[i], v[i]), a.sub(point[j], v[j])), a.mul(a.sub(w[j], v[j]), a.sub(point[i], v[i])))
			if cross.Sign() == 0 {
				inside := true
				for _, axis := range []int{i, j} {
					lo, hi := v[axis], w[axis]
					if lo.Cmp(hi) > 0 {
						lo, hi = hi, lo
					}
					if point[axis].Cmp(lo) < 0 || point[axis].Cmp(hi) > 0 {
						inside = false
					}
				}
				if inside {
					return 0, a.err
				}
			}
			if v[j].Cmp(point[j]) <= 0 {
				if w[j].Cmp(point[j]) > 0 && cross.Sign() > 0 {
					winding++
				}
			} else if w[j].Cmp(point[j]) <= 0 && cross.Sign() < 0 {
				winding--
			}
			runtime.KeepAlive(&w) // golang/go#82026: keeps w in the stack map. Remove once the Go fix is in use.
		}
	}
	if a.err != nil {
		return 0, a.err
	}
	if winding != 0 {
		return 1, nil
	}
	return -1, nil
}
func (p *hierarchyGeneralPlanarProof) classify(point [3]*big.Rat, a *hierarchyPointArithmetic, tick func() error, maxRays int) (AnalyticPointLocation, []int64, []int64, []int64, int, error) {
	fail := func(e error) (AnalyticPointLocation, []int64, []int64, []int64, int, error) {
		return "", nil, nil, nil, 0, e
	}
	var boundary []int64
	for _, face := range p.faces {
		if e := tick(); e != nil {
			return fail(e)
		}
		if a.add(a.planarDot(face.plane.n, point), face.plane.c).Sign() == 0 {
			where, e := p.finiteFace(face, point, a, tick)
			if e != nil {
				return fail(e)
			}
			if where >= 0 {
				boundary = append(boundary, face.plane.id)
			}
		}
	}
	if a.err != nil {
		return fail(a.err)
	}
	if len(boundary) > 0 {
		_, edges, vertices, e := p.base.contacts(point, a, tick)
		if e != nil {
			return fail(e)
		}
		return AnalyticPointBoundary, boundary, edges, vertices, 0, nil
	}
	for attempt := 1; attempt <= maxRays; attempt++ {
		direction := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(int64(attempt), 1), big.NewRat(int64(attempt*attempt), 1)}
		crossings := 0
		generic := true
		for _, face := range p.faces {
			if e := tick(); e != nil {
				return fail(e)
			}
			slope := a.planarDot(face.plane.n, direction)
			if slope.Sign() == 0 {
				generic = false
				break
			}
			value := a.add(a.planarDot(face.plane.n, point), face.plane.c)
			t := a.check(new(big.Rat).Quo(new(big.Rat).Neg(value), slope))
			if t.Sign() <= 0 {
				continue
			}
			var hit [3]*big.Rat
			for i := range hit {
				hit[i] = a.add(point[i], a.mul(t, direction[i]))
			}
			where, e := p.finiteFace(face, hit, a, tick)
			if e != nil {
				return fail(e)
			}
			if where == 0 {
				generic = false
				break
			}
			if where > 0 {
				crossings++
			}
		}
		if a.err != nil {
			return fail(a.err)
		}
		if generic {
			where := AnalyticPointOutside
			if crossings%2 == 1 {
				where = AnalyticPointInside
			}
			return where, nil, nil, nil, attempt, nil
		}
	}
	return fail(geom.ErrConvergence)
}
