package step

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// clipHierarchyGeneralPlanar accepts transverse finite-face events only. A line
// contained in any support plane is explicitly unsupported before events emit.
func clipHierarchyGeneralPlanar(ctx context.Context, leaf HierarchyLineLeaf, proof *hierarchyGeneralPlanarProof, u RationalHierarchyInstance, origin, direction [3]*big.Rat, lower, upper *big.Rat, a *hierarchyPointArithmetic, out *RationalHierarchyLine, maxTests, maxRays int, opts func() (geom.ExactCurvePlaneEventOptions, error), addHit func(*geom.ExactAlgebraicRoot, HierarchyLineOwner) error, coplanarMode ...bool) (HierarchyLineLeaf, error) {
	allowCoplanar := len(coplanarMode) > 0 && coplanarMode[0]
	fail := func(e error) (HierarchyLineLeaf, error) { return HierarchyLineLeaf{}, e }
	tick := func() error {
		out.PlanarTests++
		if out.PlanarTests > maxTests {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	zero := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
	axes := [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z}
	x := a.inverse(origin, u.pose.Origin, axes)
	d := a.inverse(direction, zero, axes)
	point := func(t *big.Rat) (q [3]*big.Rat) {
		for i := range q {
			q[i] = a.add(x[i], a.mul(d[i], t))
		}
		return
	}
	classify := func(t *big.Rat) (AnalyticPointLocation, []int64, []int64, []int64, error) {
		where, fs, eds, vs, n, e := proof.classify(point(t), a, tick, maxRays-out.RayDirections)
		out.RayDirections += n
		return where, fs, eds, vs, e
	}
	constants, slopes := make([]*big.Rat, len(proof.faces)), make([]*big.Rat, len(proof.faces))
	for i, f := range proof.faces {
		if e := tick(); e != nil {
			return fail(e)
		}
		constants[i] = a.add(a.planarDot(f.plane.n, x), f.plane.c)
		slopes[i] = a.planarDot(f.plane.n, d)
		if slopes[i].Sign() == 0 && constants[i].Sign() == 0 && !allowCoplanar {
			if a.err != nil {
				return fail(a.err)
			}
			leaf.UnsupportedReason = "nonconvex line coplanar with source supporting face"
			return leaf, nil
		}
	}
	if a.err != nil {
		return fail(a.err)
	}
	type station struct {
		t        *big.Rat
		boundary bool
		root     *geom.ExactAlgebraicRoot
	}
	stations := map[string]*station{}
	add := func(t *big.Rat, boundary bool) {
		key := t.RatString()
		if old := stations[key]; old != nil {
			old.boundary = old.boundary || boundary
		} else {
			stations[key] = &station{t: new(big.Rat).Set(t), boundary: boundary}
		}
	}
	add(lower, false)
	add(upper, false)
	for i, f := range proof.faces {
		if slopes[i].Sign() == 0 {
			if constants[i].Sign() != 0 {
				continue
			}
			first, second := (f.drop+1)%3, (f.drop+2)%3
			cross2 := func(ax, ay, bx, by *big.Rat) *big.Rat { return a.sub(a.mul(ax, by), a.mul(ay, bx)) }
			for _, loop := range f.loops {
				for j, vid := range loop {
					if e := tick(); e != nil {
						return fail(e)
					}
					v, w := proof.base.vertices[vid], proof.base.vertices[loop[(j+1)%len(loop)]]
					ex, ey := a.sub(w[first], v[first]), a.sub(w[second], v[second])
					px, py := a.sub(v[first], x[first]), a.sub(v[second], x[second])
					det := cross2(d[first], d[second], ex, ey)
					if det.Sign() != 0 {
						t := a.check(new(big.Rat).Quo(cross2(px, py, ex, ey), det))
						u := a.check(new(big.Rat).Quo(cross2(px, py, d[first], d[second]), det))
						if u.Sign() >= 0 && u.Cmp(big.NewRat(1, 1)) <= 0 && t.Cmp(lower) >= 0 && t.Cmp(upper) <= 0 {
							add(t, true)
						}
					} else if cross2(px, py, d[first], d[second]).Sign() == 0 {
						axis := first
						if d[axis].Sign() == 0 {
							axis = second
						}
						if d[axis].Sign() == 0 {
							return fail(geom.ErrInvalid)
						}
						for _, vertex := range [][3]*big.Rat{v, w} {
							t := a.check(new(big.Rat).Quo(a.sub(vertex[axis], x[axis]), d[axis]))
							if t.Cmp(lower) >= 0 && t.Cmp(upper) <= 0 {
								add(t, true)
							}
						}
					}
				}
			}
			continue
		}
		t := a.check(new(big.Rat).Quo(new(big.Rat).Neg(constants[i]), slopes[i]))
		if t.Cmp(lower) < 0 || t.Cmp(upper) > 0 {
			continue
		}
		where, e := proof.finiteFace(f, point(t), a, tick)
		if e != nil {
			return fail(e)
		}
		if where >= 0 {
			add(t, true)
		}
	}
	if a.err != nil {
		return fail(a.err)
	}
	ordered := make([]*station, 0, len(stations))
	for _, s := range stations {
		ordered = append(ordered, s)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].t.Cmp(ordered[j].t) < 0 })
	for _, s := range ordered {
		op, e := opts()
		if e != nil {
			return fail(e)
		}
		var n int
		s.root, n, e = geom.NewExactRationalRoot(ctx, s.t, op)
		out.Operations += n
		if e != nil {
			return fail(e)
		}
		where, fs, eds, vs, e := classify(s.t)
		if e != nil {
			return fail(e)
		}
		sign := 1
		if where == AnalyticPointInside {
			sign = -1
		} else if where == AnalyticPointBoundary {
			sign = 0
		}
		if s.t.Cmp(lower) == 0 {
			leaf.LowerSign = sign
		}
		if s.t.Cmp(upper) == 0 {
			leaf.UpperSign = sign
		}
		if s.boundary {
			if where != AnalyticPointBoundary {
				return fail(geom.ErrInvalid)
			}
			owner := leaf.Owner
			owner.ContactFaceIDs = fs
			owner.ContactEdgeIDs = eds
			owner.ContactVertexIDs = vs
			for _, id := range fs {
				multiplicity := 1
				for j, f := range proof.faces {
					if f.plane.id == id && slopes[j].Sign() == 0 {
						multiplicity = 0
					}
				}
				owner.Supports = append(owner.Supports, HierarchyLineSupport{SourceFaceID: id, Role: "planar-face", Multiplicity: multiplicity})
			}
			if len(fs) == 1 {
				owner.Multiplicity = owner.Supports[0].Multiplicity
			}
			if e = addHit(s.root, owner); e != nil {
				return fail(e)
			}
		}
	}
	for i := 1; i < len(ordered); i++ {
		mid := a.check(new(big.Rat).Quo(a.add(ordered[i-1].t, ordered[i].t), big.NewRat(2, 1)))
		where, fs, eds, _, e := classify(mid)
		if e != nil {
			return fail(e)
		}
		if where == AnalyticPointBoundary {
			if !allowCoplanar {
				return fail(geom.ErrInvalid)
			}
			leaf.BoundarySpans = append(leaf.BoundarySpans, HierarchyLineBoundarySpan{Lower: ordered[i-1].root, Upper: ordered[i].root, Roles: []string{"planar-face"}, SourceFaceIDs: fs, SourceEdgeIDs: eds})
		}
		if where == AnalyticPointInside {
			leaf.Spans = append(leaf.Spans, HierarchyLineSpan{Lower: ordered[i-1].root, Upper: ordered[i].root})
		}
	}
	if a.err != nil {
		return fail(a.err)
	}
	leaf.Supported = true
	return leaf, nil
}

// ClipGeneralRationalHierarchyLine adds transverse nonconvex/holed planar leaves.
// Coplanar planar lines are explicitly unsupported; prior convex APIs retain
// their stronger coplanar subset and are not silently widened.
func ClipGeneralRationalHierarchyLine(ctx context.Context, m RationalSphericalHierarchy, origin, direction [3]*big.Rat, lower, upper *big.Rat, o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
	return clipRationalHierarchyLine(ctx, m, origin, direction, lower, upper, o, false, true)
}
func ClipGeneralOCCTRationalHierarchyLine(ctx context.Context, m OCCTRationalSphericalHierarchy, origin, direction [3]*big.Rat, lower, upper *big.Rat, o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
	return clipRationalHierarchyLine(ctx, RationalSphericalHierarchy(m), origin, direction, lower, upper, o, true, true)
}

// ClipCoplanarGeneralRationalHierarchyLine additionally proves finite coplanar
// polygon and hole spans; the transverse-only API keeps its original gate.
func ClipCoplanarGeneralRationalHierarchyLine(ctx context.Context, m RationalSphericalHierarchy, origin, direction [3]*big.Rat, lower, upper *big.Rat, o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
	return clipRationalHierarchyLine(ctx, m, origin, direction, lower, upper, o, false, true, true)
}
func ClipCoplanarGeneralOCCTRationalHierarchyLine(ctx context.Context, m OCCTRationalSphericalHierarchy, origin, direction [3]*big.Rat, lower, upper *big.Rat, o RationalHierarchyLineOptions) (RationalHierarchyLine, error) {
	return clipRationalHierarchyLine(ctx, RationalSphericalHierarchy(m), origin, direction, lower, upper, o, true, true, true)
}
