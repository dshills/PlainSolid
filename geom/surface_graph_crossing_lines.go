package geom

import (
	"context"
	"math/big"
)

// GraphCrossingLinesOptions bounds exhaustive candidate recovery and the full
// tensor factor proof. CandidateParameters are ignored: candidates are recovered
// separately on each axis. MaxCandidates bounds each axis and candidate pairs.
type GraphCrossingLinesOptions = GraphFactorAxisLineOptions

// GraphCrossingSector identifies an open parameter quadrant about Crossing.
// Direction entries are -1 or +1; Side is the strict sign of A.Z-B.Z.
type GraphCrossingSector struct {
	Direction [2]int
	Side      int
}

// SplineGraphCrossingLines certifies precisely two full crossing contact lines.
// The quotient has a single strict sign over the entire closed domain. No global
// derivative continuity across source knots is implied. Coordinates are owned
// by the caller; the report is evidence, not an immutable downstream certificate.
type SplineGraphCrossingLines struct {
	Proven   bool
	Crossing [2]*big.Rat
	Domain   ParameterRectangle
	// Endpoints[axis][end] gives UV on that fixed-coordinate contact line.
	Endpoints                                       [2][2][2]*big.Rat
	Sectors                                         [4]GraphCrossingSector
	Graph                                           SurfaceInjectivity
	CandidatesTested, ExactOperations, Subdivisions int
}

// DiscoverSplineGraphCrossingLines proves F=(u-r)(v-s)Q for the exact positive-
// weight common-XY height numerator, with Q uniformly strict Bernstein signed
// on all union-knot cells. Recovery from coefficient polynomials supplies only
// proposals: every accepted pair is independently divided on every cell. Thus
// unrecovered candidates cannot invalidate a successful exhaustive contact proof.
func DiscoverSplineGraphCrossingLines(ctx context.Context, a, c *SurfacePatch, o GraphCrossingLinesOptions) (r SplineGraphCrossingLines, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 {
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxRootSubdivisions < 1 || o.MaxCandidates < 1 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	d, e := orderingDifference(ctx, a, c, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, d, o.MaxCells, budget)
	if e != nil {
		return r, e
	}
	if len(cells) == 0 {
		return r, ErrConvergence
	}
	r.Domain = d.Domain()
	bounds := [2][2]float64{{r.Domain.UMin, r.Domain.UMax}, {r.Domain.VMin, r.Domain.VMax}}
	var candidates [2][]*big.Rat
	for axis := 0; axis < 2; axis++ {
		p := cells[0]
		n, m, lo, hi := p.countU, p.countV, p.Domain().UMin, p.Domain().UMax
		if axis == 1 {
			n, m, lo, hi = p.countV, p.countU, p.Domain().VMin, p.Domain().VMax
		}
		var poly rationalPolynomial
		for j := 0; j < m && len(poly) == 0; j++ {
			values := make([]*big.Rat, n)
			for i := range values {
				index := i*p.countV + j
				if axis == 1 {
					index = j*p.countV + i
				}
				values[i] = p.net[index][2]
			}
			poly, e = bernsteinToPower(values, budget)
			if e != nil {
				return r, e
			}
		}
		if len(poly) < 2 {
			return r, ErrConvergence
		}
		origin := new(big.Rat).SetFloat64(lo)
		width := new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), origin)
		lower := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[axis][0]), origin), width)
		upper := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(bounds[axis][1]), origin), width)
		remaining := o.MaxRootSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		steps, e := isolatePolynomialRoots(poly, lower, upper, budget, remaining, func(lo, hi *big.Rat, multiplicity int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			root, e := recoverRationalPolynomialRoot(poly, lo, hi, budget)
			if e != nil {
				return false, e
			}
			if root == nil {
				return new(big.Rat).Mul(new(big.Rat).Sub(hi, lo), width).Cmp(big.NewRat(1, 1000000000000)) <= 0, nil
			}
			physical := new(big.Rat).Add(origin, new(big.Rat).Mul(width, root))
			if multiplicity == 1 && physical.Cmp(new(big.Rat).SetFloat64(bounds[axis][0])) > 0 && physical.Cmp(new(big.Rat).SetFloat64(bounds[axis][1])) < 0 {
				if len(candidates[axis]) >= o.MaxCandidates {
					return false, ErrConvergence
				}
				candidates[axis] = append(candidates[axis], physical)
			}
			return true, nil
		})
		r.Subdivisions += steps
		if e != nil {
			return r, e
		}
	}
	for _, u := range candidates[0] {
		for _, v := range candidates[1] {
			if r.CandidatesTested >= o.MaxCandidates {
				return r, ErrConvergence
			}
			r.CandidatesTested++
			sign := 0
			proved := true
			for _, p := range cells {
				if e := ctx.Err(); e != nil {
					return r, e
				}
				q, e := divideGraphAxisBernstein(p, 0, u, budget)
				if e != nil {
					return r, e
				}
				if q == nil {
					proved = false
					break
				}
				q, e = divideGraphAxisBernstein(q, 1, v, budget)
				if e != nil {
					return r, e
				}
				if q == nil {
					proved = false
					break
				}
				for _, h := range q.net {
					current := h[2].Sign()
					if current == 0 || (sign != 0 && sign != current) {
						proved = false
						break
					}
					sign = current
				}
				if !proved {
					break
				}
			}
			if !proved || sign == 0 {
				continue
			}
			remaining := o.MaxOperations - budget.operations
			if remaining < 1 {
				return r, ErrConvergence
			}
			r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
			budget.operations += r.Graph.ExactOperations
			if e != nil {
				return r, e
			}
			if !r.Graph.ProvenInjective {
				return r, ErrConvergence
			}
			r.Crossing = [2]*big.Rat{new(big.Rat).Set(u), new(big.Rat).Set(v)}
			for axis := 0; axis < 2; axis++ {
				for end := 0; end < 2; end++ {
					r.Endpoints[axis][end][axis] = new(big.Rat).Set(r.Crossing[axis])
					r.Endpoints[axis][end][1-axis] = new(big.Rat).SetFloat64(bounds[1-axis][end])
				}
			}
			for i, dir := range [][2]int{{1, 1}, {-1, 1}, {-1, -1}, {1, -1}} {
				r.Sectors[i] = GraphCrossingSector{Direction: dir, Side: sign * dir[0] * dir[1]}
			}
			r.Proven = true
			return r, nil
		}
	}
	return r, ErrConvergence
}

// divideGraphAxisBernstein carries only the scalar Z coefficient net. Other
// homogeneous components are deliberately unused; this is not a public patch.
func divideGraphAxisBernstein(p *SurfacePatch, axis int, root *big.Rat, b *polynomialBudget) (*SurfacePatch, error) {
	n, m, lo, hi := p.countU, p.countV, p.Domain().UMin, p.Domain().UMax
	if axis == 1 {
		n, m, lo, hi = p.countV, p.countU, p.Domain().VMin, p.Domain().VMax
	}
	if n < 2 {
		return nil, nil
	}
	q := *p
	q.countU = p.countU
	q.countV = p.countV
	if axis == 0 {
		q.countU--
		q.degreeU--
		q.knotsU = append([]float64(nil), p.knotsU[1:len(p.knotsU)-1]...)
	} else {
		q.countV--
		q.degreeV--
		q.knotsV = append([]float64(nil), p.knotsV[1:len(p.knotsV)-1]...)
	}
	q.net = make([]exactHomogeneous, q.countU*q.countV)
	divisor := rationalPolynomial{new(big.Rat).Sub(new(big.Rat).SetFloat64(lo), root), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo))}
	for j := 0; j < m; j++ {
		values := make([]*big.Rat, n)
		for i := range values {
			index := i*p.countV + j
			if axis == 1 {
				index = j*p.countV + i
			}
			values[i] = p.net[index][2]
		}
		power, e := bernsteinToPower(values, b)
		if e != nil {
			return nil, e
		}
		quotient, remainder, e := polynomialDivide(power, divisor, b)
		if e != nil {
			return nil, e
		}
		if len(remainder) != 0 {
			return nil, nil
		}
		degree := n - 2
		for i := 0; i <= degree; i++ {
			value := new(big.Rat)
			for k, c := range quotient {
				if k > i {
					break
				}
				f := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(k)), new(big.Int).Binomial(int64(degree), int64(k)))
				value.Add(value, new(big.Rat).Mul(c, f))
				if e := b.check(value); e != nil {
					return nil, e
				}
			}
			index := i*q.countV + j
			if axis == 1 {
				index = j*q.countV + i
			}
			q.net[index][2] = value
		}
	}
	return &q, nil
}
