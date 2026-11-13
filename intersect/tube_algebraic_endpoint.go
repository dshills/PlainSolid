package intersect

import (
	"context"
	"plainsolid/geom"
)

// CertifyAlgebraicGraphEndpoint checks a source-isocurve parameter root at end
// 0/1 in segment traversal order. It does not change rational endpoint hints,
// cached display representatives, or the segment's construction recipe.
func (s TubeSegment) CertifyAlgebraicGraphEndpoint(ctx context.Context, end int, proposal *geom.ExactAlgebraicRoot, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphEndpointCertificate, error) {
	if ctx == nil || s.pa == nil || end < 0 || end > 1 {
		return geom.TubeAlgebraicGraphEndpointCertificate{}, geom.ErrInvalid
	}
	return s.proof.CertifyAlgebraicGraphEndpoint(ctx, (end == 1) != s.reversed, proposal, o)
}

// TubeAlgebraicEndpointComparison retains independently proved model-event
// lineages. ProvenEqual requires original ordered support chart identity and
// equality of all four original coordinates. ParametersDiffer does not by itself
// prove different model points on an arbitrary noninjective source chart.
type TubeAlgebraicEndpointComparison struct {
	ProvenEqual, SupportsEqual, ParametersDiffer bool
	Left, Right                                  geom.TubeAlgebraicGraphEndpointCertificate
	ExactOperations                              int
}

// CompareTubeAlgebraicGraphEndpoints accepts roots, not mutable prior reports.
// Original supports must have matching domains and exact same-parameter chart
// geometry; degree/knot representations may differ. Work is cumulative.
func CompareTubeAlgebraicGraphEndpoints(ctx context.Context, left, right TubeSegment, endLeft, endRight int, rootLeft, rootRight *geom.ExactAlgebraicRoot, o geom.TubeAlgebraicGraphEndpointOptions) (r TubeAlgebraicEndpointComparison, err error) {
	if ctx == nil || left.a == nil || left.b == nil || right.a == nil || right.b == nil || endLeft < 0 || endLeft > 1 || endRight < 0 || endRight > 1 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, geom.ErrInvalid
	}
	for _, pair := range [][2]*geom.BSplineSurface{{left.a, right.a}, {left.b, right.b}} {
		if pair[0] == pair[1] {
			continue
		}
		var patches [2]*geom.SurfacePatch
		for i, surface := range pair {
			remaining := o.MaxOperations - r.ExactOperations
			if remaining < 1 {
				return r, geom.ErrConvergence
			}
			p, e := geom.NewSurfacePatch(surface, geom.SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return r, e
			}
			r.ExactOperations += p.ExactOperations()
			patches[i] = p
		}
		if patches[0].Domain() != patches[1].Domain() {
			return r, nil
		}
		remaining := o.MaxOperations - r.ExactOperations
		if remaining < 1 {
			return r, geom.ErrConvergence
		}
		identity, e := geom.CompareExactSurfaces(ctx, patches[0], patches[1], geom.ExactSurfaceComparisonOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		r.ExactOperations += identity.ExactOperations
		if e != nil {
			return r, e
		}
		if !identity.Complete {
			return r, geom.ErrConvergence
		}
		if !identity.Equal {
			return r, nil
		}
	}
	r.SupportsEqual = true
	endpoints := []struct {
		segment TubeSegment
		end     int
		root    *geom.ExactAlgebraicRoot
	}{{left, endLeft, rootLeft}, {right, endRight, rootRight}}
	for i, endpoint := range endpoints {
		child := o
		child.MaxOperations -= r.ExactOperations
		if child.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		certificate, e := endpoint.segment.CertifyAlgebraicGraphEndpoint(ctx, endpoint.end, endpoint.root, child)
		r.ExactOperations += certificate.ExactOperations
		if i == 0 {
			r.Left = certificate
		} else {
			r.Right = certificate
		}
		if e != nil {
			return r, e
		}
		if !certificate.Proven {
			return r, geom.ErrConvergence
		}
	}
	for axis := 0; axis < 4; axis++ {
		remaining := o.MaxOperations - r.ExactOperations
		if remaining < 1 {
			return r, geom.ErrConvergence
		}
		comparison, e := r.Left.Parameters[axis].Compare(ctx, r.Right.Parameters[axis], geom.ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
		r.ExactOperations += comparison.Operations
		if e != nil {
			return r, e
		}
		if !comparison.Proven {
			return r, geom.ErrConvergence
		}
		if comparison.Order != 0 {
			r.ParametersDiffer = true
		}
	}
	r.ProvenEqual = !r.ParametersDiffer
	return r, nil
}

// DiscoverAlgebraicGraphEndpoint isolates and verifies the exact algebraic
// endpoint without a proposal. end is 0/1 in this segment's traversal order.
func (s TubeSegment) DiscoverAlgebraicGraphEndpoint(ctx context.Context, end int, o geom.TubeAlgebraicGraphEndpointOptions) (geom.TubeAlgebraicGraphEndpointCertificate, error) {
	if ctx == nil || s.pa == nil || end < 0 || end > 1 {
		return geom.TubeAlgebraicGraphEndpointCertificate{}, geom.ErrInvalid
	}
	return s.proof.DiscoverAlgebraicGraphEndpoint(ctx, (end == 1) != s.reversed, o)
}
