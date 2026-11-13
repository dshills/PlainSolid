package geom

import "sort"

// InsertUKnot inserts an interior represented knot exactly in homogeneous
// rational arithmetic. Shape and original parameterization are unchanged. The
// resulting multiplicity may not exceed degree (continuous patch convention).
// Use ToBSplineSurface to export only when coefficients fit float64 losslessly.
func (p *SurfacePatch) InsertUKnot(parameter float64, count int, o SurfacePatchOptions) (*SurfacePatch, error) {
	return p.refineExactPatch(parameter, count, true, o)
}

// InsertVKnot is the corresponding exact refinement in the V direction.
func (p *SurfacePatch) InsertVKnot(parameter float64, count int, o SurfacePatchOptions) (*SurfacePatch, error) {
	return p.refineExactPatch(parameter, count, false, o)
}
func (p *SurfacePatch) refineExactPatch(parameter float64, count int, u bool, o SurfacePatchOptions) (*SurfacePatch, error) {
	options, e := patchOptions(o)
	if e != nil {
		return nil, e
	}
	if p == nil || len(p.net) == 0 || !finite(parameter) || count < 0 {
		return nil, ErrInvalid
	}
	degree, n, other, knots := p.degreeU, p.countU, p.countV, p.knotsU
	if !u {
		degree, n, other, knots = p.degreeV, p.countV, p.countU, p.knotsV
	}
	if parameter <= knots[degree] || parameter >= knots[n] {
		return nil, ErrDomain
	}
	multiplicity := sort.Search(len(knots), func(i int) bool { return knots[i] > parameter }) - sort.SearchFloat64s(knots, parameter)
	if count > degree-multiplicity {
		return nil, ErrInvalid
	}
	if n+count > options.MaxPoles/other {
		return nil, ErrConvergence
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	net := make([]exactHomogeneous, (n+count)*other)
	for strip := 0; strip < other; strip++ {
		source := make([]exactHomogeneous, n)
		for i := range source {
			index := i*p.countV + strip
			if !u {
				index = strip*p.countV + i
			}
			source[i] = p.net[index]
			for _, x := range source[i] {
				if e := budget.check(x); e != nil {
					return nil, e
				}
			}
		}
		refined, e := refineExactNet(source, knots, degree, parameter, count, budget)
		if e != nil {
			return nil, e
		}
		for i, h := range refined {
			index := i*other + strip
			if !u {
				index = strip*(n+count) + i
			}
			net[index] = h
		}
	}
	result := *p
	result.net = net
	result.options = options
	if u {
		result.countU = n + count
		result.knotsU = insertionKnots(knots, parameter, count)
		result.knotsV = append([]float64(nil), p.knotsV...)
	} else {
		result.countV = n + count
		result.knotsV = insertionKnots(knots, parameter, count)
		result.knotsU = append([]float64(nil), p.knotsU...)
	}
	if e := result.computeBounds(budget); e != nil {
		return nil, e
	}
	result.operations = budget.operations
	return &result, nil
}
