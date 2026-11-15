package trimmed

import (
	"context"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// polynomialTrimDescriptor accepts bounded native single-Bezier UV data only.
// Normalization/reversal is exact and never reconstructs a sampled trace.
func polynomialTrimDescriptor(ctx context.Context, c PCurve, reverse bool, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	switch q := c.(type) {
	case MultiSpanMappedSplineUV:
		return polynomialTrimDescriptor(ctx, q.curve, reverse, o)
	case PiecewiseMappedSplineUV:
		return polynomialTrimDescriptor(ctx, q.curve, reverse, o)
	case MappedSplineUV:
		return polynomialTrimDescriptor(ctx, q.curve, reverse, o)
	case ExactQuadraticUV:
		h := q.HomogeneousControls()
		if reverse {
			h[0], h[2] = h[2], h[0]
		}
		return exactQuadraticTrimParameterCurve(ctx, h, o)
	case ExactBezierUV:
		if reverse {
			q = q.reversed()
		}
		return q.descriptor(ctx, o)
	case SplineUV:
		if q.Curve == nil || q.Curve.Degree() < 1 || q.Curve.Degree() > 4 {
			return nil, geom.ErrConvergence
		}
		// Restrict the original authored domain before normalizing. A single
		// exact Bezier trace is required, with every redundant knot proved;
		// endpoints alone never establish identity.
		exact, e := polynomialTrimBezierDescriptor(ctx, q, o)
		if e != nil {
			return nil, e
		}
		net, _, e := exact.BezierControlNet(ctx, o)
		if e != nil {
			return nil, e
		}
		if len(net.Control) != q.Curve.Degree()+1 {
			return nil, geom.ErrConvergence
		}
		for _, p := range net.Control {
			if p[2].Sign() != 0 {
				return nil, geom.ErrConvergence
			}
		}
		return exact.Reparameterize(0, 1, reverse, o)
	default:
		return nil, geom.ErrConvergence
	}
}
func comparePolynomialTrimUse(ctx context.Context, model parametric.Edge, surface geom.Surface, use Use, o edgecheck.Options) (bool, error) {
	m, ok := model.Curve.(surfaceTrimCurve)
	if !ok || model.Range != (parametric.Interval{Max: 1}) {
		return false, nil
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	var a *geom.ExactSplineCurve
	var e error
	if m.curve == nil {
		a, _, e = coincidentUseCurve(ctx, Use{UVStart: m.a, UVEnd: m.b}, limits)
	} else {
		a, e = polynomialTrimDescriptor(ctx, m.curve, false, limits)
	}
	if e != nil {
		return false, e
	}
	b, e := polynomialTrimDescriptor(ctx, use.Curve, use.Reversed, limits)
	if e != nil {
		return false, e
	}
	return compareTrimCompositionsOnPlacedSplineSupports(ctx, m.surface, a, surface, b, o)
}

// Compare composed curves in world space by applying each represented map to
// its homogeneous support exactly. Endpoints alone do not prove agreement.
func compareTrimCompositionsOnPlacedSplineSupports(ctx context.Context, a geom.Surface, uvA *geom.ExactSplineCurve,
	b geom.Surface, uvB *geom.ExactSplineCurve, o edgecheck.Options) (bool, error) {
	baseA, mapA := parametric.SurfaceMapping(a)
	baseB, mapB := parametric.SurfaceMapping(b)
	if !supportsExactSplinePatch(baseA) || !supportsExactSplinePatch(baseB) {
		return false, nil
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	patch := func(base geom.Surface, m geom.Affine) (*geom.SurfacePatch, error) {
		p, e := exactSplineSupportPatch(base, limits)
		if e != nil || m == geom.Identity() {
			return p, e
		}
		return p.Transformed(m)
	}
	p, e := patch(baseA, mapA)
	if e != nil {
		return false, e
	}
	q, e := patch(baseB, mapB)
	if e != nil {
		return false, e
	}
	r, e := geom.CompareSurfaceCurveCompositions(ctx, p, uvA, q, uvB, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: limits, MaxEvents: o.MaxIntervals})
	return r.Complete && r.Equal, e
}
func compareSewPolynomialTrims(ctx context.Context, a, b parametric.Edge, reverse bool, o edgecheck.Options) (bool, error) {
	x, ok := a.Curve.(surfaceTrimCurve)
	if !ok {
		return false, nil
	}
	y, ok := b.Curve.(surfaceTrimCurve)
	if !ok {
		return false, nil
	}
	supported := func(c PCurve) bool {
		switch q := c.(type) {
		case ExactQuadraticUV, ExactBezierUV:
			return true
		case SplineUV:
			if q.Curve == nil || q.Curve.Degree() < 1 || q.Curve.Degree() > 4 || q.Range != (UVInterval{Max: 1}) {
				return false
			}
			n := q.Curve.Degree()
			k := q.Curve.Knots()
			if len(q.Curve.Poles()) != n+1 || len(k) != 2*(n+1) {
				return false
			}
			for i, v := range k {
				w := 0.
				if i > n {
					w = 1
				}
				if v != w {
					return false
				}
			}
			return true
		}
		return false
	}
	higher := func(c PCurve) bool {
		switch c.(type) {
		case SplineUV, ExactBezierUV:
			return true
		}
		return false
	}
	if !higher(x.curve) && !higher(y.curve) {
		return false, nil
	}
	if !supported(x.curve) || !supported(y.curve) || a.Range != (parametric.Interval{Max: 1}) || b.Range != (parametric.Interval{Max: 1}) {
		return false, nil
	}
	return comparePolynomialTrimUse(ctx, a, y.surface, Use{UVStart: y.a, UVEnd: y.b, Curve: y.curve, Reversed: reverse}, o)
}
