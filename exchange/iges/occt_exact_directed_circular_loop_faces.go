package iges

import (
	"context"
	"math/big"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func occtIGESDirectedFaceIDs(g igesPlanarGraph, id int) ([]int, error) {
	f, ok := g.records[id]
	if !ok || f.kind != 510 || len(f.values) < 5 {
		return nil, occtIGESDirectedFailure(id, "missing directed face")
	}
	n, err := intValue(f.values[2])
	if err != nil || n < 1 || n > 21 || len(f.values) != 4+n {
		return nil, occtIGESDirectedFailure(id, "invalid original ring count")
	}
	return occtIGESDirectedInts(g, id, 510, 4+n)
}

func occtIGESDirectedReadFace(ctx context.Context, g igesPlanarGraph, v []int, id, edgelist int, reversed bool, vertices []igesPlanarVec, edges []occtIGESDirectedEdge) (occtIGESDirectedFace, error) {
	f := occtIGESDirectedFace{id: id, support: v[1], reversed: reversed}
	fail := func(entity int, why string) (occtIGESDirectedFace, error) {
		return f, occtIGESDirectedFailure(entity, why)
	}
	if v[3] != 1 {
		return fail(id, "missing explicit outer wire")
	}
	switch g.records[f.support].kind {
	case 190:
		p, err := occtIGESDirectedInts(g, f.support, 190, 4)
		if err != nil {
			return f, err
		}
		if f.origin, err = g.datum(p[1], 116); err != nil {
			return f, err
		}
		if f.normal, err = g.datum(p[2], 123); err != nil {
			return f, err
		}
		if f.x, err = g.datum(p[3], 123); err != nil {
			return f, err
		}
	case 192:
		f.circular = true
		p, err := g.take(f.support, 192)
		if err != nil {
			return f, err
		}
		if len(p.values) != 5 || v[2] != 1 {
			return fail(f.support, "invalid cylinder or wall trim count")
		}
		datum := func(i, k int) (igesPlanarVec, error) {
			j, err := intValue(p.values[i])
			if err != nil {
				return igesPlanarVec{}, err
			}
			return g.datum(j, k)
		}
		if f.origin, err = datum(1, 116); err != nil {
			return f, err
		}
		if f.normal, err = datum(2, 123); err != nil {
			return f, err
		}
		if f.x, err = datum(4, 123); err != nil {
			return f, err
		}
		if f.radius, err = igesPlanarExact(p.values[3]); err != nil || f.radius.Sign() <= 0 {
			return fail(f.support, "invalid cylinder radius")
		}
		if f.normal[0].Sign() != 0 || f.normal[1].Sign() != 0 || f.x[2].Sign() != 0 ||
			f.normal.dot(f.normal).Cmp(big.NewRat(1, 1)) != 0 || f.x.dot(f.x).Cmp(big.NewRat(1, 1)) != 0 ||
			f.x[0].Sign() != 0 && f.x[1].Sign() != 0 {
			return fail(f.support, "cylinder chart is not exact cardinal XY/Z")
		}
	default:
		return fail(f.support, "unsupported face surface")
	}
	if f.normal.dot(f.normal).Sign() == 0 || f.x.dot(f.x).Sign() == 0 || f.normal.dot(f.x).Sign() != 0 {
		return fail(f.support, "invalid exact analytic frame")
	}
	stride := 5
	if f.circular {
		stride = 7
	}
	for _, loopID := range v[4:] {
		if err := ctx.Err(); err != nil {
			return f, err
		}
		q, err := g.take(loopID, 508)
		if err != nil {
			return f, err
		}
		k, err := igesPlanarCount(q, 1, 2, stride, 3)
		if err != nil || k > 64 || len(q.values) != 2+stride*k || f.circular && k != 4 {
			return fail(loopID, "invalid directed loop length")
		}
		a, err := topologyInts(q, 2+stride*k)
		if err != nil {
			return f, err
		}
		l := occtIGESDirectedLoop{id: loopID}
		first, last := -1, -1
		var firstUV, lastUV igesPlanarVec
		seen := map[int]bool{}
		for i := 0; i < k; i++ {
			b := a[2+stride*i : 2+stride*(i+1)]
			if b[0] != 0 || b[1] != edgelist || b[2] < 1 || b[2] > len(edges) || b[3] != 0 && b[3] != 1 {
				return fail(loopID, "invalid original loop owner")
			}
			u := occtIGESDirectedUse{edge: b[2] - 1, forward: b[3] == 1}
			edge := edges[u.edge]
			p, q := edge.from, edge.to
			if !u.forward {
				p, q = q, p
			}
			if seen[p] || last != -1 && p != last {
				return fail(loopID, "original coedges do not form a simple cycle")
			}
			if i == 0 {
				first = p
			}
			last, seen[p] = q, true
			if f.circular {
				if b[4] != 1 || b[5] != 0 {
					return fail(loopID, "each original cylindrical coedge needs its own pcurve")
				}
				u.pcurve = b[6]
				uv, err := occtIGESDirectedCylinderUse(g, f, u, edge, vertices[p], vertices[q])
				if err != nil {
					return f, err
				}
				if i == 0 {
					firstUV = uv[0]
				} else if !lastUV.equal(uv[0]) {
					return fail(u.pcurve, "owner pcurves do not join in the original unwrapped chart")
				}
				lastUV = uv[1]
			} else {
				if b[4] != 0 || vertices[p].sub(f.origin).dot(f.normal).Sign() != 0 ||
					edge.circular && (edge.center.sub(f.origin).dot(f.normal).Sign() != 0 || f.normal[0].Sign() != 0 || f.normal[1].Sign() != 0) {
					return fail(loopID, "original model coedge disagrees with plane")
				}
			}
			l.uses = append(l.uses, u)
		}
		if last != first || f.circular && !lastUV.equal(firstUV) {
			return fail(loopID, "original coedge cycle does not close")
		}
		f.loops = append(f.loops, l)
	}
	return f, nil
}

func occtIGESDirectedCylinderUse(g igesPlanarGraph, f occtIGESDirectedFace, use occtIGESDirectedUse, edge occtIGESDirectedEdge, p, q igesPlanarVec) ([2]igesPlanarVec, error) {
	var zero [2]igesPlanarVec
	a, err := g.take(use.pcurve, 110)
	if err != nil {
		return zero, err
	}
	fail := func(detail string) ([2]igesPlanarVec, error) { return zero, occtIGESDirectedFailure(a.id, detail) }
	if len(a.values) != 7 {
		return fail("invalid finite cylinder pcurve")
	}
	first, e1 := igesPlanarCoordinates(a.values[1:4])
	last, e2 := igesPlanarCoordinates(a.values[4:7])
	if e1 != nil || e2 != nil || first[2].Sign() != 0 || last[2].Sign() != 0 {
		return fail("pcurve is not an exact UV line")
	}
	if edge.circular {
		if edge.center[0].Cmp(f.origin[0]) != 0 || edge.center[1].Cmp(f.origin[1]) != 0 ||
			first[1].Cmp(last[1]) != 0 {
			return fail("circular owner pcurve disagrees with cylinder")
		}
		sense := -1
		if use.forward {
			sense = 1
		}
		du := new(big.Rat).Sub(last[0], first[0])
		if du.Cmp(big.NewRat(int64(90*sense*f.normal[2].Sign()), 1)) != 0 {
			return fail("pcurve selects the wrong angular span")
		}
	} else if first[0].Cmp(last[0]) != 0 || p[0].Cmp(q[0]) != 0 || p[1].Cmp(q[1]) != 0 || first[1].Cmp(last[1]) == 0 {
		return fail("straight owner pcurve is not an exact cylinder generator")
	}

	y := f.normal.cross(f.x)
	for i, uv := range []igesPlanarVec{first, last} {
		w := []igesPlanarVec{p, q}[i]
		if !uv[0].IsInt() || !uv[0].Num().IsInt64() || uv[0].Cmp(big.NewRat(-1080, 1)) < 0 || uv[0].Cmp(big.NewRat(1080, 1)) > 0 {
			return fail("pcurve angular station is outside the exact cardinal profile")
		}
		s := (uv[0].Num().Int64()%360 + 360) % 360
		cos, sin := int64(0), int64(0)
		switch s {
		case 0:
			cos = 1
		case 90:
			sin = 1
		case 180:
			cos = -1
		case 270:
			sin = -1
		default:
			return fail("pcurve angular station is not cardinal")
		}
		for j := 0; j < 2; j++ {
			polar := new(big.Rat).Add(new(big.Rat).Mul(f.x[j], big.NewRat(cos, 1)), new(big.Rat).Mul(y[j], big.NewRat(sin, 1)))
			expect := polar.Add(polar.Mul(polar, f.radius), f.origin[j])
			if expect.Cmp(w[j]) != 0 {
				return fail("pcurve endpoint misses original circle or generator")
			}
		}
		z := new(big.Rat).Add(new(big.Rat).Mul(f.normal[2], uv[1]), f.origin[2])
		if z.Cmp(w[2]) != 0 {
			return fail("pcurve endpoint misses original axial station")
		}
	}
	return [2]igesPlanarVec{first, last}, nil
}

func occtIGESDirectedUseAt(uses []occtIGESDirectedUse, index int, flip bool) occtIGESDirectedUse {
	if !flip {
		return uses[index]
	}
	u := uses[len(uses)-1-index]
	u.forward = !u.forward
	return u
}

func occtIGESDirectedBind(out *OCCTExactDirectedCircularLoopIGES, flat []trimmed.ExactCircularLoopSegment, points []geom.Vec3, edges []occtIGESDirectedEdge, faces map[int]occtIGESDirectedFace, owners [][]int) error {
	vertices := map[geom.Vec3]int{}
	for i, p := range points {
		vertices[p] = i + 1
	}
	pair := func(a, b int) [2]int { return [2]int{min(a, b), max(a, b)} }
	index := make(map[[2]int]int, len(edges))
	for i, e := range edges {
		k := pair(e.from+1, e.to+1)
		if index[k] != 0 || len(owners[i]) != 2 || owners[i][0] == owners[i][1] {
			return occtIGESDirectedFailure(out.SourceEdges, "model edges must have two independent face owners")
		}
		index[k] = i + 1
	}
	vs, es := out.Native.Vertices(), out.Native.Edges()
	roles := make([]occtIGESDirectedUse, len(es))
	n := len(flat)
	for i, edge := range es {
		a, b := vertices[vs[edge.Start]], vertices[vs[edge.End]]
		id := index[pair(a, b)]
		if min(a, b, id) < 1 {
			return occtIGESDirectedFailure(out.SourceEdges, "native edge does not match original pair")
		}
		orig := edges[id-1]
		u := occtIGESDirectedUse{edge: id - 1, forward: orig.from+1 == a}
		roles[i] = u
		circular := i < 2*n && flat[i%n].Radius > 0
		if circular != orig.circular || circular && u.forward == directedIGESClockwise(flat[i%n]) {
			return occtIGESDirectedFailure(orig.curve, "wrong model edge type or directed span")
		}
		if circular {
			s := flat[i%n]
			if orig.center[0].Cmp(compositeRat(s.Center.X)) != 0 || orig.center[1].Cmp(compositeRat(s.Center.Y)) != 0 {
				return occtIGESDirectedFailure(orig.curve, "original rim centers differ between caps")
			}
		}
		switch {
		case i < n:
			out.SourceRimEdges[0] = append(out.SourceRimEdges[0], id)
			out.SourceRimCurves[0] = append(out.SourceRimCurves[0], orig.curve)
		case i < 2*n:
			out.SourceRimEdges[1] = append(out.SourceRimEdges[1], id)
			out.SourceRimCurves[1] = append(out.SourceRimCurves[1], orig.curve)
		default:
			out.SourceVerticalEdges = append(out.SourceVerticalEdges, id)
			out.SourceVerticalCurves = append(out.SourceVerticalCurves, orig.curve)
		}
	}

	faceIDs := []int{out.SourceCaps[0], out.SourceCaps[1]}
	claimed := map[int]bool{faceIDs[0]: true, faceIDs[1]: true}
	for i := 0; i < n; i++ {
		adj := owners[roles[i].edge]
		wall := 0
		if adj[0] == faceIDs[0] {
			wall = adj[1]
		} else if adj[1] == faceIDs[0] {
			wall = adj[0]
		}
		if wall == 0 || claimed[wall] {
			return occtIGESDirectedFailure(out.SourceShell, "native wall lacks a unique original owner")
		}
		faceIDs = append(faceIDs, wall)
		claimed[wall] = true
		out.SourceWalls = append(out.SourceWalls, wall)
	}
	if len(claimed) != len(faces) {
		return occtIGESDirectedFailure(out.SourceShell, "original shell has unused faces")
	}
	for i, f := range out.Native.Faces() {
		q := faces[faceIDs[i]]
		rings := append([][]trimmed.Use{f.Outer}, f.Holes...)
		if len(rings) != len(q.loops) {
			return occtIGESDirectedFailure(q.id, "original face ring count differs")
		}
		used := make([]bool, len(rings))
		for h, ring := range rings {
			desired := make([]occtIGESDirectedUse, len(ring))
			for k, u := range ring {
				j := k
				if f.Reversed {
					j = len(ring) - 1 - j
				}
				v := roles[u.Edge]
				v.forward = v.forward != (f.Reversed != u.Reversed)
				desired[j] = v
			}
			found := -1
			for ch, loop := range q.loops {
				if used[ch] || (ch == 0) != (h == 0) || len(desired) != len(loop.uses) {
					continue
				}
				for shift := range desired {
					ok := true
					for j, w := range desired {
						x := occtIGESDirectedUseAt(loop.uses, (j+shift)%len(desired), q.reversed)
						if w.edge != x.edge || w.forward != x.forward {
							ok = false
							break
						}
					}
					if ok {
						found = ch
						break
					}
				}
				if found >= 0 {
					break
				}
			}
			if found < 0 {
				return occtIGESDirectedFailure(q.id, "original coedges differ from native directed incidence")
			}
			used[found] = true
			if i < 2 {
				out.SourceCapLoops[i] = append(out.SourceCapLoops[i], q.loops[found].id)
			}
		}
		if i >= 2 && flat[i-2].Radius > 0 {
			s := flat[i-2]
			if !q.circular || q.reversed != directedIGESClockwise(s) || q.radius.Cmp(compositeRat(s.Radius)) != 0 ||
				q.origin[0].Cmp(compositeRat(s.Center.X)) != 0 || q.origin[1].Cmp(compositeRat(s.Center.Y)) != 0 {
				return occtIGESDirectedFailure(q.support, "original cylinder disagrees with material wall")
			}
		} else {
			if q.circular {
				return occtIGESDirectedFailure(q.support, "material plane is not an original plane")
			}
			normal := igesPlanarVec{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
			switch {
			case i == 0:
				normal[2].Neg(normal[2])
			case i >= 2:
				s := flat[i-2]
				normal[0].Sub(compositeRat(s.End.Y), compositeRat(s.Start.Y))
				normal[1].Sub(compositeRat(s.Start.X), compositeRat(s.End.X))
				normal[2].SetInt64(0)
			}
			cross := q.normal.cross(normal)
			if cross.dot(cross).Sign() != 0 || (q.normal.dot(normal).Sign() < 0) != q.reversed {
				return occtIGESDirectedFailure(q.support, "original plane orientation disagrees with material")
			}
		}
	}
	return nil
}
