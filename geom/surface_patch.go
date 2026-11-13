package geom

import (
	"math"
	"math/big"
	"sort"
)

// ParameterRectangle is a closed, positive-area tensor parameter domain.
type ParameterRectangle struct{ UMin, UMax, VMin, VMax float64 }

func (r ParameterRectangle) Valid() bool {
	return finite(r.UMin) && finite(r.UMax) && finite(r.VMin) && finite(r.VMax) && r.UMin < r.UMax && r.VMin < r.VMax
}

// SurfacePatchOptions bounds each exact subpatch operation. Zero fields select
// MaxPoles=4096 (including temporary refined nets), MaxOperations=1M and
// MaxCoefficientBits=16384. Rational values are immutable after construction.
type SurfacePatchOptions struct{ MaxPoles, MaxOperations, MaxCoefficientBits int }

func patchOptions(o SurfacePatchOptions) (SurfacePatchOptions, error) {
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles < 4 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return o, ErrInvalid
	}
	return o, nil
}

// SurfacePatch is an immutable exact rational restriction of the geometry
// defined by stored BSplineSurface coefficients. It does not round refined
// control geometry back into float64. Bounds are outward-rounded enclosures of
// that ideal rational patch; floating surface-evaluation error is additional.
// The source must be clamped, with interior knot multiplicity no greater than
// degree in either parameter direction. Full-multiplicity interior seams are
// rejected even if their geometric limits happen to coincide.
type SurfacePatch struct {
	degreeU, degreeV, countU, countV int
	knotsU, knotsV                   []float64
	net                              []exactHomogeneous
	bounds                           Box
	options                          SurfacePatchOptions
	operations                       int
}

func NewSurfacePatch(surface *BSplineSurface, options SurfacePatchOptions) (*SurfacePatch, error) {
	o, e := patchOptions(options)
	if e != nil {
		return nil, e
	}
	if surface == nil || len(surface.poles) == 0 {
		return nil, ErrInvalid
	}
	if len(surface.poles) > o.MaxPoles {
		return nil, ErrConvergence
	}
	if !projectionKnotsSupported(surface.degreeU, surface.countU, surface.knotsU) || !projectionKnotsSupported(surface.degreeV, surface.countV, surface.knotsV) {
		return nil, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	net := make([]exactHomogeneous, len(surface.poles))
	for i, p := range surface.poles {
		w := new(big.Rat).SetFloat64(surface.weights[i])
		if e := budget.check(w); e != nil {
			return nil, e
		}
		net[i][3] = w
		for axis := 0; axis < 3; axis++ {
			net[i][axis] = new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(p, axis)), w)
			if e := budget.check(net[i][axis]); e != nil {
				return nil, e
			}
		}
	}
	patch := &SurfacePatch{degreeU: surface.degreeU, degreeV: surface.degreeV, countU: surface.countU, countV: surface.countV, knotsU: append([]float64(nil), surface.knotsU...), knotsV: append([]float64(nil), surface.knotsV...), net: net, options: o}
	if e := patch.computeBounds(budget); e != nil {
		return nil, e
	}
	patch.operations = budget.operations
	return patch, nil
}

// Domain returns NaNs for a nil or zero-value patch.
func (p *SurfacePatch) Domain() ParameterRectangle {
	if p == nil || len(p.net) == 0 {
		return ParameterRectangle{math.NaN(), math.NaN(), math.NaN(), math.NaN()}
	}
	return ParameterRectangle{p.knotsU[p.degreeU], p.knotsU[p.countU], p.knotsV[p.degreeV], p.knotsV[p.countV]}
}
func (p *SurfacePatch) Bounds() (Box, error) {
	if p == nil || len(p.net) == 0 {
		return Box{}, ErrInvalid
	}
	return p.bounds, nil
}
func (p *SurfacePatch) ControlShape() (int, int) {
	if p == nil {
		return 0, 0
	}
	return p.countU, p.countV
}

// KnotBreaks returns independent increasing parameter cuts, including both
// ends of each clamped domain. Each consecutive interval is a single span.
func (p *SurfacePatch) KnotBreaks() (u, v []float64) {
	if p == nil {
		return nil, nil
	}
	cuts := func(k []float64) []float64 {
		var r []float64
		for _, x := range k {
			if len(r) == 0 || x != r[len(r)-1] {
				r = append(r, x)
			}
		}
		return r
	}
	return cuts(p.knotsU), cuts(p.knotsV)
}

// ExactOperations reports work in the most recent construction/refinement.
func (p *SurfacePatch) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}
func (p *SurfacePatch) computeBounds(budget *polynomialBudget) error {
	var minimum, maximum [3]*big.Rat
	var scratch big.Rat
	for _, h := range p.net {
		if h[3].Sign() <= 0 {
			return ErrInvalid
		}
		for axis := 0; axis < 3; axis++ {
			coordinate := h[axis]
			if !h[3].IsInt() || h[3].Num().BitLen() != 1 {
				coordinate = scratch.Quo(coordinate, h[3])
			}
			if e := budget.check(coordinate); e != nil {
				return e
			}
			if minimum[axis] == nil {
				minimum[axis] = new(big.Rat).Set(coordinate)
			} else if coordinate.Cmp(minimum[axis]) < 0 {
				minimum[axis].Set(coordinate)
			}
			if maximum[axis] == nil {
				maximum[axis] = new(big.Rat).Set(coordinate)
			} else if coordinate.Cmp(maximum[axis]) > 0 {
				maximum[axis].Set(coordinate)
			}
		}
	}
	p.bounds = Box{Vec3{rationalFloatBound(minimum[0], true), rationalFloatBound(minimum[1], true), rationalFloatBound(minimum[2], true)}, Vec3{rationalFloatBound(maximum[0], false), rationalFloatBound(maximum[1], false), rationalFloatBound(maximum[2], false)}}
	if !p.bounds.Valid() {
		return ErrInvalid
	}
	return nil
}
func insertionKnots(knots []float64, t float64, count int) []float64 {
	index := sort.Search(len(knots), func(i int) bool { return knots[i] > t })
	result := make([]float64, 0, len(knots)+count)
	result = append(result, knots[:index]...)
	for i := 0; i < count; i++ {
		result = append(result, t)
	}
	return append(result, knots[index:]...)
}
func (p *SurfacePatch) split(t float64, uDirection bool, budget *polynomialBudget) (*SurfacePatch, *SurfacePatch, error) {
	if p == nil || len(p.net) == 0 || !finite(t) {
		return nil, nil, ErrInvalid
	}
	degree, count, other, knots := p.degreeU, p.countU, p.countV, p.knotsU
	if !uDirection {
		degree, count, other, knots = p.degreeV, p.countV, p.countU, p.knotsV
	}
	if t <= knots[degree] || t >= knots[count] {
		return nil, nil, ErrDomain
	}
	first := sort.SearchFloat64s(knots, t)
	last := sort.Search(len(knots), func(i int) bool { return knots[i] > t })
	insertions := degree + 1 - (last - first)
	newCount := count + insertions
	if newCount > p.options.MaxPoles/other {
		return nil, nil, ErrConvergence
	}
	newKnots := insertionKnots(knots, t, insertions)
	first = sort.SearchFloat64s(newKnots, t)
	leftCount, rightCount := first, newCount-first
	leftNet, rightNet := make([]exactHomogeneous, leftCount*other), make([]exactHomogeneous, rightCount*other)
	for strip := 0; strip < other; strip++ {
		source := make([]exactHomogeneous, count)
		for i := range source {
			if uDirection {
				source[i] = p.net[i*p.countV+strip]
			} else {
				source[i] = p.net[strip*p.countV+i]
			}
		}
		refined, e := refineExactNet(source, knots, degree, t, insertions, budget)
		if e != nil {
			return nil, nil, e
		}
		for i := 0; i < leftCount; i++ {
			if uDirection {
				leftNet[i*other+strip] = refined[i]
			} else {
				leftNet[strip*leftCount+i] = refined[i]
			}
		}
		for i := 0; i < rightCount; i++ {
			if uDirection {
				rightNet[i*other+strip] = refined[leftCount+i]
			} else {
				rightNet[strip*rightCount+i] = refined[leftCount+i]
			}
		}
	}
	left, right := *p, *p
	left.net, right.net = leftNet, rightNet
	if uDirection {
		left.countU, right.countU = leftCount, rightCount
		left.knotsU = append([]float64(nil), newKnots[:first+degree+1]...)
		right.knotsU = append([]float64(nil), newKnots[first:]...)
	} else {
		left.countV, right.countV = leftCount, rightCount
		left.knotsV = append([]float64(nil), newKnots[:first+degree+1]...)
		right.knotsV = append([]float64(nil), newKnots[first:]...)
	}
	if e := left.computeBounds(budget); e != nil {
		return nil, nil, e
	}
	if e := right.computeBounds(budget); e != nil {
		return nil, nil, e
	}
	left.operations, right.operations = budget.operations, budget.operations
	return &left, &right, nil
}
func (p *SurfacePatch) SplitU(t float64) (*SurfacePatch, *SurfacePatch, error) {
	if p == nil {
		return nil, nil, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}
	return p.split(t, true, budget)
}
func (p *SurfacePatch) SplitV(t float64) (*SurfacePatch, *SurfacePatch, error) {
	if p == nil {
		return nil, nil, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}
	return p.split(t, false, budget)
}

// Trim exactly restricts a patch to a contained closed parameter rectangle.
// Positive rational weights make its refined control hull an enclosure; no
// sampling or unverified geometric epsilon is used to form the returned box.
func (p *SurfacePatch) Trim(rectangle ParameterRectangle) (*SurfacePatch, error) {
	if p == nil || len(p.net) == 0 || !rectangle.Valid() {
		return nil, ErrInvalid
	}
	domain := p.Domain()
	if rectangle.UMin < domain.UMin || rectangle.UMax > domain.UMax || rectangle.VMin < domain.VMin || rectangle.VMax > domain.VMax {
		return nil, ErrDomain
	}
	budget := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}
	current := p
	var e error
	if rectangle.UMin > domain.UMin {
		_, current, e = current.split(rectangle.UMin, true, budget)
		if e != nil {
			return nil, e
		}
	}
	if rectangle.UMax < domain.UMax {
		current, _, e = current.split(rectangle.UMax, true, budget)
		if e != nil {
			return nil, e
		}
	}
	if rectangle.VMin > domain.VMin {
		_, current, e = current.split(rectangle.VMin, false, budget)
		if e != nil {
			return nil, e
		}
	}
	if rectangle.VMax < domain.VMax {
		current, _, e = current.split(rectangle.VMax, false, budget)
		if e != nil {
			return nil, e
		}
	}
	result := *current
	result.operations = budget.operations
	return &result, nil
}

type PlanePatchRelation uint8

const (
	PlanePatchUnresolved PlanePatchRelation = iota
	PlanePatchAbove
	PlanePatchBelow
	PlanePatchCoplanar
)

// SurfacePlaneRange encloses signed plane distances. Relation uses exact
// rational signs even when outward conversion rounds a tiny bound to zero.
// Unresolved means the hull touches/straddles the plane, not proof of a root.
type SurfacePlaneRange struct {
	Min, Max        float64
	Relation        PlanePatchRelation
	ExactOperations int
}

func (p *SurfacePatch) planeBounds(plane Plane, budget *polynomialBudget) (SurfacePlaneRange, error) {
	if p == nil || len(p.net) == 0 || !plane.valid() {
		return SurfacePlaneRange{}, ErrInvalid
	}
	before := budget.operations
	var minimum, maximum *big.Rat
	equation := exactPlaneCoefficients(plane)
	for _, h := range p.net {
		value := exactPlaneHomogeneousValue(equation, h)
		value.Quo(value, h[3])
		if e := budget.check(value); e != nil {
			return SurfacePlaneRange{}, e
		}
		if minimum == nil || value.Cmp(minimum) < 0 {
			minimum = new(big.Rat).Set(value)
		}
		if maximum == nil || value.Cmp(maximum) > 0 {
			maximum = new(big.Rat).Set(value)
		}
	}
	relation := PlanePatchUnresolved
	if minimum.Sign() > 0 {
		relation = PlanePatchAbove
	} else if maximum.Sign() < 0 {
		relation = PlanePatchBelow
	} else if minimum.Sign() == 0 && maximum.Sign() == 0 {
		relation = PlanePatchCoplanar
	}
	return SurfacePlaneRange{rationalFloatBound(minimum, true), rationalFloatBound(maximum, false), relation, budget.operations - before}, nil
}
func (p *SurfacePatch) PlaneBounds(plane Plane) (SurfacePlaneRange, error) {
	if p == nil {
		return SurfacePlaneRange{}, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}
	return p.planeBounds(plane, budget)
}
