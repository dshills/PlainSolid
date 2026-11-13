package intersect

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// CertifyEndpointOnEdge checks an exact four-parameter endpoint proposal
// against an original source face boundary. end is 0 or 1 in this segment's
// traversal direction; chart is 0 for A or 1 for B. The returned normalized
// edge coordinate increases with the original chart's free edge coordinate.
func (s TubeSegment) CertifyEndpointOnEdge(ctx context.Context, end, chart int, domain geom.ParameterRectangle,
	edge geom.SurfaceEdge, hint [4]*big.Rat, o geom.TubeEndpointEdgeOptions) (geom.TubeEndpointEdgeCertificate, error) {
	if ctx == nil || s.pa == nil || end < 0 || end > 1 || chart < 0 || chart > 1 {
		return geom.TubeEndpointEdgeCertificate{}, geom.ErrInvalid
	}
	upper := (end == 1) != s.reversed
	return s.proof.CertifyEndpointOnEdge(ctx, upper, chart, domain, edge, hint, o)
}

// CertifyEndpoint verifies exact rational parameters for the selected
// endpoint in traversal order, without requiring an original chart boundary.
func (s TubeSegment) CertifyEndpoint(ctx context.Context, end int, hint [4]*big.Rat, o geom.TubeEndpointOptions) (geom.TubeEndpointCertificate, error) {
	if ctx == nil || s.pa == nil || end < 0 || end > 1 {
		return geom.TubeEndpointCertificate{}, geom.ErrInvalid
	}
	return s.proof.CertifyEndpoint(ctx, (end == 1) != s.reversed, hint, o)
}

// CompareTubeEndpoints verifies two exact endpoint proposals, possibly from
// different spline support pairs, and compares their exact model positions.
func CompareTubeEndpoints(ctx context.Context, a, b TubeSegment, endA, endB int,
	hintA, hintB [4]*big.Rat, o geom.TubeEndpointOptions) (geom.TubeEndpointComparison, error) {
	if ctx == nil || a.pa == nil || b.pa == nil || endA < 0 || endA > 1 || endB < 0 || endB > 1 {
		return geom.TubeEndpointComparison{}, geom.ErrInvalid
	}
	return geom.CompareVerifiedTubeEndpoints(ctx, a.proof, b.proof, (endA == 1) != a.reversed,
		(endB == 1) != b.reversed, hintA, hintB, o)
}

// WithExactEndpoint returns a copy whose chosen endpoint uses verified exact
// coordinates. This removes representative rounding differences when sharing
// a topological vertex across independently certified surface pairs. The
// original proof recipe is retained with the represented hint for reconstruction.
func (s TubeSegment) WithExactEndpoint(ctx context.Context, end int, hint [4]*big.Rat, o geom.TubeEndpointOptions) (TubeSegment, error) {
	q, e := s.CertifyEndpoint(ctx, end, hint, o)
	if e != nil {
		return TubeSegment{}, e
	}
	if !q.Proven {
		return TubeSegment{}, geom.ErrConvergence
	}
	var hintValues [4]float64
	for i, x := range q.Parameters {
		f, e := geom.RepresentedExactScalar(x)
		if e != nil {
			return TubeSegment{}, geom.ErrUnrepresentable
		}
		hintValues[i] = f
	}
	data := s.recipe.Data()
	if !validTubeRecipeEndpoint(data, s.index, end) {
		return TubeSegment{}, geom.ErrInvalid
	}
	for i, h := range data.ExactEndpoints {
		if h.Segment == s.index && h.End == end {
			data.ExactEndpoints = append(data.ExactEndpoints[:i], data.ExactEndpoints[i+1:]...)
			break
		}
	}
	data.ExactEndpoints = append(data.ExactEndpoints, TubeExactEndpoint{s.index, end, hintValues})
	s.recipe = TubeRecipe{&data}
	return s.withCertifiedEndpoint(end, q), nil
}

func validTubeRecipeEndpoint(data TubeRecipeData, segment, end int) bool {
	return !data.Closed && (segment == 0 && end == 0 || segment == len(data.Cells)-1 && end == 1)
}

func (s TubeSegment) withCertifiedEndpoint(end int, q geom.TubeEndpointCertificate) TubeSegment {
	r := s.ends[end]
	r.ParametersA, r.ParametersB, r.Bounds = q.ParameterBoundsA, q.ParameterBoundsB, q.PointBounds
	r.Point = q.PointBounds.Min.Scale(.5).Add(q.PointBounds.Max.Scale(.5))
	s.ends[end] = r
	return s
}
