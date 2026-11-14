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
	tick := func() error {
		out.PlanarTests++
		if out.PlanarTests > maxTests {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	proof, reason, e := prepareHierarchyConvexPlanar(ctx, p, leaf.Owner.SourceFaceIDs, a, tick)
	if e != nil {
		return fail(e)
	}
	if proof == nil {
		leaf.UnsupportedReason = reason
		return leaf, nil
	}
	planes := proof.planes
	dot := a.planarDot
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
	contacts := func(t *big.Rat) ([]int64, []int64, []int64, error) {
		var point [3]*big.Rat
		for i := range point {
			point[i] = a.add(x[i], a.mul(d[i], t))
		}
		return proof.contacts(point, a, tick)
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
