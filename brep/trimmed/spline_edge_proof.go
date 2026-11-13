package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// exactEdgeSpline constructs a homogeneous model-edge descriptor from a
// clamped model curve, a spline isocurve or an affine image of either.
func exactEdgeSpline(e parametric.Edge) (*geom.ExactSplineCurve, bool, error) {
	return exactEdgeSplineWithOptions(e, geom.SurfacePatchOptions{})
}

func exactEdgeSplineWithOptions(e parametric.Edge, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, bool, error) {
	if c, rev, err := exactWrappedModelIso(e, o); c != nil || err != nil {
		return c, rev, err
	}
	base, placement := parametric.CurveMapping(e.Curve)
	var c *geom.ExactSplineCurve
	var err error
	switch x := base.(type) {
	case *geom.BSpline:
		c, err = geom.NewExactSplineCurve(x, o)
	case surfaceTrimCurve:
		return exactTrimIsoWithOptions(x.surface, x.a, x.b, x.curve, e.Range, placement, o)
	default:
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	c, err = c.Restrict(e.Range.Min, e.Range.Max, o)
	if err != nil {
		return nil, false, err
	}
	if placement != geom.Identity() {
		c, err = c.Transform(placement, o)
	}
	return c, false, err
}

// exactTrimIso returns the curve in increasing source spline parameter and
// whether the declared normalized trim traverses it in reverse.
func exactTrimIso(surface geom.Surface, a, b geom.Vec2, trim PCurve, span parametric.Interval, outer geom.Affine) (*geom.ExactSplineCurve, bool, error) {
	return exactTrimIsoWithOptions(surface, a, b, trim, span, outer, geom.SurfacePatchOptions{})
}

func exactTrimIsoWithOptions(surface geom.Surface, a, b geom.Vec2, trim PCurve, span parametric.Interval, outer geom.Affine, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, bool, error) {
	if trim != nil || span.Min < 0 || span.Max > 1 || span.Min >= span.Max {
		return nil, false, nil
	}
	base, placement := parametric.SurfaceMapping(surface)
	s, ok := base.(*geom.BSplineSurface)
	if !ok {
		return nil, false, nil
	}
	p, e := geom.NewSurfacePatch(s, o)
	if e != nil {
		return nil, false, e
	}
	axis, fixed := 0, a.X
	start, end := a.Y, b.Y
	if a.X != b.X {
		if a.Y != b.Y {
			mix := func(a, b geom.Vec2, t float64) [2]*big.Rat {
				var r [2]*big.Rat
				x, y := [2]float64{a.X, a.Y}, [2]float64{b.X, b.Y}
				for i := range r {
					f := new(big.Rat).Sub(ratInset(y[i]), ratInset(x[i]))
					r[i] = f.Add(f.Mul(f, ratInset(t)), ratInset(x[i]))
				}
				return r
			}
			c, e := p.AffineParameterCurve(context.Background(), mix(a, b, span.Min), mix(a, b, span.Max), geom.TubeAffineLineOptions{
				MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
			})
			if e != nil {
				return nil, false, e
			}
			for _, m := range []geom.Affine{placement, outer} {
				if m != geom.Identity() {
					c, e = c.Transform(m, o)
					if e != nil {
						return nil, false, e
					}
				}
			}
			return c, false, nil
		}
		axis, fixed, start, end = 1, a.Y, a.X, b.X
	}
	if start == end {
		return nil, false, nil
	}
	v := func(t float64) (float64, bool) { return exactFloatMix(start, end, t) }
	l, ok := v(span.Min)
	h, has := v(span.Max)
	if !ok || !has || l == h {
		return nil, false, nil
	}
	c, e := p.IsoCurve(axis, fixed, o)
	if e != nil {
		return nil, false, e
	}
	c, e = c.Restrict(min(l, h), max(l, h), o)
	if e != nil {
		return nil, false, e
	}
	for _, m := range []geom.Affine{placement, outer} {
		if m != geom.Identity() {
			c, e = c.Transform(m, o)
			if e != nil {
				return nil, false, e
			}
		}
	}
	return c, l > h, e
}

func exactFloatMix(a, b, t float64) (float64, bool) {
	x := new(big.Rat).SetFloat64(a)
	y := new(big.Rat).SetFloat64(b)
	z := new(big.Rat).SetFloat64(t)
	if x == nil || y == nil || z == nil {
		return 0, false
	}
	f, e := geom.RepresentedExactScalar(new(big.Rat).Add(x, new(big.Rat).Mul(new(big.Rat).Sub(y, x), z)))
	return f, e == nil
}

func compareSplineEdgeUse(ctx context.Context, model parametric.Edge, surface geom.Surface, use Use, o edgecheck.Options) (bool, error) {
	if use.Curve != nil {
		switch use.Curve.(type) {
		case SplineUV, ExactQuadraticUV:
			return compareSplineRationalTrimUse(ctx, model, surface, use, o)
		}
		return compareSplineWallUse(ctx, model, surface, use, o)
	}
	base, _ := parametric.SurfaceMapping(surface)
	if _, ok := base.(*geom.WrappedSurfacePatch); ok {
		limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
		x, rx, e := exactEdgeSplineWithOptions(model, limits)
		if e != nil || x == nil {
			return false, e
		}
		line, ok := analyticUseLine(surface, use)
		if !ok {
			return false, nil
		}
		y, ry, e := exactWrappedIso(line, limits)
		if e != nil || y == nil {
			return false, e
		}
		return compareSplineDescriptors(ctx, x, y, rx != ry, o)
	}
	if _, ok := base.(*geom.BSplineSurface); !ok {
		return false, nil
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	if c, outer := parametric.CurveMapping(model.Curve); outer == geom.Identity() {
		if m, ok := c.(surfaceTrimCurve); ok && m.curve == nil &&
			(m.a.X != m.b.X && m.a.Y != m.b.Y || use.UVStart.X != use.UVEnd.X && use.UVStart.Y != use.UVEnd.Y) {
			return compareSplineAffineTrimPieces(ctx, m, model.Range, surface, use, o)
		}
	}
	a, reverseModel, e := exactEdgeSplineWithOptions(model, limits)
	if e != nil || a == nil {
		return false, e
	}
	b, reverseUse, e := exactTrimIsoWithOptions(surface, use.UVStart, use.UVEnd, use.Curve, parametric.Interval{Max: 1}, geom.Identity(), limits)
	if e != nil || b == nil {
		return false, e
	}
	return compareSplineDescriptors(ctx, a, b, reverseModel != (reverseUse != use.Reversed), o)
}

func compareSplineAffineTrimPieces(ctx context.Context, m surfaceTrimCurve, span parametric.Interval,
	surface geom.Surface, use Use, o edgecheck.Options) (bool, error) {
	type trim struct {
		surface geom.Surface
		a, b    geom.Vec2
		span    parametric.Interval
	}
	parts := [2]geom.SurfaceAffineCurvePieces{}
	for i, x := range [2]trim{{m.surface, m.a, m.b, span},
		{surface, use.UVStart, use.UVEnd, parametric.Interval{Max: 1}}} {
		s, place := parametric.SurfaceMapping(x.surface)
		b, ok := s.(*geom.BSplineSurface)
		if !ok {
			return false, nil
		}
		limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
		p, e := geom.NewSurfacePatch(b, limits)
		if e == nil && place != geom.Identity() {
			p, e = p.Transformed(place)
		}
		if e != nil {
			return false, e
		}
		at := func(t float64) [2]*big.Rat {
			r := [2]*big.Rat{}
			for i, a := range [2]float64{x.a.X, x.a.Y} {
				b := [2]float64{x.b.X, x.b.Y}[i]
				v := new(big.Rat).Sub(ratInset(b), ratInset(a))
				r[i] = v.Add(v.Mul(v, ratInset(t)), ratInset(a))
			}
			return r
		}
		q, e := p.AffineParameterCurvePieces(ctx, at(x.span.Min), at(x.span.Max), geom.SurfaceAffineCurveOptions{
			TubeAffineLineOptions: geom.TubeAffineLineOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits},
			MaxPieces:             o.MaxIntervals,
		})
		if e != nil {
			if errors.Is(e, geom.ErrConvergence) || errors.Is(e, geom.ErrDomain) {
				return false, nil
			}
			return false, e
		}
		parts[i] = q
	}
	r, e := geom.CompareAffineParameterCurvePieces(ctx, parts[0], parts[1], geom.ExactCurveComparisonOptions{
		ReverseB: use.Reversed, NormalizeDomains: true, MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits, MaxSpans: o.MaxIntervals,
	})
	return r.Complete && r.Equal, e
}

func compareSplineWallUse(ctx context.Context, model parametric.Edge, surface geom.Surface, use Use, o edgecheck.Options) (bool, error) {
	var rev bool
	c := use.Curve
	for {
		q, ok := c.(reverseTrim)
		if !ok {
			break
		}
		c, rev = q.differentiableTrim, !rev
	}
	w, ok := c.(wallTrim)
	if !ok || w.surface == nil || !finite(w.length) || w.length <= 0 || !finite(w.height) {
		return false, nil
	}
	plane, ok := surface.(geom.PlaneSurface)
	if !ok {
		return false, nil
	}
	a, b := use.UVStart, use.UVEnd
	if rev {
		a, b = b, a
	}
	if a != (geom.Vec2{Y: w.start}) || b != (geom.Vec2{X: w.length, Y: w.end}) {
		return false, nil
	}
	f := plane.Frame()
	origin, u, v := f.Origin(), f.XAxis(), f.YAxis()
	// The wall pcurve is (length*t, Z(sourceUV(t))-height).
	// Prove its plane map equals the original identity XY graph
	// point over the whole range, including declared endpoints.
	if origin != (geom.Vec3{X: w.a.X, Y: w.a.Y, Z: w.height}) || v != (geom.Vec3{Z: 1}) || u.Z != 0 {
		return false, nil
	}
	for i, x := range [2]float64{u.X, u.Y} {
		p, q := [2]float64{w.a.X, w.a.Y}[i], [2]float64{w.b.X, w.b.Y}[i]
		if new(big.Rat).Mul(ratInset(x), ratInset(w.length)).Cmp(new(big.Rat).Sub(ratInset(q), ratInset(p))) != 0 {
			return false, nil
		}
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	if e := requireIdentitySplineChart(ctx, w.surface, limits); e != nil {
		return false, e
	}
	p, e := geom.NewSurfacePatch(w.surface, limits)
	if e != nil {
		return false, e
	}
	for j, x := range [2]geom.Vec2{w.a, w.b} {
		y, e := p.EvaluateRational(ctx, ratInset(x.X), ratInset(x.Y), geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits,
		})
		if e != nil {
			return false, e
		}
		z := [2]float64{w.start, w.end}[j]
		if new(big.Rat).Add(ratInset(z), ratInset(w.height)).Cmp(y.Point[2]) != 0 {
			return false, nil
		}
	}
	x, xr, e := exactEdgeSplineWithOptions(model, limits)
	if e != nil || x == nil {
		return false, e
	}
	y, yr, e := exactTrimIsoWithOptions(w.surface, w.a, w.b, nil, parametric.Interval{Max: 1}, geom.Identity(), limits)
	if e != nil || y == nil {
		return false, e
	}
	return compareSplineDescriptors(ctx, x, y, xr != (yr != (rev != use.Reversed)), o)
}

func compareSplineDescriptors(ctx context.Context, a, b *geom.ExactSplineCurve, reverse bool, o edgecheck.Options) (bool, error) {
	r, e := geom.CompareExactCurvesContext(ctx, a, b, geom.ExactCurveComparisonOptions{
		NormalizeDomains: true, ReverseB: reverse, MaxSpans: o.MaxIntervals, MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits,
	})
	return r.Complete && r.Equal, e
}

// A composed rational trim can be compared continuously across different
// source support knot partitions without reconstructing a 3D spline.
func compareSplineRationalTrimUse(ctx context.Context, model parametric.Edge, surface geom.Surface, use Use, o edgecheck.Options) (bool, error) {
	m, ok := model.Curve.(surfaceTrimCurve)
	if !ok || model.Range != (parametric.Interval{Max: 1}) {
		return false, nil
	}
	controls := func(c PCurve) ([3][3]*big.Rat, bool, error) {
		switch p := c.(type) {
		case SplineUV:
			h, e := quadraticTrimSourceControls(p)
			return h, true, e
		case ExactQuadraticUV:
			return p.HomogeneousControls(), p.control[0][0] != nil, nil
		}
		return [3][3]*big.Rat{}, false, nil
	}
	source, ok, e := controls(m.curve)
	if e != nil || !ok {
		return false, e
	}
	target, ok, e := controls(use.Curve)
	if e != nil || !ok {
		return false, e
	}
	if use.Reversed {
		target[0], target[2] = target[2], target[0]
	}
	sourceBase, sourceMap := parametric.SurfaceMapping(m.surface)
	targetBase, targetMap := parametric.SurfaceMapping(surface)
	a, ok := sourceBase.(*geom.BSplineSurface)
	if !ok {
		return false, nil
	}
	b, ok := targetBase.(*geom.BSplineSurface)
	if !ok || sourceMap != targetMap {
		return false, nil
	}
	limits := geom.SurfacePatchOptions{MaxOperations: o.MaxWork, MaxCoefficientBits: o.MaxBits}
	ap, e := geom.NewSurfacePatch(a, limits)
	if e != nil {
		return false, e
	}
	bp, e := geom.NewSurfacePatch(b, limits)
	if e != nil {
		return false, e
	}

	uvA, e := exactQuadraticTrimParameterCurve(ctx, source, limits)
	if e != nil {
		return false, e
	}
	uvB, e := exactQuadraticTrimParameterCurve(ctx, target, limits)
	if e != nil {
		return false, e
	}
	r, e := geom.CompareSurfaceCurveCompositions(ctx, ap, uvA, bp, uvB, geom.SurfaceCurveAgreementOptions{
		SurfacePatchOptions: limits, MaxEvents: o.MaxIntervals,
	})
	return r.Complete && r.Equal, e
}

func composeSplineTrimControls(ctx context.Context, p *geom.SurfacePatch, h [3][3]*big.Rat,
	o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	uv, e := exactQuadraticTrimParameterCurve(ctx, h, o)
	if e != nil {
		return nil, e
	}
	return p.ComposeParameterCurve(ctx, uv, o)
}

func exactQuadraticTrimParameterCurve(ctx context.Context, h [3][3]*big.Rat, o geom.SurfacePatchOptions) (*geom.ExactSplineCurve, error) {
	net := make([][4]*big.Rat, 3)
	for i, q := range h {
		net[i] = [4]*big.Rat{q[0], q[1], new(big.Rat), q[2]}
	}
	return geom.NewExactHomogeneousBezier(ctx, net, o)
}
