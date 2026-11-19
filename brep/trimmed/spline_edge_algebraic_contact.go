//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// AlgebraicSplineEdgeContact encloses both original model-edge parameters of
// one proved contact. It does not offer split anchors or a rounded model point.
// AtEnd is exact endpoint incidence. Differential properties remain unspecified.
type AlgebraicSplineEdgeContact struct {
	Lower, Upper [2]*big.Rat
	AtEnd        [2]bool
}

// AlgebraicSplineEdgeContactReport retains rational contacts separately from
// algebraic events and unresolved proof diagnostics. Complete is independent of
// representability as a topological split.
type AlgebraicSplineEdgeContactReport struct {
	Contacts               []SplineEdgeContact
	Algebraic              []AlgebraicSplineEdgeContact
	Proof                  geom.ExactCurveSegmentAlgebraicIntersection
	ProofSwapped, Complete bool
}

// IntersectSplineEdgesAlgebraic reports bounded exact contacts when one original
// trimmed model edge is affine in its parameter. Both parameter brackets are
// mapped back through original edge ranges and evaluator reversal exactly.
func IntersectSplineEdgesAlgebraic(ctx context.Context, a Solid, edgeA int, b Solid, edgeB int, o geom.ExactCurveSegmentOptions) (r AlgebraicSplineEdgeContactReport, err error) {
	if ctx == nil || edgeA < 0 || edgeA >= len(a.edges) || edgeB < 0 || edgeB >= len(b.edges) {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	x, y := a.edges[edgeA], b.edges[edgeB]
	options := geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	pa, ra, e := exactEdgeSplineWithOptions(x, options)
	if e != nil {
		return r, e
	}
	pb, rb, e := exactEdgeSplineWithOptions(y, options)
	if e != nil {
		return r, e
	}
	if pa == nil || pb == nil {
		return r, geom.ErrConvergence
	}
	proof, e := geom.IntersectExactCurveAffineLineAlgebraic(ctx, pa, pb, o)
	swap := false
	if e == geom.ErrConvergence && !proof.SupportingRootsComplete && len(proof.Hits) == 0 && len(proof.Pending) == 0 && len(proof.Coincident) == 0 {
		proof, e = geom.IntersectExactCurveAffineLineAlgebraic(ctx, pb, pa, o)
		swap = true
	}
	r.Proof, r.ProofSwapped, r.Complete = proof, swap, proof.Complete
	la, ha := pa.Domain()
	lb, hb := pb.Domain()
	maps := [2]splineEdgeParameterMap{{la, ha, x.Range.Min, x.Range.Max, ra}, {lb, hb, y.Range.Min, y.Range.Max, rb}}
	for _, hit := range proof.Hits {
		u, v := hit.Parameter, hit.OtherParameter
		if u == nil || v == nil {
			continue
		}
		if swap {
			u, v = v, u
		}
		q := edgeContactFromExact([2]*big.Rat{u, v}, hit.Point, maps, [2]int{edgeA, edgeB})
		q.Transverse, q.Tangent, q.Singular = hit.Transverse, hit.Tangent, hit.Singular
		r.Contacts = append(r.Contacts, q)
	}
	for _, hit := range proof.Algebraic {
		lower, upper := [2]*big.Rat{hit.Lower, hit.OtherLower}, [2]*big.Rat{hit.Upper, hit.OtherUpper}
		if swap {
			lower[0], lower[1] = lower[1], lower[0]
			upper[0], upper[1] = upper[1], upper[0]
		}
		if lower[0] == nil || lower[1] == nil || upper[0] == nil || upper[1] == nil {
			r.Complete = false
			return r, geom.ErrConvergence
		}
		var q AlgebraicSplineEdgeContact
		for axis, m := range maps {
			q.Lower[axis], q.Upper[axis] = m.bounds(lower[axis], upper[axis])
			q.AtEnd[axis] = q.Lower[axis].Cmp(q.Upper[axis]) == 0 && (q.Lower[axis].Cmp(ratInset(m.modelLo)) == 0 || q.Lower[axis].Cmp(ratInset(m.modelHi)) == 0)
		}
		r.Algebraic = append(r.Algebraic, q)
	}
	sort.SliceStable(r.Algebraic, func(i, j int) bool { return r.Algebraic[i].Lower[0].Cmp(r.Algebraic[j].Lower[0]) < 0 })
	sort.SliceStable(r.Contacts, func(i, j int) bool { return r.Contacts[i].Parameters[0].Cmp(r.Contacts[j].Parameters[0]) < 0 })
	return r, e
}
