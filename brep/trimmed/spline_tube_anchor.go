package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// SplineTubeAnchor pairs an original topological edge parameter with the
// exact section endpoint used to prove it. The parameter is in model curve
// units and is suitable for SplitSplineTubeAcrossEdges.
type SplineTubeAnchor struct {
	Anchor EdgeAnchor
	Proof  geom.TubeEndpointEdgeCertificate
}

// SplineTubeEndpoint identifies which chart side contains a path endpoint.
// Hint gives exact parameters [uA,vA,uB,vB].
type SplineTubeEndpoint struct {
	Side geom.SurfaceEdge
	Hint [4]*big.Rat
}

// SplitSplineTubeAcrossCertifiedEdges verifies both original edge anchors
// and propagates their splits to every incident face before applying the
// tube path. The original solid is returned untouched on error.
func (s Solid) SplitSplineTubeAcrossCertifiedEdges(ctx context.Context, face int, segments []intersect.TubeSegment,
	chart int, start, end SplineTubeEndpoint, o geom.TubeEndpointEdgeOptions) (Solid, AnchoredFaceSplit, error) {
	if ctx == nil || len(segments) == 0 || len(segments) > 64 {
		return Solid{}, AnchoredFaceSplit{}, geom.ErrInvalid
	}
	a, e := s.CertifySplineTubeAnchor(ctx, face, segments[0], 0, chart, start.Side, start.Hint, o)
	if e != nil {
		return Solid{}, AnchoredFaceSplit{}, e
	}
	b, e := s.CertifySplineTubeAnchor(ctx, face, segments[len(segments)-1], 1, chart, end.Side, end.Hint, o)
	if e != nil {
		return Solid{}, AnchoredFaceSplit{}, e
	}
	return s.splitSplineTubeAcrossEdges(ctx, face, a.Anchor, b.Anchor, segments, chart == 1, func(s Solid, edge int, parameter float64) (Solid, EdgeSplit, error) {
		return s.SplitSplineEdgeExact(ctx, edge, parameter, edgecheck.Options{MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits})
	})
}

// CertifySplineTubeAnchor maps an exact tube endpoint proposal onto the
// original topological edge of a rectangular spline face. The target edge
// use must agree continuously and exactly with its model curve. Endpoint UV,
// model point and edge split parameter must be representable. Existing
// vertices and curved/non-isoparametric trims are outside this operation.
func (s Solid) CertifySplineTubeAnchor(ctx context.Context, face int, segment intersect.TubeSegment, end, chart int,
	side geom.SurfaceEdge, hint [4]*big.Rat, o geom.TubeEndpointEdgeOptions) (SplineTubeAnchor, error) {
	return s.certifySplineTubeAnchor(ctx, face, segment, end, chart, side, hint, o, false)
}

func (s Solid) certifySplineTubeAnchor(ctx context.Context, face int, segment intersect.TubeSegment, end, chart int,
	side geom.SurfaceEdge, hint [4]*big.Rat, o geom.TubeEndpointEdgeOptions, allowDependentSide bool) (SplineTubeAnchor, error) {
	var out SplineTubeAnchor
	if ctx == nil || face < 0 || face >= len(s.faces) || chart < 0 || chart > 1 || end < 0 || end > 1 || side > geom.SurfaceEdgeVMax {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	f := s.faces[face]
	base, ok := f.Surface.(*geom.BSplineSurface)
	trim := segment.TrimA()
	if chart == 1 {
		trim = segment.TrimB()
	}
	if !ok || !geom.SameSplineSurface(base, trim.Support()) || len(f.Outer) != 4 || len(f.Holes) != 0 {
		return out, geom.ErrInvalid
	}
	d, e := faceUVBounds(f)
	if e != nil {
		return out, e
	}
	corners := insetCorners(d)
	start := [4]int{3, 1, 0, 2}[side]
	a, b := corners[start], corners[(start+1)%4]
	var use Use
	for _, u := range f.Outer {
		if u.Curve != nil {
			return out, geom.ErrInvalid
		}
		if u.UVStart == a && u.UVEnd == b {
			use = u
		}
	}
	if use.UVStart != a || use.UVEnd != b {
		return out, geom.ErrInvalid
	}
	p, e := segment.CertifyEndpointOnEdge(ctx, end, chart, d, side, hint, o)
	if allowDependentSide && e == geom.ErrConvergence {
		var q geom.TubeEndpointCertificate
		q, e = segment.CertifyEndpoint(ctx, end, hint, o)
		if e != nil {
			return out, e
		}
		fixed, lo, hi := 0, d.VMin, d.VMax
		if side >= geom.SurfaceEdgeVMin {
			fixed, lo, hi = 1, d.UMin, d.UMax
		}
		boundary := [4]float64{d.UMin, d.UMax, d.VMin, d.VMax}[side]
		if q.Parameters[2*chart+fixed].Cmp(ratInset(boundary)) != 0 {
			return out, geom.ErrDomain
		}
		v := q.Parameters[2*chart+1-fixed]
		alpha := new(big.Rat).Quo(new(big.Rat).Sub(v, ratInset(lo)), new(big.Rat).Sub(ratInset(hi), ratInset(lo)))
		p = geom.TubeEndpointEdgeCertificate{Proven: q.Proven, Parameters: q.Parameters, Point: q.Point,
			EdgeParameter: new(big.Rat).Set(v), NormalizedEdgeParameter: alpha,
			ParameterBoundsA: q.ParameterBoundsA, ParameterBoundsB: q.ParameterBoundsB, PointBounds: q.PointBounds, ExactOperations: q.ExactOperations}
	}
	if e != nil {
		return out, e
	}
	if !p.Proven {
		return out, geom.ErrConvergence
	}
	if p.NormalizedEdgeParameter.Sign() <= 0 || p.NormalizedEdgeParameter.Cmp(big.NewRat(1, 1)) >= 0 {
		return out, geom.ErrDomain
	}
	edge := s.edges[use.Edge]
	if !sameSplineTrim(edge, f.Surface, use) {
		match, e := compareSplineEdgeUse(ctx, edge, f.Surface, use, edgecheck.Options{
			MaxWork: o.MaxOperations, MaxBits: o.MaxCoefficientBits,
		})
		if e != nil {
			return out, e
		}
		if !match {
			return out, fmt.Errorf("trimmed: tube anchor model edge identity not certified: %w", geom.ErrConvergence)
		}
	}
	t := new(big.Rat).Set(p.NormalizedEdgeParameter)
	if (side == geom.SurfaceEdgeUMin || side == geom.SurfaceEdgeVMax) != use.Reversed {
		t.Sub(big.NewRat(1, 1), t)
	}
	parameter, exact := exactRectEdgeMix(t, edge.Range.Min, edge.Range.Max)
	if !exact || parameter <= edge.Range.Min || parameter >= edge.Range.Max {
		return out, geom.ErrUnrepresentable
	}
	for _, x := range append(p.Parameters[:], p.Point[:]...) {
		if _, e := geom.RepresentedExactScalar(x); e != nil {
			return out, geom.ErrUnrepresentable
		}
	}
	return SplineTubeAnchor{Anchor: EdgeAnchor{Edge: use.Edge, Parameter: parameter}, Proof: p}, nil
}
