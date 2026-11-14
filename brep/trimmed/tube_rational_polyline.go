package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type RationalTubePolylineOptions struct {
	// PlanarPointEvaluation uses exact rational point recovery on a planar opposite
	// support whose original chart need not be affine. Exclusive with Projection.
	PlanarPointEvaluation bool
	// Projection optionally chooses the two affine model coordinates used for
	// exact graph isolation. Nil preserves the original XY contract. Interval
	// bounds and displayed samples always remain in original XYZ coordinates.
	// Projected isolation may use an established HasBounds enclosure when only
	// its optional refinement limit is reached; the final chord proof still
	// must meet Deflection. Interval.MaxIterations bounds that refinement.
	Projection                                         *[2]int
	Deflection                                         float64
	MaxSegments, MaxDepth, MaxOperations, MaxInputBits int
	Point                                              geom.TubeAlgebraicGraphEndpointOptions
	Interval                                           geom.SurfaceTubeIntervalOptions
	EndpointPoints                                     *[2]geom.Vec3 // Optional shared displays, included in every incident bound.
}

type CertifiedTubeGraphSample struct {
	Free     *big.Rat
	Point    geom.Vec3
	Identity geom.TubeAlgebraicGraphPointCertificate
	Model    geom.AlgebraicCurveEndpointBounds
}

type CertifiedRationalTubePolyline struct {
	FreeAxis          int
	TraversalReversed bool
	Samples           []CertifiedTubeGraphSample // Increasing exact original free parameter.
	SquaredErrors     []*big.Rat
	Enclosures        []geom.SurfaceTubeIntervalEnclosure
	ExactOperations   int
}

type rationalTubeGraph interface {
	ExactFreeDomain() (int, geom.ExactParameterInterval, error)
	EvaluateRationalGraph(context.Context, *big.Rat, geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphPointCertificate, error)
	EncloseInterval(context.Context, float64, float64, geom.SurfaceTubeIntervalOptions) (geom.SurfaceTubeIntervalEnclosure, error)
}
type segmentGraphAdapter struct{ segment intersect.TubeSegment }

func (s segmentGraphAdapter) ExactFreeDomain() (int, geom.ExactParameterInterval, error) {
	axis, domain, _, e := s.segment.ExactFreeDomain()
	return axis, domain, e
}
func (s segmentGraphAdapter) EvaluateRationalGraph(c context.Context, q *big.Rat, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphPointCertificate, error) {
	return s.segment.EvaluateRationalGraph(c, q, o)
}
func (s segmentGraphAdapter) EncloseInterval(c context.Context, a, b float64, o geom.SurfaceTubeIntervalOptions) (geom.SurfaceTubeIntervalEnclosure, error) {
	return s.segment.EncloseFreeInterval(c, a, b, o)
}

// TessellateRationalTubeGraphCertified covers a closed rational free interval
// of an immutable unique tube branch. Exact graph-point identities are retained;
// no nonbinary free coordinate is rounded for point construction. Whole-branch
// boxes use outward represented interval limits. Convex box-to-chord bounds
// certify pointwise correspondence (hence both directed distances), including
// endpoint display error. This conservative first-order bound has no curvature
// efficiency claim and returns no partial polyline on limits.
func TessellateRationalTubeGraphCertified(ctx context.Context, tube geom.VerifiedSurfaceTube, start, end *big.Rat, o RationalTubePolylineOptions) (CertifiedRationalTubePolyline, error) {
	if o.PlanarPointEvaluation && o.Projection != nil {
		return CertifiedRationalTubePolyline{}, geom.ErrInvalid
	}
	if o.PlanarPointEvaluation {
		return tessellateRationalTubeGraph(ctx, planeVerifiedPolylineGraph{tube}, start, end, o)
	}
	if o.Projection != nil {
		if !validTubePolylineProjection(*o.Projection) {
			return CertifiedRationalTubePolyline{}, geom.ErrInvalid
		}
		return tessellateRationalTubeGraph(ctx, projectedVerifiedPolylineGraph{tube, *o.Projection}, start, end, o)
	}
	return tessellateRationalTubeGraph(ctx, tube, start, end, o)
}
func TessellateRationalTubeSegmentCertified(ctx context.Context, tube intersect.TubeSegment, start, end *big.Rat, o RationalTubePolylineOptions) (CertifiedRationalTubePolyline, error) {
	var graph rationalTubeGraph = segmentGraphAdapter{tube}
	if o.PlanarPointEvaluation && o.Projection != nil {
		return CertifiedRationalTubePolyline{}, geom.ErrInvalid
	}
	if o.PlanarPointEvaluation {
		graph = planeSegmentPolylineGraph{segmentGraphAdapter{tube}}
	}
	if o.Projection != nil {
		if !validTubePolylineProjection(*o.Projection) {
			return CertifiedRationalTubePolyline{}, geom.ErrInvalid
		}
		graph = projectedSegmentPolylineGraph{segmentGraphAdapter{tube}, *o.Projection}
	}
	r, e := tessellateRationalTubeGraph(ctx, graph, start, end, o)
	if e == nil {
		_, _, r.TraversalReversed, e = tube.ExactFreeDomain()
	}
	return r, e
}
func tessellateRationalTubeGraph(ctx context.Context, tube rationalTubeGraph, start, end *big.Rat, o RationalTubePolylineOptions) (CertifiedRationalTubePolyline, error) {
	var empty CertifiedRationalTubePolyline
	if ctx == nil || start == nil || end == nil || start.Cmp(end) >= 0 || !finite(o.Deflection) || o.Deflection <= 0 || !finite(o.Point.Tolerance) || o.Point.Tolerance < 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.EndpointPoints != nil && (!o.EndpointPoints[0].IsFinite() || !o.EndpointPoints[1].IsFinite()) {
		return empty, geom.ErrInvalid
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxInputBits == 0 {
		o.MaxInputBits = 4096
	}
	if o.MaxSegments < 1 || o.MaxSegments > 65536 || o.MaxDepth < 1 || o.MaxDepth > 64 || o.MaxOperations < 1 || o.MaxOperations > 1000000000 || o.MaxInputBits < 64 || o.MaxInputBits > 65536 {
		return empty, geom.ErrInvalid
	}
	for _, q := range []*big.Rat{start, end} {
		if q.Num().BitLen() > o.MaxInputBits || q.Denom().BitLen() > o.MaxInputBits {
			return empty, geom.ErrConvergence
		}
	}
	axis, domain, e := tube.ExactFreeDomain()
	if e != nil {
		return empty, e
	}
	if start.Cmp(domain.Lower) < 0 || end.Cmp(domain.Upper) > 0 {
		return empty, geom.ErrDomain
	}
	result := CertifiedRationalTubePolyline{FreeAxis: axis}
	target := ratInset(o.Deflection)
	target.Mul(target, target)
	remaining := func() (int, error) {
		n := o.MaxOperations - result.ExactOperations
		if n < 1 {
			return 0, geom.ErrConvergence
		}
		return n, nil
	}
	evaluate := func(q *big.Rat, override *geom.Vec3) (CertifiedTubeGraphSample, error) {
		var bad CertifiedTubeGraphSample
		n, e := remaining()
		if e != nil {
			return bad, e
		}
		options := o.Point
		if options.MaxOperations == 0 || options.MaxOperations > n {
			options.MaxOperations = n
		}
		proof, e := tube.EvaluateRationalGraph(ctx, q, options)
		result.ExactOperations += proof.ExactOperations
		if e != nil {
			return bad, e
		}
		if !proof.Proven || proof.ModelEvent == nil {
			return bad, geom.ErrConvergence
		}
		n, e = remaining()
		if e != nil {
			return bad, e
		}
		tolerance := o.Deflection / 16
		if tolerance == 0 {
			tolerance = o.Deflection
		}
		if o.Point.Tolerance > 0 && o.Point.Tolerance < tolerance {
			tolerance = o.Point.Tolerance
		}
		model, e := proof.ModelEvent.Enclose(ctx, geom.ComposedCurveEndpointOptions{Tolerance: tolerance, ExactCurvePlaneEventOptions: geom.ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: o.Point.MaxCoefficientBits}})
		result.ExactOperations += model.ExactOperations
		if e != nil {
			return bad, e
		}
		if !model.Converged || !model.HasRepresentative {
			return bad, geom.ErrConvergence
		}
		point := model.Point
		if override != nil {
			point = *override
		}
		if !point.IsFinite() {
			return bad, geom.ErrInvalid
		}
		return CertifiedTubeGraphSample{new(big.Rat).Set(q), point, proof, model}, nil
	}
	var endpoints [2]*geom.Vec3
	if o.EndpointPoints != nil {
		endpoints[0], endpoints[1] = &o.EndpointPoints[0], &o.EndpointPoints[1]
	}
	first, e := evaluate(start, endpoints[0])
	if e != nil {
		return empty, e
	}
	last, e := evaluate(end, endpoints[1])
	if e != nil {
		return empty, e
	}
	result.Samples = append(result.Samples, first)
	type item struct {
		a, b  CertifiedTubeGraphSample
		depth int
	}
	stack := []item{{first, last, 0}}
	for len(stack) > 0 {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(result.SquaredErrors)+len(stack) >= o.MaxSegments {
			return empty, geom.ErrConvergence
		}
		lo, _ := x.a.Free.Float64()
		hi, _ := x.b.Free.Float64()
		if !finite(lo) || !finite(hi) {
			return empty, geom.ErrUnrepresentable
		}
		if ratInset(lo).Cmp(x.a.Free) > 0 {
			lo = math.Nextafter(lo, math.Inf(-1))
		}
		if ratInset(hi).Cmp(x.b.Free) < 0 {
			hi = math.Nextafter(hi, math.Inf(1))
		}
		if lo >= hi {
			return empty, geom.ErrUnrepresentable
		}
		n, e := remaining()
		if e != nil {
			return empty, e
		}
		options := o.Interval
		if options.MaxOperations == 0 || options.MaxOperations > n {
			options.MaxOperations = n
		}
		enclosure, e := tube.EncloseInterval(ctx, lo, hi, options)
		result.ExactOperations += enclosure.ExactOperations
		if e != nil {
			return empty, e
		}
		if !enclosure.HasBounds || !enclosure.Bounds.Valid() {
			return empty, geom.ErrConvergence
		}
		squared := new(big.Rat)
		for _, point := range []geom.Vec3{x.a.Point, x.b.Point} {
			sum := new(big.Rat)
			p, a, b := exactBoundPoint(point), exactBoundPoint(enclosure.Bounds.Min), exactBoundPoint(enclosure.Bounds.Max)
			for axis := 0; axis < 3; axis++ {
				d := new(big.Rat).Sub(a[axis], p[axis])
				d.Mul(d, d)
				z := new(big.Rat).Sub(b[axis], p[axis])
				z.Mul(z, z)
				if z.Cmp(d) > 0 {
					d = z
				}
				sum.Add(sum, d)
			}
			if sum.Cmp(squared) > 0 {
				squared.Set(sum)
			}
		}
		if squared.Cmp(target) <= 0 {
			if x.a.Point == x.b.Point {
				return empty, geom.ErrDegenerate
			}
			result.Samples = append(result.Samples, x.b)
			result.SquaredErrors = append(result.SquaredErrors, squared)
			result.Enclosures = append(result.Enclosures, enclosure)
			continue
		}
		if x.depth >= o.MaxDepth {
			return empty, geom.ErrConvergence
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(x.a.Free, x.b.Free), big.NewRat(2, 1))
		if mid.Num().BitLen() > o.MaxInputBits || mid.Denom().BitLen() > o.MaxInputBits {
			return empty, geom.ErrConvergence
		}
		sample, e := evaluate(mid, nil)
		if e != nil {
			return empty, e
		}
		stack = append(stack, item{sample, x.b, x.depth + 1}, item{x.a, sample, x.depth + 1})
	}
	return result, nil
}

type planeVerifiedPolylineGraph struct{ geom.VerifiedSurfaceTube }

func (p planeVerifiedPolylineGraph) EvaluateRationalGraph(ctx context.Context, q *big.Rat, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphPointCertificate, error) {
	return p.EvaluateRationalPlane(ctx, q, o)
}

type planeSegmentPolylineGraph struct{ segmentGraphAdapter }

func (p planeSegmentPolylineGraph) EvaluateRationalGraph(ctx context.Context, q *big.Rat, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphPointCertificate, error) {
	return p.segment.EvaluateRationalPlane(ctx, q, o)
}
