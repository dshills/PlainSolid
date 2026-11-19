//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// QuadraticSplineEdgeOverlap proves an interval correspondence in original
// model-edge units: parameterB=Offset+Scale*parameterA. Lower/Upper enclose
// the exact corresponding intervals, not sampled proximity.
type QuadraticSplineEdgeOverlap struct {
	Lower, Upper  [2]*big.Rat
	Offset, Scale *big.Rat
}

// QuadraticSplineEdgeContactReport separates proved finite contacts and exact
// overlaps from unresolved conic support contacts retained in Proof.
// Contact differential flags remain unset; Proof retains the implicit-conic
// diagnostics, which are not treated as full spatial tangent classification.
type QuadraticSplineEdgeContactReport struct {
	Contacts               []SplineEdgeContact
	Algebraic              []AlgebraicSplineEdgeContact
	Overlaps               []QuadraticSplineEdgeOverlap
	Proof                  geom.ExactQuadraticArcIntersection
	ProofSwapped, Complete bool
}

// IntersectSplineEdgesQuadraticArc intersects original model edges when one is
// a single noncollinear rational quadratic Bezier arc. The other may have higher
// degree and multiple knots. Algebraic brackets and overlap maps are translated
// exactly through edge reversal/ranges; no shell-face incidence is inferred.
func IntersectSplineEdgesQuadraticArc(ctx context.Context, a Solid, edgeA int, b Solid, edgeB int, o geom.ExactCurveSegmentOptions) (r QuadraticSplineEdgeContactReport, err error) {
	if ctx == nil || edgeA < 0 || edgeA >= len(a.edges) || edgeB < 0 || edgeB >= len(b.edges) {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	x, y := a.edges[edgeA], b.edges[edgeB]
	po := geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	pa, ra, e := exactEdgeSplineWithOptions(x, po)
	if e != nil {
		return r, e
	}
	pb, rb, e := exactEdgeSplineWithOptions(y, po)
	if e != nil {
		return r, e
	}
	if pa == nil || pb == nil {
		return r, geom.ErrConvergence
	}
	proof, e := geom.IntersectExactCurveQuadraticArc(ctx, pa, pb, o)
	swap := false
	if (e == geom.ErrConvergence || e == geom.ErrDegenerate) && len(proof.Hits) == 0 && len(proof.Overlaps) == 0 && len(proof.Pending) == 0 && len(proof.Coincident) == 0 {
		proof, e = geom.IntersectExactCurveQuadraticArc(ctx, pb, pa, o)
		swap = true
	}
	r.Proof, r.ProofSwapped, r.Complete = proof, swap, proof.Complete
	la, ha := pa.Domain()
	lb, hb := pb.Domain()
	maps := [2]splineEdgeParameterMap{{la, ha, x.Range.Min, x.Range.Max, ra}, {lb, hb, y.Range.Min, y.Range.Max, rb}}
	for _, hit := range proof.Hits {
		if hit.Curve.Parameter != nil && hit.ArcParameter != nil {
			u, v := hit.Curve.Parameter, hit.ArcParameter
			if swap {
				u, v = v, u
			}
			r.Contacts = append(r.Contacts, edgeContactFromExact([2]*big.Rat{u, v}, hit.Curve.Point, maps, [2]int{edgeA, edgeB}))
			continue
		}
		lower, upper := [2]*big.Rat{hit.Curve.Lower, hit.ArcLower}, [2]*big.Rat{hit.Curve.Upper, hit.ArcUpper}
		if swap {
			lower[0], lower[1] = lower[1], lower[0]
			upper[0], upper[1] = upper[1], upper[0]
		}
		var q AlgebraicSplineEdgeContact
		for axis, m := range maps {
			if lower[axis] == nil || upper[axis] == nil {
				r.Complete = false
				return r, geom.ErrConvergence
			}
			q.Lower[axis], q.Upper[axis] = m.bounds(lower[axis], upper[axis])
			q.AtEnd[axis] = q.Lower[axis].Cmp(q.Upper[axis]) == 0 && (q.Lower[axis].Cmp(ratInset(m.modelLo)) == 0 || q.Lower[axis].Cmp(ratInset(m.modelHi)) == 0)
		}
		r.Algebraic = append(r.Algebraic, q)
	}
	for _, overlap := range proof.Overlaps {
		lower, upper := [2]*big.Rat{ratInset(overlap.CurveSpan.Start), overlap.ArcStart}, [2]*big.Rat{ratInset(overlap.CurveSpan.End), overlap.ArcEnd}
		if swap {
			lower[0], lower[1] = lower[1], lower[0]
			upper[0], upper[1] = upper[1], upper[0]
		}
		var q QuadraticSplineEdgeOverlap
		var first, last [2]*big.Rat
		for axis, m := range maps {
			first[axis], last[axis] = m.model(lower[axis]), m.model(upper[axis])
			q.Lower[axis], q.Upper[axis] = m.bounds(lower[axis], upper[axis])
		}
		delta := new(big.Rat).Sub(last[0], first[0])
		if delta.Sign() == 0 {
			r.Complete = false
			return r, geom.ErrConvergence
		}
		q.Scale = new(big.Rat).Quo(new(big.Rat).Sub(last[1], first[1]), delta)
		q.Offset = new(big.Rat).Sub(first[1], new(big.Rat).Mul(q.Scale, first[0]))
		r.Overlaps = append(r.Overlaps, q)
	}
	sort.SliceStable(r.Contacts, func(i, j int) bool { return r.Contacts[i].Parameters[0].Cmp(r.Contacts[j].Parameters[0]) < 0 })
	sort.SliceStable(r.Algebraic, func(i, j int) bool { return r.Algebraic[i].Lower[0].Cmp(r.Algebraic[j].Lower[0]) < 0 })
	return r, e
}
