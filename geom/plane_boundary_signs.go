package geom

import (
	"math/big"
	"sort"
)

// PlaneBoundarySignOptions bounds exact boundary classification. Zero fields
// select parameter root tolerance 1e-10, 4096 spans/subdivisions, 1M rational
// operations and 16384 coefficient bits.
type PlaneBoundarySignOptions struct {
	ParameterTolerance                                           float64
	MaxSpans, MaxSubdivisions, MaxOperations, MaxCoefficientBits int
}

// PlaneBoundaryRoot encloses an exact edge/plane root. Parameter follows the
// edge's increasing other coordinate. Endpoint means a Bezier span endpoint,
// which may be an interior knot of the full edge. Multiplicity is one-sided.
type PlaneBoundaryRoot struct {
	Min, Max     float64
	Multiplicity int
	Endpoint     bool
}

// PlaneBoundarySpan classifies one knot span. Positive/Negative mean that sign
// occurs somewhere, not that the whole span has strict sign. Coincident is an
// identically zero plane numerator. Roots at shared knots may appear twice.
type PlaneBoundarySpan struct {
	Edge                           SurfaceEdge
	Start, End                     float64
	Positive, Negative, Coincident bool
	Roots                          []PlaneBoundaryRoot
}

// PlaneBoundarySigns reports the entire rectangular boundary. StrictSign is
// +1/-1 only if every point is strictly on that plane side (no roots/coincidence).
// WeakSign allows zeros but requires all nonzero values on one side. SignChanges
// means both signs occur; Tangencies means an even-multiplicity one-sided root.
// Complete false makes missing spans/signs unknown, never implicitly positive.
type PlaneBoundarySigns struct {
	Spans                                         []PlaneBoundarySpan
	StrictSign, WeakSign                          int
	SignChanges, Tangencies, Coincident, Complete bool
	ExactOperations, Subdivisions                 int
}

// ClassifyPlaneBoundarySigns uses exact homogeneous Bernstein numerators and
// polynomial Sturm/GCD isolation on all four domain edges. Positive weights
// make numerator signs identical to signed plane distance signs. Surface
// interiors are not classified. Clamped continuous SurfacePatch semantics apply.
func ClassifyPlaneBoundarySigns(p *SurfacePatch, plane Plane, o PlaneBoundarySignOptions) (result PlaneBoundarySigns, err error) {
	if p == nil || len(p.net) == 0 || !plane.valid() {
		return result, ErrInvalid
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-10
	}
	if o.MaxSpans == 0 {
		o.MaxSpans = 4096
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 || o.MaxSpans < 1 || o.MaxSubdivisions < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return result, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { result.ExactOperations = budget.operations }()
	positive, negative, zero := false, false, false
	for edge := SurfaceEdge(0); edge < 4; edge++ {
		indices := p.edgeIndices(edge)
		net := make([]exactHomogeneous, len(indices))
		for i, index := range indices {
			net[i] = p.net[index]
		}
		degree, knots := p.degreeV, append([]float64(nil), p.knotsV...)
		if edge >= SurfaceEdgeVMin {
			degree, knots = p.degreeU, append([]float64(nil), p.knotsU...)
		}
		internal := []float64{}
		for i := degree + 1; i < len(knots)-degree-1; i++ {
			if knots[i] > knots[degree] && knots[i] < knots[len(knots)-degree-1] && (len(internal) == 0 || internal[len(internal)-1] != knots[i]) {
				internal = append(internal, knots[i])
			}
		}
		for _, t := range internal {
			first := sort.SearchFloat64s(knots, t)
			last := sort.Search(len(knots), func(i int) bool { return knots[i] > t })
			count := degree - (last - first)
			if count > 0 {
				var e error
				net, e = refineExactNet(net, knots, degree, t, count, budget)
				if e != nil {
					return result, e
				}
				for i := 0; i < count; i++ {
					knots = append(knots, 0)
					copy(knots[last+1:], knots[last:])
					knots[last] = t
				}
			}
		}
		for k := degree; k < len(net); k++ {
			if knots[k] == knots[k+1] {
				continue
			}
			if len(result.Spans) >= o.MaxSpans {
				return result, ErrConvergence
			}
			span := PlaneBoundarySpan{Edge: edge, Start: knots[k], End: knots[k+1]}
			coeff := make([]*big.Rat, degree+1)
			for i, h := range net[k-degree : k+1] {
				value := new(big.Rat)
				for axis := 0; axis < 3; axis++ {
					offset := new(big.Rat).Sub(h[axis], new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(plane.origin, axis)), h[3]))
					value.Add(value, new(big.Rat).Mul(offset, new(big.Rat).SetFloat64(coordinate(plane.normal, axis))))
				}
				if e := budget.check(value); e != nil {
					return result, e
				}
				coeff[i] = value
			}
			poly, e := bernsteinToPower(coeff, budget)
			if e != nil {
				return result, e
			}
			if len(poly) == 0 {
				span.Coincident = true
				zero = true
				result.Coincident = true
				result.Spans = append(result.Spans, span)
				continue
			}
			// A nonzero degree-d polynomial cannot vanish at all d+1 distinct samples.
			sign := 0
			for sample := 0; sample < len(poly) && sign == 0; sample++ {
				x := big.NewRat(int64(sample), int64(max(1, len(poly)-1)))
				v, e := polynomialValue(poly, x, budget)
				if e != nil {
					return result, e
				}
				sign = v.Sign()
			}
			span.Positive, span.Negative = sign > 0, sign < 0
			remaining := o.MaxSubdivisions - result.Subdivisions
			if remaining < 1 {
				return result, ErrConvergence
			}
			subdivisions, e := isolatePolynomialRoots(poly, big.NewRat(0, 1), big.NewRat(1, 1), budget, remaining, func(lo, hi *big.Rat, m int) (bool, error) {
				mappedLo, mappedHi := parameterMap(ParameterSpan{Start: span.Start, End: span.End}, lo), parameterMap(ParameterSpan{Start: span.Start, End: span.End}, hi)
				a, c := rationalFloatBound(mappedLo, true), rationalFloatBound(mappedHi, false)
				if c-a > o.ParameterTolerance {
					return false, nil
				}
				endpoint := lo.Cmp(hi) == 0 && (lo.Sign() == 0 || lo.Cmp(big.NewRat(1, 1)) == 0)
				span.Roots = append(span.Roots, PlaneBoundaryRoot{a, c, m, endpoint})
				zero = true
				if m%2 == 0 {
					result.Tangencies = true
				} else if !endpoint {
					span.Positive, span.Negative = true, true
				}
				return true, nil
			})
			result.Subdivisions += subdivisions
			if e != nil {
				return result, e
			}
			positive, negative = positive || span.Positive, negative || span.Negative
			result.Spans = append(result.Spans, span)
		}
	}
	result.SignChanges = positive && negative
	if positive && !negative {
		result.WeakSign = 1
	} else if negative && !positive {
		result.WeakSign = -1
	}
	if !zero {
		result.StrictSign = result.WeakSign
	}
	result.Complete = true
	return result, nil
}
