package geom

import (
	"math"
	"math/big"
)

// LineSurfaceOptions applies an optional inclusive parameter range; line
// parameters are signed distances. Zero options mean the entire infinite line,
// distance tolerance 1e-9, and maximum root precision 256 bits. MaxPrecision
// must be 64..4096. Exact discriminant signs are not replaced by a tolerance.
type LineSurfaceOptions struct {
	UseParameterRange          bool
	MinParameter, MaxParameter float64
	DistanceTolerance          float64
	MaxPrecision               uint
}

// LineSurfaceHit has a root bracket exact for the local float64 line
// coefficients. World reconstruction is checked by the reported surface
// distance residual. Tangent denotes an exact repeated quadratic root;
// Singular denotes the cone apex, where a regular surface normal is undefined.
type LineSurfaceHit struct {
	Parameter, ParameterMin, ParameterMax float64
	Point                                 Vec3
	Residual                              float64
	Tangent, Singular                     bool
}

// LineParameterRange describes coincident generator geometry. Endpoints may be
// infinite; finite apex bounds are rounded inward to representable float64
// parameters, retaining every representable parameter in the valid nappe.
type LineParameterRange struct{ Min, Max float64 }

// LineSurfaceResult separates isolated contacts from generator coincidence.
// On error a partial result must not be considered exhaustive. Precision is the
// highest big.Float root precision used (zero for rational/degenerate roots).
type LineSurfaceResult struct {
	Hits       []LineSurfaceHit
	Coincident []LineParameterRange
	Precision  uint
}

func lineSurfaceOptions(o LineSurfaceOptions) (LineSurfaceOptions, error) {
	if o.DistanceTolerance == 0 {
		o.DistanceTolerance = 1e-9
	}
	if o.MaxPrecision == 0 {
		o.MaxPrecision = 256
	}
	if !finite(o.DistanceTolerance) || o.DistanceTolerance <= 0 || o.MaxPrecision < 64 || o.MaxPrecision > 4096 {
		return o, ErrInvalid
	}
	if o.UseParameterRange {
		if math.IsNaN(o.MinParameter) || math.IsNaN(o.MaxParameter) || o.MinParameter > o.MaxParameter || math.IsInf(o.MinParameter, 1) || math.IsInf(o.MaxParameter, -1) {
			return o, ErrInvalid
		}
	} else {
		o.MinParameter, o.MaxParameter = math.Inf(-1), math.Inf(1)
	}
	return o, nil
}
func localLine(line Line, frame Frame) (Vec3, Vec3, error) {
	if !line.valid() || !frame.valid() {
		return Vec3{}, Vec3{}, ErrInvalid
	}
	origin := frame.ToLocal(line.origin)
	direction := Vec3{line.direction.Dot(frame.x), line.direction.Dot(frame.y), line.direction.Dot(frame.z)}
	if !origin.IsFinite() || !direction.IsFinite() {
		return Vec3{}, Vec3{}, ErrInvalid
	}
	return origin, direction, nil
}
func localQuadric(origin, direction Vec3, radius, slope float64, cone bool) exactQuadratic {
	q := exactQuadratic{new(big.Rat), new(big.Rat), new(big.Rat)}
	for axis := 0; axis < 3; axis++ {
		if axis == 2 && !cone {
			break
		}
		factor := big.NewRat(1, 1)
		if axis == 2 {
			v := new(big.Rat).SetFloat64(slope)
			factor.Neg(new(big.Rat).Mul(v, v))
		}
		o, d := new(big.Rat).SetFloat64(coordinate(origin, axis)), new(big.Rat).SetFloat64(coordinate(direction, axis))
		q.a.Add(q.a, new(big.Rat).Mul(factor, new(big.Rat).Mul(d, d)))
		q.b.Add(q.b, new(big.Rat).Mul(factor, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(o, d))))
		q.c.Add(q.c, new(big.Rat).Mul(factor, new(big.Rat).Mul(o, o)))
	}
	if !cone {
		r := new(big.Rat).SetFloat64(radius)
		q.c.Sub(q.c, new(big.Rat).Mul(r, r))
	}
	return q
}
func inLineRange(root quadraticRoot, q exactQuadratic, o LineSurfaceOptions) (quadraticRoot, bool) {
	for _, boundary := range []struct {
		value   float64
		minimum bool
	}{{o.MinParameter, true}, {o.MaxParameter, false}} {
		if math.IsInf(boundary.value, 0) {
			continue
		}
		comparison := root.compare(new(big.Rat).SetFloat64(boundary.value), q)
		if boundary.minimum && comparison < 0 || !boundary.minimum && comparison > 0 {
			return root, false
		}
		if comparison == 0 {
			root.parameter, root.lower, root.upper = boundary.value, boundary.value, boundary.value
			root.exact = new(big.Rat).SetFloat64(boundary.value)
		}
	}
	return root, true
}

// IntersectLineCylinder intersects an infinite analytic cylinder. Exactly
// parallel generators are returned as coincident parameter ranges; exactly
// parallel lines at other radial distances are disjoint. Near generators are
// not silently classified as coincident over an unbounded domain.
func IntersectLineCylinder(line Line, surface CylinderSurface, options LineSurfaceOptions) (LineSurfaceResult, error) {
	result := LineSurfaceResult{}
	o, e := lineSurfaceOptions(options)
	if e != nil {
		return result, e
	}
	if surface.radius <= 0 || !finite(surface.radius) {
		return result, ErrInvalid
	}
	origin, direction, e := localLine(line, surface.frame)
	if e != nil {
		return result, e
	}
	q := localQuadric(origin, direction, surface.radius, 0, false)
	roots, coincident, tangent, precision, e := solveExactQuadratic(q, o.MaxPrecision)
	result.Precision = precision
	if e != nil {
		return result, e
	}
	if coincident {
		result.Coincident = []LineParameterRange{{o.MinParameter, o.MaxParameter}}
		return result, nil
	}
	for _, root := range roots {
		root, inside := inLineRange(root, q, o)
		if !inside {
			continue
		}
		p, e := line.Eval(root.parameter)
		if e != nil {
			return result, e
		}
		local := surface.frame.ToLocal(p)
		residual := math.Abs(math.Hypot(local.X, local.Y) - surface.radius)
		if !local.IsFinite() || !finite(residual) {
			return result, ErrInvalid
		}
		if residual > o.DistanceTolerance {
			return result, ErrConvergence
		}
		result.Hits = append(result.Hits, LineSurfaceHit{root.parameter, root.lower, root.upper, p, residual, tangent, false})
	}
	return result, nil
}

// IntersectLineCone intersects the single nappe defined by ConeSurface (axial
// parameter v>=0), including its apex. The quadratic is solved without imposing
// a false epsilon on its discriminant. Roots on the opposite nappe are removed
// by exact algebraic sign comparisons. Generator coincidence returns the valid
// half-line, clipped to the requested range.
func IntersectLineCone(line Line, surface ConeSurface, options LineSurfaceOptions) (LineSurfaceResult, error) {
	result := LineSurfaceResult{}
	o, e := lineSurfaceOptions(options)
	if e != nil {
		return result, e
	}
	if !finite(surface.slope) || surface.slope <= 0 {
		return result, ErrInvalid
	}
	origin, direction, e := localLine(line, surface.frame)
	if e != nil {
		return result, e
	}
	q := localQuadric(origin, direction, 0, surface.slope, true)
	roots, coincident, tangent, precision, e := solveExactQuadratic(q, o.MaxPrecision)
	result.Precision = precision
	if e != nil {
		return result, e
	}
	var apexParameter *big.Rat
	if direction.Z != 0 {
		apexParameter = new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).SetFloat64(origin.Z), new(big.Rat).SetFloat64(direction.Z)))
	}
	if coincident {
		minimum, maximum := o.MinParameter, o.MaxParameter
		if direction.Z == 0 {
			if origin.Z < 0 {
				return result, nil
			}
		} else {
			bound, _ := apexParameter.Float64()
			if !finite(bound) {
				return result, ErrConvergence
			}
			if direction.Z > 0 {
				minimum = math.Max(minimum, rationalFloatBound(apexParameter, false))
			} else {
				maximum = math.Min(maximum, rationalFloatBound(apexParameter, true))
			}
		}
		if minimum <= maximum {
			result.Coincident = []LineParameterRange{{minimum, maximum}}
		}
		return result, nil
	}
	for _, root := range roots {
		root, inside := inLineRange(root, q, o)
		if !inside {
			continue
		}
		nappeSign := 0
		if direction.Z == 0 {
			nappeSign = new(big.Rat).SetFloat64(origin.Z).Sign()
		} else {
			nappeSign = root.compare(apexParameter, q)
			if direction.Z < 0 {
				nappeSign = -nappeSign
			}
		}
		if nappeSign < 0 {
			continue
		}
		p, e := line.Eval(root.parameter)
		if e != nil {
			return result, e
		}
		local := surface.frame.ToLocal(p)
		radial := math.Hypot(local.X, local.Y)
		scale := math.Hypot(surface.slope, 1)
		sin, cos := surface.slope/scale, 1/scale
		projection := sin*radial + cos*local.Z
		residual := math.Abs(cos*radial - sin*local.Z)
		if projection < 0 {
			residual = math.Hypot(radial, local.Z)
		}
		if !local.IsFinite() || !finite(projection) || !finite(residual) {
			return result, ErrInvalid
		}
		if residual > o.DistanceTolerance {
			return result, ErrConvergence
		}
		result.Hits = append(result.Hits, LineSurfaceHit{root.parameter, root.lower, root.upper, p, residual, tangent, nappeSign == 0})
	}
	return result, nil
}
