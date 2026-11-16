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
// defined by stored BSplineSurface coefficients or supplied exact homogeneous
// Bernstein controls. It does not round refined
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

// WithLimits returns the same immutable geometry with new work limits. Existing
// controls must fit those limits before the patch can be used or subdivided.
func (p *SurfacePatch) WithLimits(options SurfacePatchOptions) (*SurfacePatch, error) {
	o, e := patchOptions(options)
	if e != nil {
		return nil, e
	}
	if p == nil || len(p.net) == 0 {
		return nil, ErrInvalid
	}
	if len(p.net) > o.MaxPoles {
		return nil, ErrConvergence
	}
	if p.options == o {
		return p, nil
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	for _, h := range p.net {
		for _, c := range h {
			if e := b.check(c); e != nil {
				return nil, e
			}
		}
	}
	out := *p
	out.options = o
	if e = out.computeBounds(b); e != nil {
		return nil, e
	}
	out.operations = b.operations
	return &out, nil
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
	var hull [3]exactFractionRange
	var numerator, denominator big.Int
	for _, h := range p.net {
		w := h[3]
		if w.Sign() <= 0 {
			return ErrInvalid
		}
		unitWeight := w.IsInt() && w.Num().BitLen() == 1
		for axis := range hull {
			coordinate := h[axis]
			n, d := coordinate.Num(), coordinate.Denom()
			if !unitWeight {
				numerator.Mul(n, w.Denom())
				denominator.Mul(d, w.Num())
				n, d = &numerator, &denominator
			}
			if err := hull[axis].extend(n, d, budget); err != nil {
				return err
			}
		}
	}
	ranges := [3]CertifiedScalarRange{
		certifiedRange(hull[0].interval()), certifiedRange(hull[1].interval()), certifiedRange(hull[2].interval()),
	}
	p.bounds = Box{
		Vec3{ranges[0].Min, ranges[1].Min, ranges[2].Min},
		Vec3{ranges[0].Max, ranges[1].Max, ranges[2].Max},
	}
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
	// Short knot plans are shared by all tensor strips. Keep generic
	// high-degree refinement incremental under its original work limit.
	var steps []exactKnotInsertion
	if degree <= 8 {
		steps = exactKnotRefinement(knots, degree, t, insertions)
	}
	for strip := 0; strip < other; strip++ {
		source := make([]exactHomogeneous, count)
		for i := range source {
			if uDirection {
				source[i] = p.net[i*p.countV+strip]
			} else {
				source[i] = p.net[strip*p.countV+i]
			}
		}
		var refined []exactHomogeneous
		var e error
		if steps != nil {
			refined, e = applyExactKnotRefinement(source, steps, budget)
		} else {
			refined, e = refineExactNet(source, knots, degree, t, insertions, budget)
		}
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
	return p.TrimWithWorkspace(rectangle, nil)
}

// SurfacePatchTrimWorkspace reuses exact split results when successive
// restrictions share a prefix. Its zero value is ready for use. It holds at
// most 1024 splits and 32768 control points for one operation and is used serially.
type SurfacePatchTrimWorkspace struct {
	splits map[surfacePatchTrimSplit]surfacePatchTrimChildren
	poles  int
}

type surfacePatchTrimSplit struct {
	patch         *SurfacePatch
	parameterBits uint64
	uDirection    bool
}

type surfacePatchTrimChildren struct {
	left, right *SurfacePatch
	work        int
}

func (w *SurfacePatchTrimWorkspace) split(p *SurfacePatch, t float64, u bool, budget *polynomialBudget) (*SurfacePatch, *SurfacePatch, error) {
	if w == nil {
		return p.split(t, u, budget)
	}
	// Signed zero is numerically the same cut, but child knots retain
	// the supplied parameter representation.
	key := surfacePatchTrimSplit{p, math.Float64bits(t), u}
	if split, ok := w.splits[key]; ok {
		// A completed split used the same coefficient limit on the same
		// immutable input. Charge its original work even on reuse. When
		// the full charge cannot fit, replay the split to preserve errors.
		if split.work <= budget.maxOperations-budget.operations {
			budget.operations += split.work
			return split.left, split.right, nil
		}
		return p.split(t, u, budget)
	}
	before := budget.operations
	left, right, err := p.split(t, u, budget)
	if err != nil {
		return nil, nil, err
	}
	poles := len(left.net) + len(right.net)
	if len(w.splits) < 1024 && poles <= 32768-w.poles {
		if w.splits == nil {
			w.splits = make(map[surfacePatchTrimSplit]surfacePatchTrimChildren)
		}
		w.splits[key] = surfacePatchTrimChildren{left, right, budget.operations - before}
		w.poles += poles
	}
	return left, right, nil
}

// TrimWithWorkspace exactly restricts a patch like Trim, with optional reuse
// across restrictions in a serial operation. Each prefix charges the same
// work to the patch's construction limits as an uncached restriction.
func (p *SurfacePatch) TrimWithWorkspace(rectangle ParameterRectangle, workspace *SurfacePatchTrimWorkspace) (*SurfacePatch, error) {
	if p == nil {
		return nil, ErrInvalid
	}
	out, _, err := p.trim(rectangle, p.options.MaxOperations, workspace)
	return out, err
}

// TrimWithWorkLimit exactly restricts a patch like Trim, with an explicit
// operation limit for this call. It reports the work charged even on failure
// and preserves the patch's original limits for subsequent calls.
func (p *SurfacePatch) TrimWithWorkLimit(rectangle ParameterRectangle, maxOperations int) (*SurfacePatch, int, error) {
	return p.trim(rectangle, maxOperations, nil)
}

func (p *SurfacePatch) trim(rectangle ParameterRectangle, maxOperations int, workspace *SurfacePatchTrimWorkspace) (_ *SurfacePatch, work int, err error) {
	if p == nil || len(p.net) == 0 || !rectangle.Valid() {
		return nil, 0, ErrInvalid
	}
	if maxOperations < 1 {
		return nil, 0, ErrInvalid
	}
	domain := p.Domain()
	if rectangle.UMin < domain.UMin || rectangle.UMax > domain.UMax || rectangle.VMin < domain.VMin || rectangle.VMax > domain.VMax {
		return nil, 0, ErrDomain
	}
	budget := &polynomialBudget{maxOperations: maxOperations, maxBits: p.options.MaxCoefficientBits}
	defer func() { work = min(budget.operations, budget.maxOperations) }()
	current := p
	var e error
	if rectangle.UMin > domain.UMin {
		_, current, e = workspace.split(current, rectangle.UMin, true, budget)
		if e != nil {
			return nil, 0, e
		}
	}
	if rectangle.UMax < domain.UMax {
		current, _, e = workspace.split(current, rectangle.UMax, true, budget)
		if e != nil {
			return nil, 0, e
		}
	}
	if rectangle.VMin > domain.VMin {
		_, current, e = workspace.split(current, rectangle.VMin, false, budget)
		if e != nil {
			return nil, 0, e
		}
	}
	if rectangle.VMax < domain.VMax {
		current, _, e = workspace.split(current, rectangle.VMax, false, budget)
		if e != nil {
			return nil, 0, e
		}
	}
	result := *current
	result.operations = budget.operations
	return &result, budget.operations, nil
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
