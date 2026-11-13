package geom

import (
	"fmt"
	"math"
	"math/big"
)

// PlanarLineOptions supplies geometric coplanarity tolerances in addition to
// the root precision, residual and optional line range controls. AngularTolerance
// defaults to 1e-12 and bounds the absolute normal-direction dot product.
type PlanarLineOptions struct {
	LineSurfaceOptions
	AngularTolerance float64
}

// LineEllipseHit carries both curve parameters. ApproximateCoplanarity is true
// when stored local line origin/direction have nonzero normal components; such
// hits are tolerance-qualified and their full 3D residual is checked.
type LineEllipseHit struct {
	LineSurfaceHit
	EllipseParameter       float64
	ApproximateCoplanarity bool
}
type LineEllipseResult struct {
	Hits                []LineEllipseHit
	Precision           uint
	CoplanarityResidual float64
}

// IntersectLineEllipse solves the coplanar specialization, including rational
// exact-sign tangencies and optional line-parameter clipping. A noncoplanar line
// outside the supplied tolerances is rejected, not treated as a general 3D
// line/curve query. Residual is the distance between the line point and an
// evaluated ellipse point, an upper bound on geometric mismatch.
func IntersectLineEllipse(line Line, ellipse Ellipse, options PlanarLineOptions) (LineEllipseResult, error) {
	result := LineEllipseResult{}
	o, e := lineSurfaceOptions(options.LineSurfaceOptions)
	if e != nil {
		return result, e
	}
	angular := options.AngularTolerance
	if angular == 0 {
		angular = 1e-12
	}
	if !finite(angular) || angular < 0 || angular >= 1 || !ellipse.frame.valid() || ellipse.rx <= 0 || ellipse.ry <= 0 {
		return result, ErrInvalid
	}
	origin, direction, e := localLine(line, ellipse.frame)
	if e != nil {
		return result, e
	}
	result.CoplanarityResidual = math.Abs(origin.Z)
	if result.CoplanarityResidual > o.DistanceTolerance || math.Abs(direction.Z) > angular {
		return result, fmt.Errorf("%w: line is not coplanar with ellipse", ErrInvalid)
	}
	q := exactQuadratic{new(big.Rat), new(big.Rat), big.NewRat(-1, 1)}
	for axis, radius := range []float64{ellipse.rx, ellipse.ry} {
		r := new(big.Rat).SetFloat64(radius)
		r.Mul(r, r)
		x, d := new(big.Rat).SetFloat64(coordinate(origin, axis)), new(big.Rat).SetFloat64(coordinate(direction, axis))
		q.a.Add(q.a, new(big.Rat).Quo(new(big.Rat).Mul(d, d), r))
		q.b.Add(q.b, new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(x, d)), r))
		q.c.Add(q.c, new(big.Rat).Quo(new(big.Rat).Mul(x, x), r))
	}
	roots, coincident, tangent, precision, e := solveExactQuadratic(q, o.MaxPrecision)
	result.Precision = precision
	if e != nil {
		return result, e
	}
	if coincident {
		return result, ErrDegenerate
	}
	for _, root := range roots {
		root, inside := inLineRange(root, q, o)
		if !inside {
			continue
		}
		point, e := line.Eval(root.parameter)
		if e != nil {
			return result, e
		}
		local := ellipse.frame.ToLocal(point)
		x, y := local.X/ellipse.rx, local.Y/ellipse.ry
		if !local.IsFinite() || !finite(x) || !finite(y) {
			return result, ErrInvalid
		}
		parameter := math.Atan2(y, x)
		ellipsePoint, e := ellipse.Eval(parameter)
		if e != nil {
			return result, e
		}
		residual := point.Distance(ellipsePoint)
		if !finite(residual) {
			return result, ErrInvalid
		}
		if residual > o.DistanceTolerance {
			return result, ErrConvergence
		}
		result.Hits = append(result.Hits, LineEllipseHit{LineSurfaceHit{root.parameter, root.lower, root.upper, point, residual, tangent, false}, parameter, origin.Z != 0 || direction.Z != 0})
	}
	return result, nil
}

// SplineLineOptions uses SplinePlaneOptions for exact root bounds and work
// budgets. Supporting-plane tolerances default to 1e-9 in distance and 1e-12
// in angular dot product. No planarity is inferred from control points.
type SplineLineOptions struct {
	SplinePlaneOptions
	PlanarityTolerance, AngularTolerance float64
}

// SplineLineHit retains the spline's isolated root diagnostics and adds the
// corresponding line point/parameter and their 3D separation.
type SplineLineHit struct {
	PlaneCurveHit
	LineParameter float64
	LinePoint     Vec3
	LineResidual  float64
}

// SplineLineOverlap is a coincident curve span. MaxControlResidual bounds the
// control points' distance from the line; positivity of rational weights makes
// it a hull bound for this span, subject to float64 refinement error.
type SplineLineOverlap struct {
	Span               ParameterSpan
	MaxControlResidual float64
}
type SplineLineResult struct {
	Hits                     []SplineLineHit
	Coincident               []SplineLineOverlap
	MaxPlanarityResidual     float64
	Operations, Subdivisions int
}

// IntersectSplineLine specializes curve/curve intersection to a rational spline
// and an infinite line in a supplied supporting plane. The auxiliary plane
// through the line reduces the problem to exact homogeneous polynomial roots.
// Coplanarity and every hit/overlap are checked in 3D. Boundary hits preserve
// the one-sided span semantics of IntersectSplinePlane. On error the result is
// partial; this API does not claim intersections of general spatial curves.
func IntersectSplineLine(curve *BSpline, line Line, supportingPlane Plane, options SplineLineOptions) (SplineLineResult, error) {
	result := SplineLineResult{}
	if curve == nil || len(curve.poles) == 0 || !line.valid() || !supportingPlane.valid() {
		return result, ErrInvalid
	}
	planarity, angular := options.PlanarityTolerance, options.AngularTolerance
	if planarity == 0 {
		planarity = 1e-9
	}
	if angular == 0 {
		angular = 1e-12
	}
	if !finite(planarity) || planarity < 0 || !finite(angular) || angular < 0 || angular >= 1 {
		return result, ErrInvalid
	}
	for _, p := range curve.poles {
		distance := math.Abs(supportingPlane.SignedDistance(p))
		if !finite(distance) {
			return result, ErrInvalid
		}
		result.MaxPlanarityResidual = math.Max(result.MaxPlanarityResidual, distance)
	}
	linePlaneDistance := math.Abs(supportingPlane.SignedDistance(line.origin))
	if !finite(linePlaneDistance) {
		return result, ErrInvalid
	}
	result.MaxPlanarityResidual = math.Max(result.MaxPlanarityResidual, linePlaneDistance)
	if result.MaxPlanarityResidual > planarity || math.Abs(supportingPlane.normal.Dot(line.direction)) > angular {
		return result, fmt.Errorf("%w: line and spline must lie in supporting plane", ErrInvalid)
	}
	sectionPlane, e := NewPlane(line.origin, supportingPlane.normal.Cross(line.direction))
	if e != nil {
		return result, e
	}
	section, sectionError := IntersectSplinePlane(curve, sectionPlane, options.SplinePlaneOptions)
	result.Operations, result.Subdivisions = section.Operations, section.Subdivisions
	distanceTolerance := options.DistanceTolerance
	if distanceTolerance == 0 {
		distanceTolerance = 1e-8
	}
	for _, hit := range section.Hits {
		point, parameter, e := line.Closest(hit.Point)
		if e != nil {
			return result, e
		}
		residual := point.Distance(hit.Point)
		if !finite(residual) {
			return result, ErrInvalid
		}
		if residual > distanceTolerance {
			return result, ErrConvergence
		}
		result.Hits = append(result.Hits, SplineLineHit{hit, parameter, point, residual})
	}
	a, b := curve.Domain()
	for _, span := range section.Coincident {
		piece := curve
		if span.Start > a {
			_, piece, e = piece.Split(span.Start)
			if e != nil {
				return result, e
			}
		}
		if span.End < b {
			piece, _, e = piece.Split(span.End)
			if e != nil {
				return result, e
			}
		}
		maximum := 0.0
		for _, pole := range piece.poles {
			point, _, e := line.Closest(pole)
			if e != nil {
				return result, e
			}
			residual := point.Distance(pole)
			if !finite(residual) {
				return result, ErrInvalid
			}
			maximum = math.Max(maximum, residual)
		}
		if maximum > distanceTolerance {
			return result, ErrConvergence
		}
		result.Coincident = append(result.Coincident, SplineLineOverlap{span, maximum})
	}
	return result, sectionError
}
