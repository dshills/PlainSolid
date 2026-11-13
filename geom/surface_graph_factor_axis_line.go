package geom

import (
	"context"
	"math/big"
)

// GraphFactorAxisLineOptions bounds exact rational factor discovery. Defaults
// add 8192 root subdivisions and 128 candidates to GraphPointContactOptions.
// CandidateParameters are optional exact proposals in original axis units;
// every proposal still undergoes the full factor and strict quotient proof.
type GraphFactorAxisLineOptions struct {
	GraphPointContactOptions
	MaxRootSubdivisions, MaxCandidates int
	CandidateParameters                []*big.Rat
}

// GraphFactorAxisLineDiscovery certifies only through Line.Proven. Unrecovered
// contains algebraic proposal brackets; their presence does not invalidate an
// accepted global proof. Failure is not an absence certificate.
type GraphFactorAxisLineDiscovery struct {
	Line SplineGraphAxisLine
	// Multiplicity is the exact common axis factor order when Line.Proven.
	Multiplicity                                    int
	CandidatesTested, ExactOperations, Subdivisions int
	Unrecovered                                     []ExactParameterInterval
}

// DiscoverSplineGraphFactorAxisLine permits higher-degree height numerators.
// It recovers rational candidates from one exact axis polynomial, divides every
// transverse Bernstein coefficient polynomial on every knot cell by the same
// physical linear factor, then proves all quotient Bernstein coefficients have
// one strict sign. This proves exactly one whole contact line and strict sides.
// Repeated factors and zero/mixed quotient bounds remain unresolved.
func DiscoverSplineGraphFactorAxisLine(ctx context.Context, a, c *SurfacePatch, axis int, o GraphFactorAxisLineOptions) (GraphFactorAxisLineDiscovery, error) {
	return discoverSplineGraphAxisContact(ctx, a, c, axis, o, false)
}

// DiscoverSplineGraphAxisContact proves exactly one rational interior axis
// contact line, allowing repeated factors. Its exact Multiplicity is uniform
// across all knot cells. Even multiplicity gives same-sided tangency; odd
// multiplicity gives crossing (only multiplicity one is transverse). Strict
// Bernstein quotient bounds exclude every additional contact. No global
// differentiability across source knots is claimed.
func DiscoverSplineGraphAxisContact(ctx context.Context, a, c *SurfacePatch, axis int, o GraphFactorAxisLineOptions) (GraphFactorAxisLineDiscovery, error) {
	return discoverSplineGraphAxisContact(ctx, a, c, axis, o, true)
}
func discoverSplineGraphAxisContact(ctx context.Context, a, c *SurfacePatch, axis int, o GraphFactorAxisLineOptions, repeated bool) (r GraphFactorAxisLineDiscovery, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || axis < 0 || axis > 1 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxRootSubdivisions == 0 {
		o.MaxRootSubdivisions = 8192
	}
	if o.MaxCandidates == 0 {
		o.MaxCandidates = 128
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxRootSubdivisions < 1 || o.MaxCandidates < 1 || len(o.CandidateParameters) > o.MaxCandidates {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations; r.Line.ExactOperations = budget.operations }()
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	domain := difference.Domain()
	bounds := [2][2]float64{{domain.UMin, domain.UMax}, {domain.VMin, domain.VMax}}
	var candidates []*big.Rat
	seen := map[string]bool{}
	add := func(root *big.Rat) error {
		if root == nil {
			return ErrInvalid
		}
		if e := budget.check(root); e != nil {
			return e
		}
		if root.Cmp(new(big.Rat).SetFloat64(bounds[axis][0])) <= 0 || root.Cmp(new(big.Rat).SetFloat64(bounds[axis][1])) >= 0 {
			return nil
		}
		key := root.RatString()
		if seen[key] {
			return nil
		}
		if len(candidates) >= o.MaxCandidates {
			return ErrConvergence
		}
		seen[key] = true
		candidates = append(candidates, new(big.Rat).Set(root))
		return nil
	}
	for _, candidate := range o.CandidateParameters {
		if e := add(candidate); e != nil {
			return r, e
		}
	}
	if len(candidates) == 0 {
		if len(cells) == 0 {
			return r, ErrConvergence
		}
		p := cells[0]
		count, other, lo, hi := p.countU, p.countV, p.Domain().UMin, p.Domain().UMax
		if axis == 1 {
			count, other, lo, hi = p.countV, p.countU, p.Domain().VMin, p.Domain().VMax
		}
		var polynomial rationalPolynomial
		for j := 0; j < other && len(polynomial) == 0; j++ {
			values := make([]*big.Rat, count)
			for i := range values {
				index := i*p.countV + j
				if axis == 1 {
					index = j*p.countV + i
				}
				values[i] = p.net[index][2]
			}
			polynomial, e = bernsteinToPower(values, budget)
			if e != nil {
				return r, e
			}
		}
		if len(polynomial) < 2 {
			return r, ErrConvergence
		}
		origin, width := new(big.Rat).SetFloat64(lo), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo))
		lower := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[axis][0]), origin), width)
		upper := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[axis][1]), origin), width)
		steps, e := isolatePolynomialRoots(polynomial, lower, upper, budget, o.MaxRootSubdivisions, func(lo, hi *big.Rat, _ int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			candidate, e := recoverRationalPolynomialRoot(polynomial, lo, hi, budget)
			if e != nil {
				return false, e
			}
			if candidate != nil {
				return true, add(new(big.Rat).Add(origin, new(big.Rat).Mul(width, candidate)))
			}
			left, right := new(big.Rat).Add(origin, new(big.Rat).Mul(width, lo)), new(big.Rat).Add(origin, new(big.Rat).Mul(width, hi))
			if new(big.Rat).Sub(right, left).Cmp(big.NewRat(1, 1000000000000)) > 0 {
				return false, nil
			}
			r.Unrecovered = append(r.Unrecovered, ExactParameterInterval{Lower: left, Upper: right})
			return true, nil
		})
		r.Subdivisions = steps
		if e != nil {
			return r, e
		}
	}
	for _, root := range candidates {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		r.CandidatesTested++
		maxMultiplicity := 1
		if repeated {
			maxMultiplicity = difference.degreeU
			if axis == 1 {
				maxMultiplicity = difference.degreeV
			}
		}
		var sign, multiplicity int
		proved := false
		for m := 1; m <= maxMultiplicity; m++ {
			roots := make([]*big.Rat, m)
			for i := range roots {
				roots[i] = root
			}
			var e error
			sign, proved, e = graphAxisStrictQuotients(ctx, cells, axis, roots, budget)
			if e != nil {
				return r, e
			}
			if proved {
				multiplicity = m
				break
			}
		}
		if !proved {
			continue
		}
		remaining := o.MaxOperations - budget.operations
		if remaining < 1 {
			return r, ErrConvergence
		}
		graph, e := CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
		budget.operations += graph.ExactOperations
		if e != nil {
			return r, e
		}
		if !graph.ProvenInjective {
			return r, ErrConvergence
		}
		line := SplineGraphAxisLine{Proven: true, Axis: axis, Coordinate: new(big.Rat).Set(root), LowerSide: -sign, UpperSide: sign, Domain: domain, Graph: graph}
		for _, p := range cells {
			line.CertifiedCells = append(line.CertifiedCells, p.Domain())
		}
		knots := difference.knotsU
		if axis == 1 {
			knots = difference.knotsV
		}
		for _, knot := range knots {
			line.OnKnot = line.OnKnot || root.Cmp(new(big.Rat).SetFloat64(knot)) == 0
		}
		for endpoint := 0; endpoint < 2; endpoint++ {
			line.Endpoints[endpoint][axis] = new(big.Rat).Set(root)
			line.Endpoints[endpoint][1-axis] = new(big.Rat).SetFloat64(bounds[1-axis][endpoint])
		}
		if multiplicity%2 == 0 {
			line.LowerSide = sign
		}
		r.Multiplicity = multiplicity
		r.Line = line
		return r, nil
	}
	return r, ErrConvergence
}
func graphAxisStrictQuotient(ctx context.Context, cells []*SurfacePatch, axis int, root *big.Rat, b *polynomialBudget) (sign int, proved bool, err error) {
	return graphAxisStrictQuotients(ctx, cells, axis, []*big.Rat{root}, b)
}

func graphAxisStrictQuotients(ctx context.Context, cells []*SurfacePatch, axis int, roots []*big.Rat, b *polynomialBudget) (sign int, proved bool, err error) {
	for _, p := range cells {
		if e := ctx.Err(); e != nil {
			return 0, false, e
		}
		count, other, lo, hi := p.countU, p.countV, p.Domain().UMin, p.Domain().UMax
		if axis == 1 {
			count, other, lo, hi = p.countV, p.countU, p.Domain().VMin, p.Domain().VMax
		}
		if count < 2 {
			return 0, false, nil
		}
		divisor := rationalPolynomial{big.NewRat(1, 1)}
		for _, root := range roots {
			var e error
			divisor, e = polynomialProduct(divisor, rationalPolynomial{new(big.Rat).Sub(new(big.Rat).SetFloat64(lo), root), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo))}, b)
			if e != nil {
				return 0, false, e
			}
		}
		if count-1 < len(roots) {
			return 0, false, nil
		}
		for j := 0; j < other; j++ {
			values := make([]*big.Rat, count)
			for i := range values {
				index := i*p.countV + j
				if axis == 1 {
					index = j*p.countV + i
				}
				values[i] = p.net[index][2]
			}
			power, e := bernsteinToPower(values, b)
			if e != nil {
				return 0, false, e
			}
			quotient, remainder, e := polynomialDivide(power, divisor, b)
			if e != nil {
				return 0, false, e
			}
			if len(remainder) != 0 {
				return 0, false, nil
			}
			degree := count - 1 - len(roots)
			for i := 0; i <= degree; i++ {
				coefficient := new(big.Rat)
				for k, q := range quotient {
					if k > i {
						break
					}
					factor := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(degree), int64(k)))
					coefficient.Add(coefficient, new(big.Rat).Mul(factor, q))
					if e := b.check(coefficient); e != nil {
						return 0, false, e
					}
				}
				if coefficient.Sign() == 0 {
					return 0, false, nil
				}
				if sign == 0 {
					sign = coefficient.Sign()
				} else if sign != coefficient.Sign() {
					return 0, false, nil
				}
			}
		}
	}
	return sign, sign != 0, nil
}
