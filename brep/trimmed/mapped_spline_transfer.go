package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ExactTrimModelBezier composes an original bounded pcurve with its single
// Bézier support. The output uses normalized trim traversal or its exact
// reverse and retains the rational model coefficients. The support may use
// a represented affine placement.
func ExactTrimModelBezier(ctx context.Context, support geom.Surface, trim PCurve, reverse bool, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	if ctx == nil || support == nil || trim == nil {
		return nil, geom.ErrInvalid
	}
	p, err := exactFaceSupportPatch(Face{Surface: support}, o)
	if err != nil {
		return nil, err
	}
	c, err := polynomialTrimDescriptor(ctx, trim, reverse, o)
	if err != nil {
		return nil, err
	}
	return p.ComposeParameterCurve(ctx, c, o)
}

// ProveMappedSplineTransfer recovers a positive owner speed for an original
// directed cubic or quartic spline restriction and a standard model Bézier
// edge. The endpoint tangent proposes the map. Whole-curve homogeneous
// equality certifies it over the complete original edge. The model basis
// may have its own represented parameter interval.
func ProveMappedSplineTransfer(ctx context.Context, support geom.Surface, source SplineUV, model *geom.ExactSplineCurve, o geom.SurfacePatchOptions) (MappedSplineUV, error) {
	var zero MappedSplineUV
	if ctx == nil || model == nil {
		return zero, geom.ErrInvalid
	}
	raw, err := ExactTrimModelBezier(ctx, support, source, false, o)
	if err != nil {
		return zero, err
	}
	normalized, err := model.Reparameterize(0, 1, false, o)
	if err != nil {
		return zero, err
	}
	a, _, err := raw.BezierControlNet(ctx, o)
	if err != nil {
		return zero, err
	}
	b, _, err := normalized.BezierControlNet(ctx, o)
	if err != nil {
		return zero, err
	}
	if len(a.Control) < 2 || len(b.Control) < 2 {
		return zero, geom.ErrDegenerate
	}
	tangent := func(span geom.ExactBezierSpan, axis int) *big.Rat {
		p, q := span.Control[0], span.Control[1]
		num := new(big.Rat).Sub(new(big.Rat).Mul(q[axis], p[3]), new(big.Rat).Mul(p[axis], q[3]))
		num.Mul(num, big.NewRat(int64(len(span.Control)-1), 1))
		return num.Quo(num, new(big.Rat).Mul(p[3], p[3]))
	}
	var weight *big.Rat
	for axis := range 3 {
		x := tangent(a, axis)
		if x.Sign() != 0 {
			weight = new(big.Rat).Quo(tangent(b, axis), x)
			break
		}
	}
	if weight == nil || weight.Sign() <= 0 {
		return zero, geom.ErrConvergence
	}
	q, err := NewMappedSplineUV(ctx, source, weight, o)
	if err != nil {
		return zero, err
	}
	lift, err := ExactTrimModelBezier(ctx, support, q, false, o)
	if err != nil {
		return zero, err
	}
	comparison, err := geom.CompareExactCurvesContext(ctx, lift, normalized, geom.ExactCurveComparisonOptions{
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
	if err != nil {
		return zero, err
	}
	if !comparison.Complete || !comparison.Equal {
		return zero, geom.ErrConvergence
	}
	return q, nil
}
