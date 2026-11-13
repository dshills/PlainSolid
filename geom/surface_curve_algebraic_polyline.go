package geom

import (
	"context"
	"math"
	"math/big"
)

// ComposedCurvePolylineOptions bounds second-derivative chord certification.
// Defaults: deflection 1e-6, 4096 segments, depth 32, 8192 endpoint subdivisions,
// 8M operations and 16384 coefficient bits. Optional EndpointPoints are shared
// represented junctions; their errors are independently re-proved, never trusted.
type ComposedCurvePolylineOptions struct {
	SurfacePatchEvaluationOptions
	Deflection                                     float64
	MaxSegments, MaxDepth, MaxEndpointSubdivisions int
	EndpointPoints                                 *[2]Vec3
}

// ComposedPolylineVertex has an exact rational parameter for interior samples
// (singleton Parameter); algebraic endpoints retain their root enclosures.
type ComposedPolylineVertex struct {
	Point      Vec3
	Parameter  ExactParameterInterval
	Algebraic  bool
	ErrorUpper float64
}

// ComposedPolylineSegment's ErrorUpper bounds the pointwise curve-to-chord
// error under affine interpolation in the true endpoint parameters, including
// both represented vertex errors. ParameterHull encloses that true interval.
type ComposedPolylineSegment struct {
	Vertices      [2]int
	ParameterHull ExactParameterInterval
	ErrorUpper    float64
}

// CertifiedComposedPolyline retains certified prefix segments on limits. Only
// Complete proves coverage of the full algebraic cell domain.
type CertifiedComposedPolyline struct {
	Complete                              bool
	Vertices                              []ComposedPolylineVertex
	Segments                              []ComposedPolylineSegment
	MaximumError                          float64
	ExactOperations, EndpointSubdivisions int
}

// TessellateCertified approximates this cell with a provable model-space chord
// bound. All derivative calculations use original global parameters. If an
// endpoint cannot be represented within tolerance, or positivity/derivative
// bounds cannot be established within budgets, the result stays incomplete.
func (c *ExactComposedCurveCell) TessellateCertified(ctx context.Context, o ComposedCurvePolylineOptions) (r CertifiedComposedPolyline, err error) {
	if ctx == nil || c == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(c.power[3]) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.Deflection == 0 {
		o.Deflection = 1e-6
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 4096
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 32
	}
	if o.MaxEndpointSubdivisions == 0 {
		o.MaxEndpointSubdivisions = 8192
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.Deflection) || o.Deflection <= 0 || o.MaxSegments < 1 || o.MaxDepth < 1 || o.MaxDepth > 64 || o.MaxEndpointSubdivisions < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	if o.EndpointPoints != nil && (!o.EndpointPoints[0].IsFinite() || !o.EndpointPoints[1].IsFinite()) {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	tolerance := new(big.Rat).SetFloat64(o.Deflection)
	sqrtBits := 128
	if o.Deflection < 1 {
		sqrtBits += int(math.Ceil(-math.Log2(o.Deflection)))
	}
	sqrtUpper := func(value *big.Rat) (*big.Rat, error) {
		bits := sqrtBits
		available := (budget.maxBits - value.Num().BitLen()) / 2
		if bits > available {
			bits = available
		}
		if bits < 1 {
			return nil, ErrConvergence
		}
		_, upper, e := rationalSqrtBounds(value, bits, budget)
		return upper, e
	}
	type vertex struct {
		public ComposedPolylineVertex
		error  *big.Rat
	}
	var endpoints [2]vertex
	endpointTolerance := o.Deflection / 16
	if endpointTolerance == 0 {
		endpointTolerance = o.Deflection
	}
	for side := 0; side < 2; side++ {
		remaining := o.MaxOperations - budget.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		bounds, e := c.EncloseEndpoint(ctx, side, ComposedCurveEndpointOptions{ExactCurvePlaneEventOptions: ExactCurvePlaneEventOptions{MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxEndpointSubdivisions}, Tolerance: endpointTolerance})
		budget.operations += bounds.ExactOperations
		r.EndpointSubdivisions += bounds.Subdivisions
		if e != nil {
			return r, e
		}
		if !bounds.Converged || !bounds.HasRepresentative {
			return r, ErrConvergence
		}
		point := bounds.Point
		errorUpper := new(big.Rat).SetFloat64(bounds.ErrorUpper)
		if o.EndpointPoints != nil {
			point = o.EndpointPoints[side]
			coordinates := [3]float64{point.X, point.Y, point.Z}
			squared := new(big.Rat)
			for axis, f := range coordinates {
				represented := new(big.Rat).SetFloat64(f)
				lo := new(big.Rat).Abs(new(big.Rat).Sub(represented, bounds.Coordinates[axis].Lower))
				hi := new(big.Rat).Abs(new(big.Rat).Sub(represented, bounds.Coordinates[axis].Upper))
				if hi.Cmp(lo) > 0 {
					lo = hi
				}
				squared.Add(squared, new(big.Rat).Mul(lo, lo))
				if e := budget.check(squared); e != nil {
					return r, e
				}
			}
			errorUpper, e = sqrtUpper(squared)
			if e != nil {
				return r, e
			}
		}
		if errorUpper.Cmp(tolerance) >= 0 {
			return r, ErrConvergence
		}
		endpoints[side] = vertex{public: ComposedPolylineVertex{Point: point, Parameter: bounds.Parameter, Algebraic: bounds.Parameter.Lower.Cmp(bounds.Parameter.Upper) != 0, ErrorUpper: rationalFloatBound(errorUpper, false)}, error: errorUpper}
	}
	// C''=(D'W-2DW')/W³, D=N'W-NW'.
	var second [3]rationalPolynomial
	wd := polynomialDerivative(c.power[3])
	for axis := 0; axis < 3; axis++ {
		left, e := polynomialProduct(polynomialDerivative(c.power[axis]), c.power[3], budget)
		if e != nil {
			return r, e
		}
		right, e := polynomialProduct(c.power[axis], wd, budget)
		if e != nil {
			return r, e
		}
		first, e := subtractScaledPolynomial(left, right, big.NewRat(1, 1), budget)
		if e != nil {
			return r, e
		}
		left, e = polynomialProduct(polynomialDerivative(first), c.power[3], budget)
		if e != nil {
			return r, e
		}
		right, e = polynomialProduct(first, wd, budget)
		if e != nil {
			return r, e
		}
		second[axis], e = subtractScaledPolynomial(left, right, big.NewRat(2, 1), budget)
		if e != nil {
			return r, e
		}
	}
	sample := func(parameter *big.Rat) (vertex, error) {
		var out vertex
		denominator, e := polynomialValue(c.power[3], parameter, budget)
		if e != nil {
			return out, e
		}
		if denominator.Sign() <= 0 {
			return out, ErrConvergence
		}
		var coordinates [3]float64
		squared := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			n, e := polynomialValue(c.power[axis], parameter, budget)
			if e != nil {
				return out, e
			}
			exact := new(big.Rat).Quo(n, denominator)
			coordinates[axis], _ = exact.Float64()
			if !finite(coordinates[axis]) {
				return out, ErrUnrepresentable
			}
			delta := new(big.Rat).Sub(exact, new(big.Rat).SetFloat64(coordinates[axis]))
			squared.Add(squared, new(big.Rat).Mul(delta, delta))
			if e := budget.check(squared); e != nil {
				return out, e
			}
		}
		errorUpper, e := sqrtUpper(squared)
		if e != nil {
			return out, e
		}
		out.error = errorUpper
		out.public = ComposedPolylineVertex{Point: Vec3{coordinates[0], coordinates[1], coordinates[2]}, Parameter: ExactParameterInterval{Lower: new(big.Rat).Set(parameter), Upper: new(big.Rat).Set(parameter)}, ErrorUpper: rationalFloatBound(errorUpper, false)}
		return out, nil
	}
	r.Vertices = append(r.Vertices, endpoints[0].public)
	var visit func(vertex, vertex, int) error
	visit = func(start, end vertex, depth int) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if len(r.Segments) >= o.MaxSegments {
			return ErrConvergence
		}
		lo, hi := start.public.Parameter.Lower, end.public.Parameter.Upper
		parameter := exactInterval{lo: lo, hi: hi}
		denominator, e := polynomialBernsteinInterval(c.power[3], parameter, budget)
		if e != nil {
			return e
		}
		var bound *big.Rat
		if denominator.lo.Sign() > 0 {
			square, e := intervalMultiply(denominator, denominator, budget)
			if e != nil {
				return e
			}
			cube, e := intervalMultiply(square, denominator, budget)
			if e != nil {
				return e
			}
			normSquared := new(big.Rat)
			for axis := 0; axis < 3; axis++ {
				n, e := polynomialBernsteinInterval(second[axis], parameter, budget)
				if e != nil {
					return e
				}
				derivative, e := intervalDivide(n, cube, budget)
				if e != nil {
					return e
				}
				maximum := new(big.Rat).Abs(derivative.lo)
				other := new(big.Rat).Abs(derivative.hi)
				if other.Cmp(maximum) > 0 {
					maximum = other
				}
				normSquared.Add(normSquared, new(big.Rat).Mul(maximum, maximum))
				if e := budget.check(normSquared); e != nil {
					return e
				}
			}
			norm, e := sqrtUpper(normSquared)
			if e != nil {
				return e
			}
			width := new(big.Rat).Sub(hi, lo)
			bound = new(big.Rat).Quo(new(big.Rat).Mul(norm, new(big.Rat).Mul(width, width)), big.NewRat(8, 1))
			rounding := start.error
			if end.error.Cmp(rounding) > 0 {
				rounding = end.error
			}
			bound.Add(bound, rounding)
			if e := budget.check(bound); e != nil {
				return e
			}
		}
		if bound != nil && bound.Cmp(tolerance) <= 0 {
			first := len(r.Vertices) - 1
			r.Vertices = append(r.Vertices, end.public)
			errorUpper := rationalFloatBound(bound, false)
			r.Segments = append(r.Segments, ComposedPolylineSegment{Vertices: [2]int{first, first + 1}, ParameterHull: ExactParameterInterval{Lower: new(big.Rat).Set(lo), Upper: new(big.Rat).Set(hi)}, ErrorUpper: errorUpper})
			r.MaximumError = math.Max(r.MaximumError, errorUpper)
			return nil
		}
		if depth >= o.MaxDepth {
			return ErrConvergence
		}
		left, right := start.public.Parameter.Upper, end.public.Parameter.Lower
		if left.Cmp(right) >= 0 {
			return ErrConvergence
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(left, right), big.NewRat(2, 1))
		middle, e := sample(mid)
		if e != nil {
			return e
		}
		if e := visit(start, middle, depth+1); e != nil {
			return e
		}
		return visit(middle, end, depth+1)
	}
	if e := visit(endpoints[0], endpoints[1], 0); e != nil {
		return r, e
	}
	r.Complete = true
	return r, nil
}
