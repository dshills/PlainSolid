package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// SplineEdgeContact describes an exact contact between two model edges.
// Parameters are in the original model-curve units, in operand order.
// Anchors are available only for exactly represented, strictly interior
// parameters with an exactly represented model point. No vertex is created.
type SplineEdgeContact struct {
	Parameters                    [2]*big.Rat
	Point                         [3]*big.Rat
	Anchors                       [2]EdgeAnchor
	Splittable                    bool
	Transverse, Tangent, Singular bool
	AtEnd                         [2]bool
}

// SplineEdgeContactReport distinguishes a complete finite event set from
// unresolved algebraic or coincident contacts. Contacts are in increasing
// parameter order on A, even if its model evaluator is reversed. Proof
// retains lower-level brackets and spans in the exact spline descriptors'
// parameter units. ProofSwapped says its curve order is B, A.
type SplineEdgeContactReport struct {
	Contacts     []SplineEdgeContact
	Proof        geom.ExactCurveSegmentIntersection
	ProofSwapped bool
	Complete     bool
}

// IntersectSplineEdges certifies edge contacts when at least one edge is an
// exact affine spline segment. Supported model curves include clamped rational
// splines and exact surface iso-curves. If a geometric contact cannot be
// represented as a split, its exact coordinates are still reported. This
// operation does not classify the incidence of the adjacent shell faces.
func IntersectSplineEdges(ctx context.Context, a Solid, edgeA int, b Solid, edgeB int,
	o geom.ExactCurveSegmentOptions) (SplineEdgeContactReport, error) {
	var r SplineEdgeContactReport
	if ctx == nil || edgeA < 0 || edgeA >= len(a.edges) || edgeB < 0 || edgeB >= len(b.edges) {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	x, y := a.edges[edgeA], b.edges[edgeB]
	pa, ra, e := exactEdgeSplineWithOptions(x, geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil || pa == nil {
		if e == nil {
			e = geom.ErrConvergence
		}
		return r, e
	}
	pb, rb, e := exactEdgeSplineWithOptions(y, geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil || pb == nil {
		if e == nil {
			e = geom.ErrConvergence
		}
		return r, e
	}
	p, e := geom.IntersectExactCurveAffineLine(ctx, pa, pb, o)
	swap := false
	if e == geom.ErrConvergence && len(p.Hits) == 0 && len(p.Pending) == 0 && len(p.Coincident) == 0 {
		p, e = geom.IntersectExactCurveAffineLine(ctx, pb, pa, o)
		swap = true
	}
	r.Proof, r.ProofSwapped, r.Complete = p, swap, p.Complete
	la, ha := pa.Domain()
	lb, hb := pb.Domain()
	maps := [2]splineEdgeParameterMap{{la, ha, x.Range.Min, x.Range.Max, ra}, {lb, hb, y.Range.Min, y.Range.Max, rb}}
	for _, hit := range p.Hits {
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
	sort.SliceStable(r.Contacts, func(i, j int) bool { return r.Contacts[i].Parameters[0].Cmp(r.Contacts[j].Parameters[0]) < 0 })
	return r, e
}

type splineEdgeParameterMap struct {
	lo, hi, modelLo, modelHi float64
	reverse                  bool
}

func (m splineEdgeParameterMap) model(t *big.Rat) *big.Rat {
	v := new(big.Rat).Sub(t, ratInset(m.lo))
	v.Quo(v, new(big.Rat).Sub(ratInset(m.hi), ratInset(m.lo)))
	if m.reverse {
		v.Sub(big.NewRat(1, 1), v)
	}
	v.Mul(v, new(big.Rat).Sub(ratInset(m.modelHi), ratInset(m.modelLo)))
	return v.Add(v, ratInset(m.modelLo))
}

func (m splineEdgeParameterMap) bounds(a, b *big.Rat) (*big.Rat, *big.Rat) {
	lo, hi := m.model(a), m.model(b)
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	return lo, hi
}

func edgeContactFromExact(t [2]*big.Rat, point [3]*big.Rat, maps [2]splineEdgeParameterMap, ids [2]int) (q SplineEdgeContact) {
	for i, p := range point {
		q.Point[i] = new(big.Rat).Set(p)
	}
	_, representable := geom.RepresentedExactPoint(q.Point)
	q.Splittable = representable == nil
	for i, m := range maps {
		q.Parameters[i] = m.model(t[i])
		t := q.Parameters[i]
		q.AtEnd[i] = t.Cmp(ratInset(m.modelLo)) == 0 || t.Cmp(ratInset(m.modelHi)) == 0
		v, e := geom.RepresentedExactScalar(t)
		q.Splittable = q.Splittable && e == nil && v > m.modelLo && v < m.modelHi
		q.Anchors[i] = EdgeAnchor{ids[i], v}
	}
	if !q.Splittable {
		q.Anchors = [2]EdgeAnchor{}
	}
	return q
}
