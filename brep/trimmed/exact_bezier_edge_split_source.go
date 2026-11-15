package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

func splitStoredBezierDescriptor(ctx context.Context, c PCurve, o edgecheck.Options) (ExactBezierUV, error) {
	if q, ok := c.(MultiSpanMappedSplineUV); ok {
		if len(q.parts) < 2 {
			return ExactBezierUV{}, geom.ErrInvalid
		}
		return q.curve, nil
	}
	if q, ok := c.(PiecewiseMappedSplineUV); ok {
		if q.station == nil {
			return ExactBezierUV{}, geom.ErrInvalid
		}
		return q.curve, nil
	}
	if q, ok := c.(MappedSplineUV); ok {
		if q.weight == nil {
			return ExactBezierUV{}, geom.ErrInvalid
		}
		return q.curve, nil
	}
	if q, ok := c.(ExactBezierUV); ok {
		return q, nil
	}
	if q, ok := c.(SplineUV); ok {
		result, e := mixedStoredHigherSplineUV(ctx, q, geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
		if e != nil {
			return ExactBezierUV{}, e
		}
		if exact, ok := result.(ExactBezierUV); ok {
			return exact, nil
		}
	}
	return ExactBezierUV{}, geom.ErrConvergence
}

// Preserve the authored spline rather than replacing it by the proof descriptor.
// local follows the coedge; the raw split follows SplineUV.Curve before its own
// reversal. Final native correspondence independently rechecks both restrictions.
func splitStoredBezierSources(ctx context.Context, source PCurve, local *big.Rat, a, b ExactBezierUV, o edgecheck.Options) (PCurve, PCurve, error) {
	if q, ok := source.(MultiSpanMappedSplineUV); ok {
		return splitMultiSpanMappedSource(ctx, q, local, a, b, geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
	}
	if q, ok := source.(PiecewiseMappedSplineUV); ok {
		return splitPiecewiseMappedSource(ctx, q, local, a, b, geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
	}
	if q, ok := source.(MappedSplineUV); ok {
		return splitMappedSplineSource(ctx, q, local, a, b, geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits})
	}
	q, ok := source.(SplineUV)
	if !ok {
		return a, b, nil
	}
	t := new(big.Rat).Set(local)
	if q.Reversed {
		t.Sub(big.NewRat(1, 1), t)
	}
	raw := new(big.Rat).Add(ratInset(q.Range.Min), new(big.Rat).Mul(t, new(big.Rat).Sub(ratInset(q.Range.Max), ratInset(q.Range.Min))))
	value, _ := raw.Float64()
	if !finite(value) || ratInset(value).Cmp(raw) != 0 {
		return nil, nil, geom.ErrUnrepresentable
	}
	if value <= q.Range.Min || value >= q.Range.Max {
		return nil, nil, geom.ErrConvergence
	}
	left, right := q, q
	if q.Reversed {
		left.Range.Min = value
		right.Range.Max = value
	} else {
		left.Range.Max = value
		right.Range.Min = value
	}
	return left, right, nil
}
