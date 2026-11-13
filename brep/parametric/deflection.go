package parametric

import (
	"context"
	"errors"
	"fmt"
	"math"

	"plainsolid/brep/splinebounds"
	"plainsolid/geom"
)

// ErrDeflectionUnsupported identifies a patch without analytic derivative bounds.
var ErrDeflectionUnsupported = errors.New("parametric: analytic deflection bound unavailable")

// DeflectionBound returns an analytic upper bound for interpolation error on
// a uniform subdivisions-by-subdivisions grid, including the stored tolerance
// for shared-edge vertex displacement. This uses second-derivative norms
// and parameter variance. It applies to built-in analytic patches and
// supported clamped rational spline patches; floating
// evaluation roundoff is outside the real-arithmetic bound.
func (s Solid) DeflectionBound(subdivisions int) (float64, error) {
	return s.deflectionBound(context.Background(), subdivisions)
}

func (s Solid) deflectionBound(ctx context.Context, subdivisions int) (float64, error) {
	if subdivisions < 1 {
		return 0, fmt.Errorf("parametric: invalid subdivision count")
	}
	q, e := s.deflectionCurvature(ctx)
	if e != nil || len(s.faces) == 0 {
		return 0, e
	}
	n := float64(subdivisions)
	bound := upAdd(upDiv(upDiv(q, n), n), s.tolerance)
	if !finite(bound) {
		return 0, geom.ErrInvalid
	}
	return bound, nil
}

func (s Solid) deflectionCurvature(ctx context.Context) (float64, error) {
	bound := 0.
	for _, f := range s.faces {
		q, e := SurfaceDeflectionBoundContext(ctx, f.Surface, f.U, f.V)
		if e != nil {
			return 0, e
		}
		bound = max(bound, q)
	}
	return bound, nil
}

// SurfaceDeflectionBound bounds affine interpolation of a supported analytic
// surface on a parameter triangle contained in the given rectangle. The bound
// applies to any parameter triangle in the rectangle and excludes evaluation
// roundoff and any displacement of the supplied model vertices.
func SurfaceDeflectionBound(s geom.Surface, u, v Interval) (float64, error) {
	return SurfaceDeflectionBoundContext(context.Background(), s, u, v)
}

// SurfaceDeflectionBoundContext is SurfaceDeflectionBound with cancellation.
func SurfaceDeflectionBoundContext(ctx context.Context, s geom.Surface, u, v Interval) (float64, error) {
	if ctx == nil {
		return 0, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return 0, e
	}
	dx, dy := u.Max-u.Min, v.Max-v.Min
	if dx < 0 || dy < 0 || !finite(dx) || !finite(dy) {
		return 0, geom.ErrInvalid
	}
	a, b, c, e := surfaceDerivatives(ctx, s, u, v)
	if e != nil {
		return 0, e
	}
	dx, dy = up(dx), up(dy)
	uu, uv, vv := upMul(upMul(a, dx), dx), upMul(upMul(upMul(b, dx), dy), 2), upMul(upMul(c, dy), dy)
	q := upDiv(upAdd(upAdd(uu, uv), vv), 8)
	if !finite(q) {
		return 0, geom.ErrInvalid
	}
	return q, nil
}

func up(x float64) float64 {
	if x == 0 {
		return 0
	}
	return math.Nextafter(x, math.Inf(1))
}
func upAdd(a, b float64) float64 {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	return up(a + b)
}
func upMul(a, b float64) float64 {
	if a == 0 || b == 0 {
		return 0
	}
	return math.Nextafter(a*b, math.Inf(1))
}
func upDiv(a, b float64) float64 {
	if a == 0 {
		return 0
	}
	return math.Nextafter(a/b, math.Inf(1))
}

func surfaceDerivatives(ctx context.Context, s geom.Surface, u, v Interval) (float64, float64, float64, error) {
	switch x := s.(type) {
	case geom.TwistedCylinderSurface:
		return x.SecondDerivativeBounds()
	case geom.TwistedAnnularSurface:
		return x.SecondDerivativeBounds()
	case geom.HelicalProfileSurface:
		return x.SecondDerivativeBounds(geom.ParameterRectangle{UMin: u.Min, UMax: u.Max, VMin: v.Min, VMax: v.Max})
	case offsetBoreChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case steinmetzExteriorCylinder:
		return x.secondDerivativeBounds(ctx, u, v)
	case steinmetzDualCapChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case steinmetzSlabChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case unequalUnionChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case steinmetzUnionChart:
		if u.Min < -math.Pi/2 || u.Max > math.Pi/2 || v.Min < 0 || v.Max > 1 {
			return 0, 0, 0, geom.ErrDomain
		}
		return upMul(3, x.radius), x.radius, 0, nil
	case steinmetzClippedCylinder:
		return x.secondDerivativeBounds(ctx, u, v)
	case steinmetzDisk:
		return x.secondDerivativeBounds(ctx, u, v)
	case steinmetzChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case unequalCylinderCommonChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case skewCylinderChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case skewOverlapCylinderChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case obliqueCylinderChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case obliqueCylinderUnionChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case splitCylinderCutChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case obliqueSplitChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case generalCylinderCommonChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case toolBoreChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case offsetCommonChart:
		return x.secondDerivativeBounds(ctx, u, v)
	case *geom.BSplineSurface:
		r, e := splinebounds.Bound(ctx, x, splinebounds.Domain{U0: u.Min, U1: u.Max, V0: v.Min, V1: v.Max}, splinebounds.Options{})
		if e != nil {
			if errors.Is(e, splinebounds.ErrUnsupported) || errors.Is(e, splinebounds.ErrInvalid) {
				return 0, 0, 0, fmt.Errorf("%w: %v", ErrDeflectionUnsupported, e)
			}
			return 0, 0, 0, e
		}
		return r.DUU, r.DUV, r.DVV, nil
	case *geom.WrappedSurfacePatch:
		if x == nil || x.FundamentalPatch() == nil {
			return 0, 0, 0, ErrDeflectionUnsupported
		}
		d := x.Domain()
		if u.Min < d.UMin || u.Max > d.UMax || v.Min < d.VMin || v.Max > d.VMax {
			return 0, 0, 0, ErrDeflectionUnsupported
		}
		recipe, e := x.ExportRecipe()
		if e != nil {
			return 0, 0, 0, fmt.Errorf("%w: %v", ErrDeflectionUnsupported, e)
		}
		return surfaceDerivatives(ctx, recipe.Surface, u, v)
	case mappedSurface:
		a, b, c, e := surfaceDerivatives(ctx, x.surface, u, v)
		if e != nil {
			return 0, 0, 0, e
		}
		n := 0.
		for _, r := range x.transform {
			for j := range 3 {
				n = up(math.Hypot(n, r[j]))
			}
		}
		return upMul(a, n), upMul(b, n), upMul(c, n), nil
	default:
		a, b, c, ok := derivativeBounds(s, v)
		if !ok {
			return 0, 0, 0, fmt.Errorf("%w: %T", ErrDeflectionUnsupported, s)
		}
		return up(a), up(b), up(c), nil
	}
}

func derivativeBounds(s geom.Surface, v Interval) (uu, uv, vv float64, ok bool) {
	switch x := s.(type) {
	case geom.PlaneSurface:
		return 0, 0, 0, true
	case geom.CylinderSurface:
		_, r := x.Parameters()
		return r, 0, 0, true
	case geom.ConeSurface:
		_, r := x.Parameters()
		return math.Max(math.Abs(v.Min), math.Abs(v.Max)) * r, r, 0, true
	case geom.SphereSurface:
		_, r := x.Parameters()
		return r, r, r, true
	case geom.TorusSurface:
		_, a, b := x.Parameters()
		return a + b, b, b, true
	case reparameterizedCylinder:
		_, r := x.s.Parameters()
		k := x.u.Max - x.u.Min
		return r * k * k, 0, 0, true
	case ruledArcs:
		_, ra := x.a.Parameters()
		_, rb := x.b.Parameters()
		a, b := x.u.Max-x.u.Min, x.v.Max-x.v.Min
		return math.Max(ra*a*a, rb*b*b), ra*a + rb*b, 0, true
	case cutCylinder:
		_, r := x.rim.cyl.Parameters()
		k := math.Hypot(x.rim.cosine, x.rim.sine)
		return math.Hypot(r, k), k, 0, true
	case slantedFan:
		_, r := x.rim.cyl.Parameters()
		k := math.Hypot(x.rim.cosine, x.rim.sine)
		return math.Hypot(r, k), math.Hypot(r, k), 0, true
	case arcFan:
		_, r := x.surface.Parameters()
		return r, r, 0, true
	case disk:
		return math.Max(math.Abs(v.Min), math.Abs(v.Max)), 1, 0, true
	case revolvedSide:
		return math.Max(x.a.X, x.b.X), math.Abs(x.a.X - x.b.X), 0, true
	case ellipticPatch:
		return math.Max(math.Max(x.lower.X, x.upper.X), math.Max(x.lower.Y, x.upper.Y)),
			math.Max(math.Abs(x.lower.X-x.upper.X), math.Abs(x.lower.Y-x.upper.Y)), 0, true
	case ellipticBand:
		var a, b, c float64
		for _, r := range [2][4]float64{x.x, x.y} {
			a = max(a, r[0], r[1], r[2], r[3])
			for i := range 3 {
				b = max(b, 3*math.Abs(r[i+1]-r[i]))
			}
			for i := range 2 {
				c = max(c, 6*math.Abs((r[i+2]-r[i+1])-(r[i+1]-r[i])))
			}
		}
		return a, b, c, true
	case mappedSurface:
		a, b, c, ok := derivativeBounds(x.surface, v)
		n := 0.
		for _, r := range x.transform {
			n = math.Hypot(n, math.Hypot(math.Hypot(r[0], r[1]), r[2]))
		}
		return a * n, b * n, c * n, ok
	default:
		return 0, 0, 0, false
	}
}
