package geom

import (
	"context"
	"math/big"
	"sort"
)

// GraphContactBand has strict height Side on its open axis interval and the
// entire closed transverse domain. Interior interval endpoints are contact lines.
type GraphContactBand struct {
	Start, End *big.Rat
	Side       int
}
type ParallelGraphContactLine struct {
	Coordinate           *big.Rat
	Endpoints            [2][2]*big.Rat
	OnKnot               bool
	LowerSide, UpperSide int
}

// SplineGraphParallelLines exhausts the contact set when Proven. Every line is
// simple/transverse in the height function, and Bands give complementary signs.
// Zero lines is a proved strict separation. Unrecovered/Repeated diagnostics do
// not themselves establish contact, and failure is not an absence certificate.
type SplineGraphParallelLines struct {
	Proven                        bool
	Axis                          int
	Domain                        ParameterRectangle
	Lines                         []ParallelGraphContactLine
	Bands                         []GraphContactBand
	Graph                         SurfaceInjectivity
	Unrecovered                   []ExactParameterInterval
	Repeated                      []*big.Rat
	ExactOperations, Subdivisions int
}

// DiscoverSplineGraphParallelLines requires a common product of distinct simple
// rational axis factors on every union-knot cell and a strictly signed Bernstein
// quotient. All roots of one coefficient polynomial in the original axis domain
// are isolated; unresolved algebraic/repeated roots conservatively reject this
// subset. CandidateParameters are not used: enumeration cannot rely on hints.
func DiscoverSplineGraphParallelLines(ctx context.Context, a, c *SurfacePatch, axis int, o GraphFactorAxisLineOptions) (r SplineGraphParallelLines, err error) {
	r.Axis = axis
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
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 1 || o.MaxDepth < 0 || o.MaxRootSubdivisions < 1 || o.MaxCandidates < 1 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	difference, e := orderingDifference(ctx, a, c, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	cells, e := graphContactBezierCells(ctx, difference, o.MaxCells, b)
	if e != nil {
		return r, e
	}
	if len(cells) == 0 {
		return r, ErrConvergence
	}
	r.Domain = difference.Domain()
	bounds := [2][2]float64{{r.Domain.UMin, r.Domain.UMax}, {r.Domain.VMin, r.Domain.VMax}}
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
		polynomial, e = bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
	}
	if len(polynomial) == 0 {
		return r, ErrConvergence
	}
	origin, width := new(big.Rat).SetFloat64(lo), new(big.Rat).Sub(new(big.Rat).SetFloat64(hi), new(big.Rat).SetFloat64(lo))
	domainLo, domainHi := new(big.Rat).SetFloat64(bounds[axis][0]), new(big.Rat).SetFloat64(bounds[axis][1])
	lower := new(big.Rat).Quo(new(big.Rat).Sub(domainLo, origin), width)
	upper := new(big.Rat).Quo(new(big.Rat).Sub(domainHi, origin), width)
	var roots []*big.Rat
	unsupported := false
	steps, e := isolatePolynomialRoots(polynomial, lower, upper, b, o.MaxRootSubdivisions, func(lo, hi *big.Rat, multiplicity int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		candidate, e := recoverRationalPolynomialRoot(polynomial, lo, hi, b)
		if e != nil {
			return false, e
		}
		if candidate != nil {
			root := new(big.Rat).Add(origin, new(big.Rat).Mul(width, candidate))
			if root.Cmp(domainLo) <= 0 || root.Cmp(domainHi) >= 0 {
				unsupported = true
				return true, nil
			}
			if multiplicity != 1 {
				r.Repeated = append(r.Repeated, root)
				unsupported = true
				return true, nil
			}
			if len(roots) >= o.MaxCandidates {
				return false, ErrConvergence
			}
			roots = append(roots, root)
			return true, nil
		}
		left, right := new(big.Rat).Add(origin, new(big.Rat).Mul(width, lo)), new(big.Rat).Add(origin, new(big.Rat).Mul(width, hi))
		if new(big.Rat).Sub(right, left).Cmp(big.NewRat(1, 1000000000000)) > 0 {
			return false, nil
		}
		r.Unrecovered = append(r.Unrecovered, ExactParameterInterval{Lower: left, Upper: right})
		unsupported = true
		return true, nil
	})
	r.Subdivisions = steps
	if e != nil {
		return r, e
	}
	if unsupported {
		return r, ErrConvergence
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Cmp(roots[j]) < 0 })
	for i := 1; i < len(roots); i++ {
		if roots[i].Cmp(roots[i-1]) <= 0 {
			return r, ErrConvergence
		}
	}
	sign, proved, e := graphAxisStrictQuotients(ctx, cells, axis, roots, b)
	if e != nil {
		return r, e
	}
	if !proved {
		return r, ErrConvergence
	}
	remaining := o.MaxOperations - b.operations
	if remaining < 1 {
		return r, ErrConvergence
	}
	r.Graph, e = CertifySurfaceInjectivity(a, SurfaceInjectivityOptions{ProjectionAxes: [2]int{0, 1}, MaxCells: o.MaxCells, MaxOperations: remaining, MaxCoefficientBits: o.MaxCoefficientBits})
	b.operations += r.Graph.ExactOperations
	if e != nil {
		return r, e
	}
	if !r.Graph.ProvenInjective {
		return r, ErrConvergence
	}
	cuts := append([]*big.Rat{domainLo}, roots...)
	cuts = append(cuts, domainHi)
	side := sign
	if len(roots)%2 == 1 {
		side = -side
	}
	for i := 0; i < len(cuts)-1; i++ {
		r.Bands = append(r.Bands, GraphContactBand{Start: new(big.Rat).Set(cuts[i]), End: new(big.Rat).Set(cuts[i+1]), Side: side})
		side = -side
	}
	knots := difference.knotsU
	if axis == 1 {
		knots = difference.knotsV
	}
	for i, root := range roots {
		line := ParallelGraphContactLine{Coordinate: new(big.Rat).Set(root), LowerSide: r.Bands[i].Side, UpperSide: r.Bands[i+1].Side}
		for _, knot := range knots {
			line.OnKnot = line.OnKnot || root.Cmp(new(big.Rat).SetFloat64(knot)) == 0
		}
		for endpoint := 0; endpoint < 2; endpoint++ {
			line.Endpoints[endpoint][axis] = new(big.Rat).Set(root)
			line.Endpoints[endpoint][1-axis] = new(big.Rat).SetFloat64(bounds[1-axis][endpoint])
		}
		r.Lines = append(r.Lines, line)
	}
	r.Proven = true
	return r, nil
}
