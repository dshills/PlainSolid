package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// SplineEdgeProjectedEvent bounds an isolated intersection on both model
// edges in their original units. Exact is nil for an unrecovered algebraic
// point. At a knot, adjacent proof spans can report the same endpoint.
type SplineEdgeProjectedEvent struct {
	Lower, Upper        [2]*big.Rat
	Exact               *SplineEdgeContact
	Transverse, Tangent bool
}

// SplineEdgeProjectedOverlap bounds a coincident interval on each edge.
// Reversed reports whether correspondence runs in opposite directions.
type SplineEdgeProjectedOverlap struct {
	Lower, Upper [2]*big.Rat
	Reversed     bool
}

// SplineEdgeProjectionReport retains the original descriptor-level proof
// and the same events mapped into the original model edge parameter units.
type SplineEdgeProjectionReport struct {
	Events     []SplineEdgeProjectedEvent
	Coincident []SplineEdgeProjectedOverlap
	Proof      geom.AffineProjectionCurveIntersection
	Complete   bool
}

// IntersectSplineEdgesByProjection proves spatial intersections between
// two exact spline edges with a common nonconstant affine coordinate along
// the selected model-space covector. Speeds, edge ranges and directions can
// differ. Isolated algebraic roots retain exact brackets; only representable
// rational interior events yield anchors. This does not classify shell faces.
func IntersectSplineEdgesByProjection(ctx context.Context, a Solid, edgeA int, b Solid, edgeB int, covector geom.Vec3, o geom.ExactCurveSegmentOptions) (SplineEdgeProjectionReport, error) {
	var r SplineEdgeProjectionReport
	if ctx == nil || edgeA < 0 || edgeA >= len(a.edges) || edgeB < 0 || edgeB >= len(b.edges) || !covector.IsFinite() {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	x, y := a.edges[edgeA], b.edges[edgeB]
	p := geom.SurfacePatchOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}
	pa, ra, e := exactEdgeSplineWithOptions(x, p)
	if e != nil {
		return r, e
	}
	pb, rb, e := exactEdgeSplineWithOptions(y, p)
	if e != nil {
		return r, e
	}
	if pa == nil || pb == nil {
		return r, geom.ErrConvergence
	}
	loA, hiA := pa.Domain()
	loB, hiB := pb.Domain()
	maps := [2]splineEdgeParameterMap{{loA, hiA, x.Range.Min, x.Range.Max, ra}, {loB, hiB, y.Range.Min, y.Range.Max, rb}}
	r.Proof, e = geom.IntersectExactCurvesAffineProjection(ctx, pa, pb, covector, o)
	r.Complete = r.Proof.Complete
	for _, hit := range r.Proof.Hits {
		event := SplineEdgeProjectedEvent{Transverse: hit.Evidence.Transverse, Tangent: hit.Evidence.Tangent}
		for i, m := range maps {
			event.Lower[i], event.Upper[i] = m.bounds(hit.Lower[i], hit.Upper[i])
		}
		if hit.Parameters[0] != nil && hit.Parameters[1] != nil {
			q := edgeContactFromExact(hit.Parameters, hit.Evidence.Point, maps, [2]int{edgeA, edgeB})
			q.Transverse, q.Tangent = event.Transverse, event.Tangent
			event.Exact = &q
		}
		r.Events = append(r.Events, event)
	}
	sort.SliceStable(r.Events, func(i, j int) bool { return r.Events[i].Lower[0].Cmp(r.Events[j].Lower[0]) < 0 })
	for _, overlap := range r.Proof.Coincident {
		var q SplineEdgeProjectedOverlap
		for i, m := range maps {
			q.Lower[i], q.Upper[i] = m.bounds(overlap.Lower[i], overlap.Upper[i])
		}
		q.Reversed = (r.Proof.Maps[0].Scale.Sign() != r.Proof.Maps[1].Scale.Sign()) != (ra != rb)
		r.Coincident = append(r.Coincident, q)
	}
	sort.SliceStable(r.Coincident, func(i, j int) bool { return r.Coincident[i].Lower[0].Cmp(r.Coincident[j].Lower[0]) < 0 })
	return r, e
}
