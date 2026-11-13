package geom

import (
	"fmt"
	"math"
	"math/big"
	"sort"
)

// SplinePlaneOptions bounds exact root isolation and final floating evaluation.
// Zero values choose 1e-10 parameter tolerance, 1e-8 distance tolerance, degree
// 12, 4096 poles, 1000000 rational operations, 16384 coefficient bits and 8192
// subdivisions. Degree may be raised to 64; coefficient growth can be costly.
type SplinePlaneOptions struct {
	ParameterTolerance, DistanceTolerance                                           float64
	MaxDegree, MaxControlPoints, MaxOperations, MaxCoefficientBits, MaxSubdivisions int
}

// ParameterSpan identifies a nonzero original knot span by parameter endpoints.
type ParameterSpan struct {
	Start, End float64
	Index      int
}

// PlaneCurveHit encloses an isolated root of the exact homogeneous plane
// numerator for stored float64 geometry. Parameter is a representative; Point
// and Residual use float64 evaluation. Multiplicity is exact within this span.
// Tangent means multiplicity >1; Crossing means odd multiplicity. At knot
// boundaries the incident spans intentionally report separate one-sided hits.
// ParameterMin/Max are outward-rounded exact rational isolation endpoints.
type PlaneCurveHit struct {
	Parameter, ParameterMin, ParameterMax float64
	Point                                 Vec3
	Residual                              float64
	Multiplicity                          int
	Tangent, Crossing                     bool
	Span                                  ParameterSpan
}

// SplinePlaneResult distinguishes isolated contacts from exactly coincident
// spans. On ErrConvergence it is partial and must not be treated as exhaustive.
type SplinePlaneResult struct {
	Hits                     []PlaneCurveHit
	Coincident               []ParameterSpan
	Operations, Subdivisions int
}

func rationalFloatBound(x *big.Rat, lower bool) float64 {
	v, _ := x.Float64()
	if math.IsInf(v, 1) {
		if lower {
			return math.MaxFloat64
		}
		return v
	}
	if math.IsInf(v, -1) {
		if !lower {
			return -math.MaxFloat64
		}
		return v
	}
	// Compare against the actual represented value, including signed underflow
	// to zero. The Float64 exact flag is deliberately not trusted.
	comparison := new(big.Rat).SetFloat64(v).Cmp(x)
	if lower && comparison > 0 {
		v = math.Nextafter(v, math.Inf(-1))
	}
	if !lower && comparison < 0 {
		v = math.Nextafter(v, math.Inf(1))
	}
	return v
}
func parameterMap(span ParameterSpan, x *big.Rat) *big.Rat {
	a := new(big.Rat).SetFloat64(span.Start)
	width := new(big.Rat).Sub(new(big.Rat).SetFloat64(span.End), a)
	return new(big.Rat).Add(a, new(big.Rat).Mul(width, x))
}

// IntersectSplinePlane exhaustively isolates roots on a clamped rational
// B-spline within explicit budgets. Exact scalar knot refinement and rational
// Sturm sequences detect even-multiplicity tangencies without sign sampling.
// Exactly coincident spans require every homogeneous coefficient to be exactly
// zero; near coincidence is not silently promoted to coincidence. Unclamped
// knots are rejected. Returned root brackets are exact for stored input numbers;
// Point residuals remain floating-point diagnostics, checked against tolerance.
func IntersectSplinePlane(curve *BSpline, plane Plane, options SplinePlaneOptions) (result SplinePlaneResult, err error) {
	o := options
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.DistanceTolerance == 0 {
		o.DistanceTolerance = 1e-8
	}
	if o.MaxDegree == 0 {
		o.MaxDegree = 12
	}
	if o.MaxControlPoints == 0 {
		o.MaxControlPoints = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if curve == nil || len(curve.poles) == 0 || !plane.valid() || !finite(o.ParameterTolerance) || !finite(o.DistanceTolerance) || o.ParameterTolerance <= 0 || o.DistanceTolerance <= 0 || o.MaxDegree < 1 || o.MaxDegree > 64 || o.MaxControlPoints < 2 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 {
		return result, ErrInvalid
	}
	if curve.degree > o.MaxDegree || len(curve.poles) > o.MaxControlPoints {
		return result, ErrConvergence
	}
	a, b := curve.Domain()
	for i := 0; i <= curve.degree; i++ {
		if curve.knots[i] != a || curve.knots[len(curve.knots)-1-i] != b {
			return result, fmt.Errorf("%w: intersection requires clamped knots", ErrInvalid)
		}
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		result.Operations = budget.operations
		sort.SliceStable(result.Hits, func(i, j int) bool {
			if result.Hits[i].Parameter == result.Hits[j].Parameter {
				return result.Hits[i].Span.Index < result.Hits[j].Span.Index
			}
			return result.Hits[i].Parameter < result.Hits[j].Parameter
		})
	}()
	breaks, coefficients, e := exactPlaneBezierSpans(curve, plane, budget)
	if e != nil {
		return result, e
	}
	hom := toHom(curve.poles, curve.weights)
	makeHit := func(span ParameterSpan, left, right *big.Rat, multiplicity int) (PlaneCurveHit, bool, error) {
		lo, hi := parameterMap(span, left), parameterMap(span, right)
		mid := new(big.Rat).Quo(new(big.Rat).Add(lo, hi), big.NewRat(2, 1))
		parameter, _ := mid.Float64()
		var p Vec3
		var e error
		if parameter == span.End {
			index := sort.SearchFloat64s(curve.knots, parameter) - 1
			p, e = projectHom(evalHomSpan(curve.degree, hom, curve.knots, parameter, index))
		} else {
			p, e = curve.Eval(parameter)
		}
		if e != nil {
			return PlaneCurveHit{}, false, e
		}
		residual := math.Abs(plane.SignedDistance(p))
		if !finite(residual) {
			return PlaneCurveHit{}, false, ErrInvalid
		}
		hit := PlaneCurveHit{parameter, rationalFloatBound(lo, true), rationalFloatBound(hi, false), p, residual, multiplicity, multiplicity > 1, multiplicity%2 == 1, span}
		width := new(big.Rat).Sub(hi, lo)
		return hit, width.Cmp(new(big.Rat).SetFloat64(o.ParameterTolerance)) <= 0 && residual <= o.DistanceTolerance, nil
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	for spanIndex := 0; spanIndex < len(breaks)-1; spanIndex++ {
		span := ParameterSpan{breaks[spanIndex], breaks[spanIndex+1], spanIndex}
		start := spanIndex * (curve.degree + 1)
		power, e := bernsteinToPower(coefficients[start:start+curve.degree+1], budget)
		if e != nil {
			return result, e
		}
		if len(power) == 0 {
			result.Coincident = append(result.Coincident, span)
			continue
		}
		interior := clonePolynomial(power)
		for _, endpoint := range []*big.Rat{zero, one} {
			multiplicity, e := rootMultiplicity(interior, endpoint, budget)
			if e != nil {
				return result, e
			}
			if multiplicity > 0 {
				hit, ok, e := makeHit(span, endpoint, endpoint, multiplicity)
				if e != nil {
					return result, e
				}
				if !ok {
					return result, ErrConvergence
				}
				result.Hits = append(result.Hits, hit)
				factor := rationalPolynomial{new(big.Rat).Neg(endpoint), big.NewRat(1, 1)}
				for j := 0; j < multiplicity; j++ {
					interior, _, e = polynomialDivide(interior, factor, budget)
					if e != nil {
						return result, e
					}
				}
			}
		}
		if len(interior) < 2 {
			continue
		}
		gcd, e := polynomialGCD(interior, polynomialDerivative(interior), budget)
		if e != nil {
			return result, e
		}
		squareFree, _, e := polynomialDivide(interior, gcd, budget)
		if e != nil {
			return result, e
		}
		sequence, e := polynomialSturm(squareFree, budget)
		if e != nil {
			return result, e
		}
		multiplicitySequences := [][]rationalPolynomial{}
		for len(gcd) > 1 {
			seq, e := polynomialSturm(gcd, budget)
			if e != nil {
				return result, e
			}
			multiplicitySequences = append(multiplicitySequences, seq)
			gcd, e = polynomialGCD(gcd, polynomialDerivative(gcd), budget)
			if e != nil {
				return result, e
			}
		}
		type interval struct {
			left, right *big.Rat
			count       int
		}
		count, e := sturmCount(sequence, zero, one, budget)
		if e != nil {
			return result, e
		}
		pending := []interval{{new(big.Rat), big.NewRat(1, 1), count}}
		for len(pending) > 0 {
			current := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if current.count == 0 {
				continue
			}
			if current.count == 1 {
				exact, e := polynomialValue(squareFree, current.right, budget)
				if e != nil {
					return result, e
				}
				left, right := current.left, current.right
				if exact.Sign() == 0 {
					left = right
				}
				multiplicity := 1
				if left.Cmp(right) == 0 {
					multiplicity, e = rootMultiplicity(power, left, budget)
					if e != nil {
						return result, e
					}
				} else {
					for _, seq := range multiplicitySequences {
						n, e := sturmCount(seq, left, right, budget)
						if e != nil {
							return result, e
						}
						if n > 0 {
							multiplicity++
						}
					}
				}
				hit, ok, e := makeHit(span, left, right, multiplicity)
				if e != nil {
					return result, e
				}
				if ok {
					result.Hits = append(result.Hits, hit)
					continue
				}
				if left.Cmp(right) == 0 {
					return result, ErrConvergence
				}
			}
			if result.Subdivisions >= o.MaxSubdivisions {
				return result, ErrConvergence
			}
			result.Subdivisions++
			mid := new(big.Rat).Quo(new(big.Rat).Add(current.left, current.right), big.NewRat(2, 1))
			leftCount, e := sturmCount(sequence, current.left, mid, budget)
			if e != nil {
				return result, e
			}
			pending = append(pending, interval{mid, current.right, current.count - leftCount}, interval{current.left, mid, leftCount})
		}
	}
	return result, nil
}
