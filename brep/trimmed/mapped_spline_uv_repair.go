package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// Splitting phi_w at model t gives child coefficients A=1+(w-1)t
// and w/A, with original source cut phi_w(t). The map is positive throughout.
// Each child rebuilds from the unchanged original curve and exact raw interval;
// both complete curves are compared to independent homogeneous subdivision.
func splitMappedSplineSource(ctx context.Context, q MappedSplineUV, t *big.Rat, a, b ExactBezierUV, o geom.SurfacePatchOptions) (PCurve, PCurve, error) {
	raw, e := q.AuthoredParameter(t)
	if e != nil {
		return nil, nil, e
	}
	value, e := geom.RepresentedExactScalar(raw)
	if e != nil {
		return nil, nil, e
	}
	if value <= q.source.Range.Min || value >= q.source.Range.Max {
		return nil, nil, geom.ErrConvergence
	}
	left, right := q.source, q.source
	if left.Reversed {
		left.Range.Min = value
		right.Range.Max = value
	} else {
		left.Range.Max = value
		right.Range.Min = value
	}
	A := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(new(big.Rat).Sub(q.weight, big.NewRat(1, 1)), t))
	B := new(big.Rat).Quo(q.weight, A)
	x, e := NewMappedSplineUV(ctx, left, A, o)
	if e != nil {
		return nil, nil, e
	}
	y, e := NewMappedSplineUV(ctx, right, B, o)
	if e != nil {
		return nil, nil, e
	}
	for i, c := range []MappedSplineUV{x, y} {
		if e := proveMappedRepairCurve(ctx, c.curve, []ExactBezierUV{a, b}[i], o); e != nil {
			return nil, nil, e
		}
	}
	return x, y, nil
}

func proveMappedRepairCurve(ctx context.Context, a, b ExactBezierUV, o geom.SurfacePatchOptions) error {
	x, e := a.descriptor(ctx, o)
	if e != nil {
		return e
	}
	y, e := b.descriptor(ctx, o)
	if e != nil {
		return e
	}
	p, e := geom.CompareExactCurvesContext(ctx, x, y, geom.ExactCurveComparisonOptions{MaxSpans: 8, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return e
	}
	if !p.Complete || !p.Equal {
		return geom.ErrConvergence
	}
	return nil
}

// Joining requires exact original curve, range adjacency, orientation and map
// compatibility, not only matching endpoints. Its three source descriptor and
// two whole-curve comparisons each retain the existing bounded native merge
// limits; no sampled parameter fitting or replacement source is allowed.
func joinMappedSplineSources(ctx context.Context, a, b MappedSplineUV, t *big.Rat) (PCurve, [2]geom.Vec2, error) {
	var zero [2]geom.Vec2
	if a.weight == nil || b.weight == nil || !sameAuthoredBezierSpline(a.source.Curve, b.source.Curve) || a.source.Reversed != b.source.Reversed || a.source.ExactEndpoints != b.source.ExactEndpoints {
		return nil, zero, geom.ErrConvergence
	}
	q := a.source
	if q.Reversed {
		if a.source.Range.Min != b.source.Range.Max {
			return nil, zero, geom.ErrConvergence
		}
		q.Range.Min = b.source.Range.Min
	} else {
		if a.source.Range.Max != b.source.Range.Min {
			return nil, zero, geom.ErrConvergence
		}
		q.Range.Max = b.source.Range.Max
	}
	weight := new(big.Rat).Mul(a.weight, b.weight)
	A := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(new(big.Rat).Sub(weight, big.NewRat(1, 1)), t))
	if A.Cmp(a.weight) != 0 {
		return nil, zero, geom.ErrConvergence
	}
	o := geom.SurfacePatchOptions{MaxOperations: 1000000, MaxCoefficientBits: 16384}
	whole, e := NewMappedSplineUV(ctx, q, weight, o)
	if e != nil {
		return nil, zero, e
	}
	raw, e := whole.AuthoredParameter(t)
	if e != nil {
		return nil, zero, e
	}
	expected := a.source.Range.Max
	if q.Reversed {
		expected = a.source.Range.Min
	}
	if raw.Cmp(ratInset(expected)) != 0 {
		return nil, zero, geom.ErrConvergence
	}
	x, y, e := whole.curve.Split(t)
	if e != nil {
		return nil, zero, e
	}
	for i, c := range []MappedSplineUV{a, b} {
		if e := proveMappedRepairCurve(ctx, c.curve, []ExactBezierUV{x, y}[i], o); e != nil {
			return nil, zero, e
		}
	}
	return whole, whole.curve.ends, nil
}
