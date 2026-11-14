package step

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

type hierarchyPlanarFace struct {
	n  [3]*big.Rat
	c  *big.Rat
	id int64
}

// clipHierarchyPlanar proves a convex hole-free source-bound subset. Unsupported
// geometry is explicit; source graph errors remain atomic errors.
func clipHierarchyPlanar(ctx context.Context, leaf HierarchyLineLeaf, p IndependentExactMaterial, u RationalHierarchyInstance, origin, direction [3]*big.Rat, lower, upper *big.Rat, a *hierarchyPointArithmetic, out *RationalHierarchyLine, maxTests int, opts func() (geom.ExactCurvePlaneEventOptions, error), addHit func(*geom.ExactAlgebraicRoot, HierarchyLineOwner) error) (HierarchyLineLeaf, error) {
	fail := func(e error) (HierarchyLineLeaf, error) { return HierarchyLineLeaf{}, e }
	unsupported := func() (HierarchyLineLeaf, error) {
		if a.err != nil {
			return fail(a.err)
		}
		leaf.Supported = false
		leaf.UnsupportedReason = "planar leaf requires exact convex hole-free outward faces"
		return leaf, nil
	}
	tick := func() error {
		out.PlanarTests++
		if out.PlanarTests > maxTests {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	vertices, edges, faces := p.Planar.Solid.Vertices(), p.Planar.Solid.Edges(), p.Planar.Solid.Faces()
	if len(faces) != len(leaf.Owner.SourceFaceIDs) {
		return fail(geom.ErrInvalid)
	}
	v := make([][3]*big.Rat, len(vertices))
	for i, q := range vertices {
		v[i] = mappedRatVec(q)
	}
	vs := make([]int64, len(v))
	es := make([]int64, len(edges))
	var planes []hierarchyPlanarFace
	dot := func(x, y [3]*big.Rat) *big.Rat {
		q := new(big.Rat)
		for i := range x {
			q = a.add(q, a.mul(x[i], y[i]))
		}
		return q
	}
	sub := func(x, y [3]*big.Rat) (q [3]*big.Rat) {
		for i := range q {
			q[i] = a.sub(x[i], y[i])
		}
		return
	}
	cross := func(x, y [3]*big.Rat) (q [3]*big.Rat) {
		for i := range q {
			q[i] = a.sub(a.mul(x[(i+1)%3], y[(i+2)%3]), a.mul(x[(i+2)%3], y[(i+1)%3]))
		}
		return
	}
	equal := func(x, y [3]*big.Rat) bool {
		for i := range x {
			if x[i].Cmp(y[i]) != 0 {
				return false
			}
		}
		return true
	}
	cache := map[int64]linearEdge{}
	for fi, face := range faces {
		if e := tick(); e != nil {
			return fail(e)
		}
		if len(face.Holes) > 0 || len(face.Boundary) < 3 {
			return unsupported()
		}
		fid := leaf.Owner.SourceFaceIDs[fi]
		fr, e := simple(p.Source, fid, "ADVANCED_FACE", 4)
		if e != nil {
			return fail(e)
		}
		bounds, e := listRefs(fr.Args[1], 1)
		if e != nil || len(bounds) != 1 {
			return unsupported()
		}
		en := p.Source.Entities[bounds[0]]
		if len(en.Records) != 1 {
			return fail(geom.ErrInvalid)
		}
		br := en.Records[0]
		if len(br.Args) != 3 {
			return fail(geom.ErrInvalid)
		}
		oriented, e := boolean(br.Args[2])
		if e != nil {
			return fail(e)
		}
		loop, e := edgeLoop(p.Source, br.Args[1].Ref, cache)
		if e != nil {
			return fail(e)
		}
		if len(loop.points) != len(face.Boundary) {
			return fail(geom.ErrInvalid)
		}
		indices := make([]int, len(loop.points))
		for j, pid := range loop.points {
			pr, e := simple(p.Source, pid, "CARTESIAN_POINT", 2)
			if e != nil || len(pr.Args[1].Items) != 3 {
				return fail(geom.ErrInvalid)
			}
			var exact [3]*big.Rat
			for axis := range exact {
				exact[axis], e = liftSourceRat(pr.Args[1].Items[axis])
				if e != nil {
					return fail(e)
				}
			}
			found := -1
			for i, q := range v {
				if e = tick(); e != nil {
					return fail(e)
				}
				if equal(exact, q) {
					if found >= 0 {
						return fail(geom.ErrInvalid)
					}
					found = i
				}
			}
			if found < 0 {
				return unsupported()
			}
			indices[j] = found
			if vs[found] != 0 && vs[found] != loop.vertices[j] {
				return fail(geom.ErrInvalid)
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
				return fail(geom.ErrInvalid)
			}
			if es[use.Edge] != 0 && es[use.Edge] != source {
				return fail(geom.ErrInvalid)
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
			return fail(e)
		}
		sourceFrame, e := mappedExactFrame(p.Source, support.Args[1].Ref)
		if e != nil {
			return fail(e)
		}
		sourceNormal := mappedRatVec(sourceFrame.ZAxis())
		sourceOrigin := mappedRatVec(sourceFrame.Origin())
		sense, e := boolean(fr.Args[3])
		if e != nil {
			return fail(e)
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
				return fail(e)
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
		return fail(a.err)
	}
	leaf.Supported = true
	zero := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	axes := [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z}
	x := a.inverse(origin, u.pose.Origin, axes)
	d := a.inverse(direction, zero, axes)
	constants, slopes := make([]*big.Rat, len(planes)), make([]*big.Rat, len(planes))
	lo, hi := new(big.Rat).Set(lower), new(big.Rat).Set(upper)
	var coincident []int64
	outside := false
	for i, plane := range planes {
		constants[i] = a.add(dot(plane.n, x), plane.c)
		slopes[i] = dot(plane.n, d)
		if slopes[i].Sign() == 0 {
			if constants[i].Sign() > 0 {
				outside = true
			}
			if constants[i].Sign() == 0 {
				coincident = append(coincident, plane.id)
			}
			continue
		}
		t := a.check(new(big.Rat).Quo(new(big.Rat).Neg(constants[i]), slopes[i]))
		if slopes[i].Sign() > 0 {
			if t.Cmp(hi) < 0 {
				hi.Set(t)
			}
		} else if t.Cmp(lo) > 0 {
			lo.Set(t)
		}
	}
	classify := func(t *big.Rat) int {
		boundary := false
		for i := range planes {
			sign := a.add(constants[i], a.mul(slopes[i], t)).Sign()
			if sign > 0 {
				return 1
			}
			boundary = boundary || sign == 0
		}
		if boundary {
			return 0
		}
		return -1
	}
	leaf.LowerSign = classify(lower)
	leaf.UpperSign = classify(upper)
	if a.err != nil {
		return fail(a.err)
	}
	if outside || lo.Cmp(hi) > 0 {
		return leaf, nil
	}
	root := func(t *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		op, e := opts()
		if e != nil {
			return nil, e
		}
		r, n, e := geom.NewExactRationalRoot(ctx, t, op)
		out.Operations += n
		return r, e
	}
	lroot, e := root(lo)
	if e != nil {
		return fail(e)
	}
	hroot, e := root(hi)
	if e != nil {
		return fail(e)
	}
	contacts := func(t *big.Rat) (fs, eds, verts []int64, err error) {
		var point [3]*big.Rat
		for i := range point {
			point[i] = a.add(x[i], a.mul(d[i], t))
		}
		for i, plane := range planes {
			if err = tick(); err != nil {
				return
			}
			if a.add(constants[i], a.mul(slopes[i], t)).Sign() == 0 {
				fs = append(fs, plane.id)
			}
		}
		for i, q := range v {
			if equal(q, point) {
				verts = append(verts, vs[i])
			}
		}
		for i, ed := range edges {
			if err = tick(); err != nil {
				return
			}
			ab := sub(v[ed.End], v[ed.Start])
			ap := sub(point, v[ed.Start])
			cp := cross(ab, ap)
			if dot(cp, cp).Sign() != 0 {
				continue
			}
			z := dot(ap, ab)
			if z.Sign() >= 0 && z.Cmp(dot(ab, ab)) <= 0 {
				eds = append(eds, es[i])
			}
		}
		err = a.err
		return
	}
	emit := func(t *big.Rat, r *geom.ExactAlgebraicRoot) error {
		fs, eds, verts, e := contacts(t)
		if e != nil {
			return e
		}
		discrete := false
		for i := range planes {
			if slopes[i].Sign() != 0 && a.add(constants[i], a.mul(slopes[i], t)).Sign() == 0 {
				discrete = true
			}
		}
		if !discrete {
			return nil
		}
		own := leaf.Owner
		own.ContactFaceIDs = fs
		own.ContactEdgeIDs = eds
		own.ContactVertexIDs = verts
		for i, plane := range planes {
			if a.add(constants[i], a.mul(slopes[i], t)).Sign() == 0 {
				multiplicity := 1
				if slopes[i].Sign() == 0 {
					multiplicity = 0
				}
				own.Supports = append(own.Supports, HierarchyLineSupport{Role: "planar-face", Multiplicity: multiplicity, SourceFaceID: plane.id})
			}
		}
		if len(own.Supports) == 1 {
			own.Multiplicity = own.Supports[0].Multiplicity
		}
		return addHit(r, own)
	}
	if e = emit(lo, lroot); e != nil {
		return fail(e)
	}
	if lo.Cmp(hi) != 0 {
		if e = emit(hi, hroot); e != nil {
			return fail(e)
		}
		if len(coincident) == 0 {
			leaf.Spans = append(leaf.Spans, HierarchyLineSpan{lroot, hroot})
		} else {
			mid := a.check(new(big.Rat).Quo(a.add(lo, hi), big.NewRat(2, 1)))
			fs, eds, _, e := contacts(mid)
			if e != nil {
				return fail(e)
			}
			leaf.BoundarySpans = append(leaf.BoundarySpans, HierarchyLineBoundarySpan{Lower: lroot, Upper: hroot, Roles: []string{"planar-face"}, SourceFaceIDs: fs, SourceEdgeIDs: eds})
		}
	}
	if a.err != nil {
		return fail(a.err)
	}
	return leaf, nil
}
