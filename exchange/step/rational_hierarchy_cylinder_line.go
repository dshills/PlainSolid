package step

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

func clipHierarchyCylinder(ctx context.Context, leaf HierarchyLineLeaf, x, d [3]*big.Rat, r, h, lower, upper *big.Rat, a *hierarchyPointArithmetic, out *RationalHierarchyLine, opts func() (geom.ExactCurvePlaneEventOptions, error), compare func(*geom.ExactAlgebraicRoot, *geom.ExactAlgebraicRoot) (int, error), addHit func(*geom.ExactAlgebraicRoot, HierarchyLineOwner) error) (HierarchyLineLeaf, error) {
	fail := func(e error) (HierarchyLineLeaf, error) { return HierarchyLineLeaf{}, e }
	radial := []*big.Rat{a.sub(a.add(a.mul(x[0], x[0]), a.mul(x[1], x[1])), a.mul(r, r)), a.mul(big.NewRat(2, 1), a.add(a.mul(x[0], d[0]), a.mul(x[1], d[1]))), a.add(a.mul(d[0], d[0]), a.mul(d[1], d[1]))}
	eval := func(t *big.Rat) *big.Rat { return a.add(radial[0], a.mul(t, a.add(radial[1], a.mul(t, radial[2])))) }
	classify := func(t *big.Rat) int {
		z := a.add(x[2], a.mul(t, d[2]))
		q := eval(t)
		if z.Sign() < 0 || z.Cmp(h) > 0 || q.Sign() > 0 {
			return 1
		}
		if z.Sign() == 0 || z.Cmp(h) == 0 || q.Sign() == 0 {
			return 0
		}
		return -1
	}
	leaf.LowerSign = classify(lower)
	leaf.UpperSign = classify(upper)
	if a.err != nil {
		return fail(a.err)
	}
	lo, hi := new(big.Rat).Set(lower), new(big.Rat).Set(upper)
	var capT [2]*big.Rat
	var coincident []string
	if d[2].Sign() == 0 {
		if x[2].Sign() < 0 || x[2].Cmp(h) > 0 {
			return leaf, nil
		}
		if x[2].Sign() == 0 {
			coincident = append(coincident, "lower-cap")
		}
		if x[2].Cmp(h) == 0 {
			coincident = append(coincident, "upper-cap")
		}
	} else {
		capT[0] = a.check(new(big.Rat).Quo(new(big.Rat).Neg(x[2]), d[2]))
		capT[1] = a.check(new(big.Rat).Quo(a.sub(h, x[2]), d[2]))
		low, high := capT[0], capT[1]
		if low.Cmp(high) > 0 {
			low, high = high, low
		}
		if lo.Cmp(low) < 0 {
			lo.Set(low)
		}
		if hi.Cmp(high) > 0 {
			hi.Set(high)
		}
		if lo.Cmp(hi) > 0 {
			return leaf, a.err
		}
	}
	if a.err != nil {
		return fail(a.err)
	}
	root := func(t *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		op, e := opts()
		if e != nil {
			return nil, e
		}
		q, n, e := geom.NewExactRationalRoot(ctx, t, op)
		out.Operations += n
		return q, e
	}
	lroot, e := root(lo)
	if e != nil {
		return fail(e)
	}
	hroot, e := root(hi)
	if e != nil {
		return fail(e)
	}
	wallCoincident := radial[0].Sign() == 0 && radial[1].Sign() == 0 && radial[2].Sign() == 0
	if wallCoincident {
		coincident = append(coincident, "wall")
	}
	var roots []geom.ExactPolynomialRoot
	if !wallCoincident {
		op, e := opts()
		if e != nil {
			return fail(e)
		}
		isolated, e := geom.IsolateExactPolynomialRoots(ctx, radial, lo, hi, op)
		out.Operations += isolated.Operations
		out.Subdivisions += isolated.Subdivisions
		if e != nil {
			return fail(e)
		}
		roots = isolated.Roots
		if e = sortHierarchyLineRoots(roots, compare); e != nil {
			return fail(e)
		}
	}
	// Candidate roots have already passed the exact axial slab. Add both cap
	// roles at a rim, including support coincidences, once per occurrence.
	emit := func(q *geom.ExactAlgebraicRoot, wallMultiplicity int, wall bool) error {
		own := leaf.Owner
		for _, role := range coincident {
			own.Supports = append(own.Supports, HierarchyLineSupport{role, 0})
		}
		if wall && !wallCoincident {
			own.Supports = append(own.Supports, HierarchyLineSupport{"wall", wallMultiplicity})
		}
		for i, t := range capT {
			if t == nil || t.Cmp(lower) < 0 || t.Cmp(upper) > 0 {
				continue
			}
			cr, e := root(t)
			if e != nil {
				return e
			}
			order, e := compare(q, cr)
			if e != nil {
				return e
			}
			if order == 0 {
				role := "lower-cap"
				if i == 1 {
					role = "upper-cap"
				}
				own.Supports = append(own.Supports, HierarchyLineSupport{role, 1})
			}
		}
		if len(own.Supports) == 1 {
			own.Multiplicity = own.Supports[0].Multiplicity
		}
		return addHit(q, own)
	}
	for _, q := range roots {
		if e = emit(q.Root, q.Multiplicity, true); e != nil {
			return fail(e)
		}
	}
	for _, t := range capT {
		if t == nil || t.Cmp(lower) < 0 || t.Cmp(upper) > 0 || eval(t).Sign() > 0 {
			continue
		}
		q, e := root(t)
		if e != nil {
			return fail(e)
		}
		duplicate := false
		for _, w := range roots {
			order, e := compare(q, w.Root)
			if e != nil {
				return fail(e)
			}
			if order == 0 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			if e = emit(q, 0, wallCoincident); e != nil {
				return fail(e)
			}
		}
	}
	if a.err != nil {
		return fail(a.err)
	}
	if lo.Cmp(hi) == 0 {
		return leaf, nil
	}
	appendSpan := func(l, u *geom.ExactAlgebraicRoot) {
		if len(coincident) > 0 {
			leaf.BoundarySpans = append(leaf.BoundarySpans, HierarchyLineBoundarySpan{l, u, append([]string(nil), coincident...)})
		} else {
			leaf.Spans = append(leaf.Spans, HierarchyLineSpan{l, u})
		}
	}
	if wallCoincident {
		appendSpan(lroot, hroot)
		return leaf, nil
	}
	sign := eval(lo).Sign()
	if sign == 0 {
		sign = a.add(radial[1], a.mul(big.NewRat(2, 1), a.mul(lo, radial[2]))).Sign()
		if sign == 0 {
			sign = radial[2].Sign()
		}
	}
	previous := lroot
	for _, q := range roots {
		order, e := compare(q.Root, lroot)
		if e != nil {
			return fail(e)
		}
		if order == 0 {
			continue
		}
		if sign < 0 {
			appendSpan(previous, q.Root)
		}
		if q.Multiplicity%2 == 1 {
			sign = -sign
		}
		previous = q.Root
	}
	order, e := compare(previous, hroot)
	if e != nil {
		return fail(e)
	}
	if order < 0 && sign < 0 {
		appendSpan(previous, hroot)
	}
	if a.err != nil {
		return fail(a.err)
	}
	return leaf, nil
}

// Root isolation completeness does not imply record order: endpoint roots can
// precede interior roots. Sort exact identities before material sign traversal.
func sortHierarchyLineRoots(roots []geom.ExactPolynomialRoot, compare func(*geom.ExactAlgebraicRoot, *geom.ExactAlgebraicRoot) (int, error)) error {
	for i := 1; i < len(roots); i++ {
		v := roots[i]
		j := i
		for j > 0 {
			order, e := compare(v.Root, roots[j-1].Root)
			if e != nil {
				return e
			}
			if order == 0 {
				return geom.ErrInvalid
			}
			if order > 0 {
				break
			}
			roots[j] = roots[j-1]
			j--
		}
		roots[j] = v
	}
	return nil
}
