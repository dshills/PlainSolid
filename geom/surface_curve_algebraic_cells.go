package geom

import (
	"context"
	"math/big"
)

// ExactComposedCurveCell retains an exact rational model continuation in the
// original global UV-curve parameter. Its actual domain endpoints are private
// algebraic roots. Positive denominator is proved on that entire closed domain,
// not on arbitrary continuation parameters. The descriptor is immutable.
type ExactComposedCurveCell struct {
	power     [4]rationalPolynomial
	endpoints [2]*ExactGraphLineRoot
	cell      ParameterRectangle
}

func (c *ExactComposedCurveCell) Cell() ParameterRectangle {
	if c == nil {
		return ParameterRectangle{}
	}
	return c.cell
}

// EndpointIntervals are owned rational root enclosures, not exact domain cuts.
func (c *ExactComposedCurveCell) EndpointIntervals() [2]ExactParameterInterval {
	if c == nil {
		return [2]ExactParameterInterval{}
	}
	return [2]ExactParameterInterval{c.endpoints[0].Interval(), c.endpoints[1].Interval()}
}

// ClassifyEndpoint determines whether an exact cell endpoint is rational,
// using the retained root identity rather than its display enclosure.
func (c *ExactComposedCurveCell) ClassifyEndpoint(ctx context.Context, endpoint int, o ExactCurvePlaneEventOptions) (ExactRootClassification, error) {
	if c == nil || endpoint < 0 || endpoint > 1 || c.endpoints[endpoint] == nil {
		return ExactRootClassification{}, ErrInvalid
	}
	root := ExactAlgebraicRoot{root: c.endpoints[endpoint]}
	return root.Classify(ctx, o)
}

// HomogeneousPower returns caller-owned ascending power coefficients of
// (w*x,w*y,w*z,w) in the original global parameter. These do not make irrational
// endpoint parameters representable in an exchange format.
func (c *ExactComposedCurveCell) HomogeneousPower(ctx context.Context, o SurfacePatchEvaluationOptions) (out [4][]*big.Rat, operations int, err error) {
	if ctx == nil || c == nil || len(c.power[3]) == 0 {
		return out, 0, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, 0, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, 0, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	for axis := range out {
		out[axis] = make([]*big.Rat, len(c.power[axis]))
		for i, x := range c.power[axis] {
			if e := ctx.Err(); e != nil {
				return [4][]*big.Rat{}, 0, e
			}
			if e := b.check(x); e != nil {
				return [4][]*big.Rat{}, 0, e
			}
			out[axis][i] = new(big.Rat).Set(x)
		}
	}
	return out, b.operations, nil
}

// RationalComposedCurvePoint retains exact model coordinates and outward bounds.
type RationalComposedCurvePoint struct {
	Point                         [3]*big.Rat
	Bounds                        Box
	ExactOperations, Subdivisions int
}

// EvaluateRational first proves parameter membership against both algebraic
// endpoints. ErrDomain means the represented rational lies outside this cell's
// actual interval, even if it lies inside a loose endpoint enclosure.
func (c *ExactComposedCurveCell) EvaluateRational(ctx context.Context, parameter *big.Rat, o ExactCurvePlaneEventOptions) (r RationalComposedCurvePoint, err error) {
	if ctx == nil || c == nil || parameter == nil || c.endpoints[0] == nil || c.endpoints[1] == nil || len(c.power[3]) == 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	o, e := graphRootOptions(o)
	if e != nil {
		return r, e
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	if e := b.check(parameter); e != nil {
		return r, e
	}
	cap := rationalPolynomial{new(big.Rat).Neg(parameter), big.NewRat(1, 1)}
	for side, root := range c.endpoints {
		for _, bound := range root.bounds {
			if e := b.check(bound); e != nil {
				return r, e
			}
		}
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		for _, x := range root.polynomial {
			if e := b.check(x); e != nil {
				return r, e
			}
		}
		comparison, e := compareExactPolynomialEvents(ctx, [2]rationalPolynomial{root.polynomial, cap}, [2][2]*big.Rat{root.bounds, {new(big.Rat).Set(parameter), new(big.Rat).Set(parameter)}}, b, remaining)
		r.Subdivisions += comparison.Subdivisions
		if e != nil {
			return r, e
		}
		if !comparison.Proven {
			return r, ErrConvergence
		}
		if side == 0 && comparison.Order > 0 || side == 1 && comparison.Order < 0 {
			return r, ErrDomain
		}
	}
	denominator, e := polynomialValue(c.power[3], parameter, b)
	if e != nil {
		return r, e
	}
	if denominator.Sign() <= 0 {
		return r, ErrConvergence
	}
	for axis := 0; axis < 3; axis++ {
		numerator, e := polynomialValue(c.power[axis], parameter, b)
		if e != nil {
			return r, e
		}
		r.Point[axis] = new(big.Rat).Quo(numerator, denominator)
		if e := b.check(r.Point[axis]); e != nil {
			return r, e
		}
	}
	r.Bounds = Box{Min: Vec3{rationalFloatBound(r.Point[0], true), rationalFloatBound(r.Point[1], true), rationalFloatBound(r.Point[2], true)}, Max: Vec3{rationalFloatBound(r.Point[0], false), rationalFloatBound(r.Point[1], false), rationalFloatBound(r.Point[2], false)}}
	return r, nil
}

// SurfaceCurveCompositionCells is an ordered full-domain algebraic partition.
// Each neighboring pair shares an exact private endpoint identity. Events are
// display enclosures only; mutation never alters descriptor provenance.
type SurfaceCurveCompositionCells struct {
	Complete                      bool
	Cells                         []*ExactComposedCurveCell
	Events                        []ExactParameterInterval
	ExactOperations, Subdivisions int
}

// BuildSurfaceCurveCompositionCells retains symbolic per-cell continuations
// even when UV knot crossings are irrational. Complete coverage, event order,
// cell membership and positive denominators follow from exhaustive exact knot
// event enumeration and continuity. It exports no rounded algebraic cuts.
func (p *SurfacePatch) BuildSurfaceCurveCompositionCells(ctx context.Context, uv *ExactSplineCurve, o SurfaceCurveAgreementOptions) (r SurfaceCurveCompositionCells, err error) {
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	o.SurfacePatchOptions = options
	var events []*ExactGraphLineRoot
	proof, e := compareSurfaceCurveCompositions(ctx, p, uv, p, uv, o, &events)
	r.ExactOperations = proof.ExactOperations
	r.Subdivisions = proof.Subdivisions
	if e != nil {
		return r, e
	}
	if !proof.Complete || !proof.Equal {
		return r, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations - proof.ExactOperations, maxBits: options.MaxCoefficientBits}
	if b.maxOperations < 1 {
		return r, ErrConvergence
	}
	defer func() { r.ExactOperations = proof.ExactOperations + b.operations }()
	var coordinates [3]rationalPolynomial
	for axis, component := range []int{0, 1, 3} {
		values := make([]*big.Rat, uv.patch.countU)
		for i := range values {
			values[i] = uv.patch.net[i*uv.patch.countV][component]
		}
		coordinates[axis], e = bernsteinToPower(values, b)
		if e != nil {
			return r, e
		}
	}
	for _, interval := range proof.Intervals {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		patch, e := trimPatchBudget(p, interval.Cells[0], b)
		if e != nil {
			return r, e
		}
		power, e := symbolicSurfaceCurvePower(ctx, patch, coordinates, b)
		if e != nil {
			return r, e
		}
		cell := &ExactComposedCurveCell{power: power, cell: interval.Cells[0]}
		for side, index := range interval.Events {
			source := events[index]
			root := &ExactGraphLineRoot{bounds: [2]*big.Rat{new(big.Rat).Set(source.bounds[0]), new(big.Rat).Set(source.bounds[1])}, polynomial: make(rationalPolynomial, len(source.polynomial))}
			for i, x := range source.polynomial {
				root.polynomial[i] = new(big.Rat).Set(x)
			}
			cell.endpoints[side] = root
		}
		r.Cells = append(r.Cells, cell)
	}
	r.Events = proof.Events
	r.Complete = true
	return r, nil
}
