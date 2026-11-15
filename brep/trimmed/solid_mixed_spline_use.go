package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// Preserve the exact original restriction and orientation. The representation
// conversion never changes normalized coedge t or exports rational controls to
// floating poles; complete original model-edge comparison still follows.
func mixedStoredHigherSplineUV(ctx context.Context, q SplineUV, o geom.SurfacePatchOptions) (PCurve, error) {
	if q.Curve == nil || q.Curve.Degree() < 3 || q.Curve.Degree() > 4 {
		return nil, geom.ErrConvergence
	}
	c, e := polynomialTrimBezierDescriptor(ctx, q, o)
	if e != nil {
		return nil, e
	}
	net, _, e := c.BezierControlNet(ctx, o)
	if e != nil {
		return nil, e
	}
	if len(net.Control) < 4 || len(net.Control) > 5 {
		return nil, geom.ErrConvergence
	}
	h := make([][3]*big.Rat, len(net.Control))
	for i, p := range net.Control {
		if p[2].Sign() != 0 {
			return nil, geom.ErrDomain
		}
		j := i
		h[j] = [3]*big.Rat{new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1]), new(big.Rat).Set(p[3])}
	}
	qout, e := NewExactBezierUV(h)
	if e != nil {
		return nil, e
	}
	return qout, nil
}

// Exact whole-curve identity permits the existing original straight model-edge
// proof; equal endpoints alone never permit this reduction.
func mixedStoredSplineAffineIdentity(ctx context.Context, c PCurve, u Use, o geom.SurfacePatchOptions) (bool, error) {
	exact, _, e := coincidentUseCurve(ctx, Use{Curve: c}, o)
	if e != nil {
		return false, e
	}
	line, _, e := coincidentUseCurve(ctx, Use{UVStart: u.UVStart, UVEnd: u.UVEnd}, o)
	if e != nil {
		return false, e
	}
	r, e := geom.CompareExactCurvesContext(ctx, exact, line, geom.ExactCurveComparisonOptions{NormalizeDomains: true, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	return e == nil && r.Complete && r.Equal, e
}
