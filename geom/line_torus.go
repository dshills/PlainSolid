package geom

import (
	"math"
	"math/big"
	"sort"
)

// LineTorusOptions bounds exact quartic root isolation. Zero values select
// distance tolerance 1e-9, absolute/relative parameter tolerances 1e-10/1e-12,
// 1M rational operations, 32768 coefficient bits and 8192 subdivisions.
// Without UseParameterRange the entire infinite line is searched; finite or
// infinite bounds may otherwise specify a closed interval.
type LineTorusOptions struct {
	UseParameterRange                                                 bool
	MinParameter, MaxParameter                                        float64
	DistanceTolerance, ParameterTolerance, RelativeParameterTolerance float64
	MaxOperations, MaxCoefficientBits, MaxSubdivisions                int
}

// LineTorusHit adds exact quartic multiplicity to the usual line hit data.
// Tangent means multiplicity >1; Crossing means odd multiplicity. A ring torus
// may have a fourth-order contact, which is preserved rather than merged into a
// generic numerical double root.
type LineTorusHit struct {
	LineSurfaceHit
	Multiplicity int
	Crossing     bool
}

// LineTorusResult contains isolated contacts; a finite ring torus cannot contain
// an entire straight generator. Results are partial on error, not exhaustive.
type LineTorusResult struct {
	Hits                     []LineTorusHit
	Operations, Subdivisions int
}

func torusLinePolynomial(origin, direction Vec3, major, minor float64, budget *polynomialBudget) (rationalPolynomial, error) {
	radius := new(big.Rat).SetFloat64(major)
	radius.Mul(radius, radius)
	tube := new(big.Rat).SetFloat64(minor)
	tube.Mul(tube, tube)
	sphere := rationalPolynomial{new(big.Rat).Sub(radius, tube), new(big.Rat), new(big.Rat)}
	radial := rationalPolynomial{new(big.Rat), new(big.Rat), new(big.Rat)}
	for axis := 0; axis < 3; axis++ {
		x, d := new(big.Rat).SetFloat64(coordinate(origin, axis)), new(big.Rat).SetFloat64(coordinate(direction, axis))
		terms := []*big.Rat{new(big.Rat).Mul(x, x), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(x, d)), new(big.Rat).Mul(d, d)}
		for i, term := range terms {
			sphere[i].Add(sphere[i], term)
			if e := budget.check(sphere[i]); e != nil {
				return nil, e
			}
			if axis < 2 {
				radial[i].Add(radial[i], term)
			}
		}
	}
	result, e := polynomialProduct(sphere, sphere, budget)
	if e != nil {
		return nil, e
	}
	factor := new(big.Rat).Mul(big.NewRat(4, 1), radius)
	for i, coefficient := range radial {
		result[i].Sub(result[i], new(big.Rat).Mul(factor, coefficient))
		if e := budget.check(result[i]); e != nil {
			return nil, e
		}
	}
	return trimPolynomial(result), nil
}

// IntersectLineTorus finds all contacts of a line with a ring TorusSurface.
// Exact rational quartic coefficients, GCDs and Sturm sequences classify and
// bracket roots without epsilon discriminants or sampled crossing detection.
// Root bounds refer to the stored local float64 line coefficients; every world
// point must also satisfy the requested geometric torus distance residual.
func IntersectLineTorus(line Line, surface TorusSurface, options LineTorusOptions) (result LineTorusResult, err error) {
	o := options
	if o.DistanceTolerance == 0 {
		o.DistanceTolerance = 1e-9
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.RelativeParameterTolerance == 0 {
		o.RelativeParameterTolerance = 1e-12
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 32768
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if !finite(o.DistanceTolerance) || o.DistanceTolerance <= 0 || !finite(o.ParameterTolerance) || o.ParameterTolerance < 0 || !finite(o.RelativeParameterTolerance) || o.RelativeParameterTolerance < 0 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxSubdivisions < 1 || !surface.frame.valid() || !finite(surface.major) || !finite(surface.minor) || surface.minor <= 0 || surface.major <= surface.minor {
		return result, ErrInvalid
	}
	if o.UseParameterRange {
		if math.IsNaN(o.MinParameter) || math.IsNaN(o.MaxParameter) || o.MinParameter > o.MaxParameter || math.IsInf(o.MinParameter, 1) || math.IsInf(o.MaxParameter, -1) {
			return result, ErrInvalid
		}
	} else {
		o.MinParameter, o.MaxParameter = math.Inf(-1), math.Inf(1)
	}
	origin, direction, e := localLine(line, surface.frame)
	if e != nil {
		return result, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() {
		result.Operations = budget.operations
		sort.SliceStable(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter < result.Hits[j].Parameter })
	}()
	polynomial, e := torusLinePolynomial(origin, direction, surface.major, surface.minor, budget)
	if e != nil {
		return result, e
	}
	bound, e := polynomialRootBound(polynomial, budget)
	if e != nil {
		return result, e
	}
	lower, upper := new(big.Rat).Neg(bound), new(big.Rat).Set(bound)
	if finite(o.MinParameter) {
		minimum := new(big.Rat).SetFloat64(o.MinParameter)
		if minimum.Cmp(lower) > 0 {
			lower = minimum
		}
	}
	if finite(o.MaxParameter) {
		maximum := new(big.Rat).SetFloat64(o.MaxParameter)
		if maximum.Cmp(upper) < 0 {
			upper = maximum
		}
	}
	accept := func(left, right *big.Rat, multiplicity int) (bool, error) {
		mid := new(big.Rat).Quo(new(big.Rat).Add(left, right), big.NewRat(2, 1))
		parameter, _ := mid.Float64()
		if !finite(parameter) {
			return false, nil
		}
		minimum, maximum := rationalFloatBound(left, true), rationalFloatBound(right, false)
		if !finite(minimum) || !finite(maximum) {
			return false, nil
		}
		threshold := o.ParameterTolerance + o.RelativeParameterTolerance*math.Abs(parameter)
		if !finite(threshold) {
			return false, ErrInvalid
		}
		if maximum-minimum > threshold {
			return false, nil
		}
		point, e := line.Eval(parameter)
		if e != nil {
			return false, e
		}
		local := surface.frame.ToLocal(point)
		residual := math.Abs(math.Hypot(math.Hypot(local.X, local.Y)-surface.major, local.Z) - surface.minor)
		if !local.IsFinite() || !finite(residual) {
			return false, ErrInvalid
		}
		if residual > o.DistanceTolerance {
			lf, _ := left.Float64()
			rf, _ := right.Float64()
			if lf == rf {
				return false, ErrConvergence
			}
			return false, nil
		}
		result.Hits = append(result.Hits, LineTorusHit{LineSurfaceHit{parameter, minimum, maximum, point, residual, multiplicity > 1, false}, multiplicity, multiplicity%2 == 1})
		return true, nil
	}
	subdivisions, e := isolatePolynomialRoots(polynomial, lower, upper, budget, o.MaxSubdivisions, accept)
	result.Subdivisions = subdivisions
	if e != nil {
		return result, e
	}
	sort.SliceStable(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter < result.Hits[j].Parameter })
	for i := 1; i < len(result.Hits); i++ {
		if result.Hits[i].Parameter == result.Hits[i-1].Parameter {
			return result, ErrConvergence
		}
	}
	return result, nil
}
