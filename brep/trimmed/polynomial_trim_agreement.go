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
	case ExactQuadraticUV:
		h := q.HomogeneousControls()
		if reverse {
			h[0], h[2] = h[2], h[0]
		}
		return exactQuadraticTrimParameterCurve(ctx, h, o)
	case SplineUV:
		if q.Curve == nil || q.Curve.Degree() < 1 || q.Curve.Degree() > 4 || q.Range != (UVInterval{Max: 1}) {
			return nil, geom.ErrConvergence
		}
		n := q.Curve.Degree()
		poles := q.Curve.Poles()
		knots := q.Curve.Knots()
		if len(poles) != n+1 || len(knots) != 2*(n+1) {
			return nil, geom.ErrConvergence
		}
		for i, k := range knots {
			want := 0.
			if i > n {
				want = 1
			}
			if k != want {
				return nil, geom.ErrConvergence
			}
		}
		for _, p := range poles {
			if p.Z != 0 {
				return nil, geom.ErrConvergence
			}
		}
		exact, e := geom.NewExactSplineCurve(q.Curve, o)
		if e != nil {
			return nil, e
		}
		if q.Reversed != reverse {
			return exact.Reparameterize(0, 1, true, o)
		}
		return exact, nil
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
	a, e := polynomialTrimDescriptor(ctx, m.curve, false, limits)
	if e != nil {
		return false, e
	}
	b, e := polynomialTrimDescriptor(ctx, use.Curve, use.Reversed, limits)
	if e != nil {
		return false, e
	}
	source, sourceMap := parametric.SurfaceMapping(m.surface)
	target, targetMap := parametric.SurfaceMapping(surface)
	sa, ok := source.(*geom.BSplineSurface)
	if !ok {
		return false, nil
	}
	sb, ok := target.(*geom.BSplineSurface)
	if !ok || sourceMap != targetMap {
		return false, nil
	}
	ap, e := geom.NewSurfacePatch(sa, limits)
	if e != nil {
		return false, e
	}
	bp, e := geom.NewSurfacePatch(sb, limits)
	if e != nil {
		return false, e
	}
	r, e := geom.CompareSurfaceCurveCompositions(ctx, ap, a, bp, b, geom.SurfaceCurveAgreementOptions{SurfacePatchOptions: limits, MaxEvents: o.MaxIntervals})
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
		case ExactQuadraticUV:
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
	_, xok := x.curve.(SplineUV)
	_, yok := y.curve.(SplineUV)
	if !xok && !yok {
		return false, nil
	}
	if !supported(x.curve) || !supported(y.curve) || a.Range != (parametric.Interval{Max: 1}) || b.Range != (parametric.Interval{Max: 1}) {
		return false, nil
	}
	return comparePolynomialTrimUse(ctx, a, y.surface, Use{UVStart: y.a, UVEnd: y.b, Curve: y.curve, Reversed: reverse}, o)
}
