package geom

import (
	"context"
	"math/big"
	"sort"
)

// ExactComposedPlaneEvent retains an immutable plane-contact parameter root.
// Its private cell supplies model evaluation. Parameter equality between events
// implies a shared model point only if the source curves are independently known
// to agree at that parameter (as in one continuous composition collection).
type ExactComposedPlaneEvent struct {
	root         *ExactGraphLineRoot
	cell         *ExactComposedCurveCell
	multiplicity int
	endpointMask uint8
}

func (e *ExactComposedPlaneEvent) Interval() ExactParameterInterval {
	if e == nil {
		return ExactParameterInterval{}
	}
	return e.root.Interval()
}
func (e *ExactComposedPlaneEvent) Multiplicity() int {
	if e == nil {
		return 0
	}
	return e.multiplicity
}

// EndpointMask is 1 for the exact start root, 2 for the exact end root, or zero.
func (e *ExactComposedPlaneEvent) EndpointMask() uint8 {
	if e == nil {
		return 0
	}
	return e.endpointMask
}

// CompareParameter proves equality/order in the common original parameter.
func (e *ExactComposedPlaneEvent) CompareParameter(ctx context.Context, other *ExactComposedPlaneEvent, o ExactCurvePlaneEventOptions) (ExactCurveEventOrder, error) {
	if e == nil || other == nil {
		return ExactCurveEventOrder{}, ErrInvalid
	}
	return CompareExactGraphLineRoots(ctx, e.root, other.root, o)
}

// Enclose bounds this exact contact point and its cell-side derivatives, without
// treating an algebraic parameter enclosure endpoint as the actual root.
func (e *ExactComposedPlaneEvent) Enclose(ctx context.Context, o ComposedCurveEndpointOptions) (AlgebraicCurveEndpointBounds, error) {
	if e == nil || e.cell == nil || e.root == nil {
		return AlgebraicCurveEndpointBounds{}, ErrInvalid
	}
	cell := *e.cell
	cell.endpoints[0] = e.root
	return cell.EncloseEndpoint(ctx, 0, o)
}

// CoordinateRoot proves a model coordinate of the original event, retaining
// its algebraic identity. It is limited to the same source polynomial degree
// as ExactComposedCurveCell.CoordinateRoot.
func (e *ExactComposedPlaneEvent) CoordinateRoot(ctx context.Context, axis int, o ExactCurvePlaneEventOptions) (*ExactAlgebraicRoot, int, error) {
	if e == nil || e.cell == nil || e.root == nil {
		return nil, 0, ErrInvalid
	}
	return e.cell.CoordinateRoot(ctx, e.Root(), axis, o)
}

// CoordinateRootWithWork also reports the root isolation subdivisions used
// to prove this event's coordinate, for callers with one shared budget.
func (e *ExactComposedPlaneEvent) CoordinateRootWithWork(ctx context.Context, axis int, o ExactCurvePlaneEventOptions) (*ExactAlgebraicRoot, int, int, error) {
	if e == nil || e.cell == nil || e.root == nil {
		return nil, 0, 0, ErrInvalid
	}
	return e.cell.CoordinateRootWithWork(ctx, e.Root(), axis, o)
}

// CompareModelPoint proves equality or separation of two original algebraic
// event images, independently of their parameterizations and root brackets.
// Coordinate images must satisfy the bounded CoordinateRoot domain.
func (e *ExactComposedPlaneEvent) CompareModelPoint(ctx context.Context, other *ExactComposedPlaneEvent,
	o ExactCurvePlaneEventOptions) (equal bool, operations int, err error) {
	if ctx == nil || e == nil || other == nil {
		return false, 0, ErrInvalid
	}
	o, err = graphRootOptions(o)
	if err != nil {
		return false, 0, err
	}
	for axis := 0; axis < 3; axis++ {
		if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
			return false, operations, ErrConvergence
		}
		a, n, steps, err := e.CoordinateRootWithWork(ctx, axis, o)
		operations += n
		o.MaxOperations -= n
		o.MaxSubdivisions -= steps
		if err != nil {
			return false, operations, err
		}
		if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
			return false, operations, ErrConvergence
		}
		b, n, steps, err := other.CoordinateRootWithWork(ctx, axis, o)
		operations += n
		o.MaxOperations -= n
		o.MaxSubdivisions -= steps
		if err != nil {
			return false, operations, err
		}
		if o.MaxOperations < 1 || o.MaxSubdivisions < 1 {
			return false, operations, ErrConvergence
		}
		q, err := a.Compare(ctx, b, o)
		operations += q.Operations
		o.MaxOperations -= q.Operations
		o.MaxSubdivisions -= q.Subdivisions
		if err != nil {
			return false, operations, err
		}
		if !q.Proven {
			return false, operations, ErrConvergence
		}
		if q.Order != 0 {
			return false, operations, nil
		}
	}
	return true, operations, nil
}

// ComposedCurvePlaneIntersection is exhaustive only when Complete. Coincident
// means the entire cell curve lies in the plane, and Boundary carries its two
// exact endpoint identities (multiplicity zero, not isolated plane roots).
// Isolated Events are ordered in the original global curve parameter.
type ComposedCurvePlaneIntersection struct {
	Complete, Coincident     bool
	Events                   []*ExactComposedPlaneEvent
	Boundary                 [2]*ExactComposedPlaneEvent
	Domain                   [2]ExactParameterInterval
	Operations, Subdivisions int
}

// IntersectRawPlane uses exact Nx*x+Ny*y+Nz*z+D=0 coefficients. It isolates the
// continuation's roots over the outer rational endpoint brackets, then proves
// every candidate inside/outside/equal to the actual algebraic domain endpoints.
// A bracket overlapping a boundary never suffices for inclusion. Returned exact
// event provenance supports joining repeated/shared knot contacts across cells.
func (c *ExactComposedCurveCell) IntersectRawPlane(ctx context.Context, plane [4]*big.Rat, o ExactCurvePlaneEventOptions) (r ComposedCurvePlaneIntersection, err error) {
	if ctx == nil || c == nil || len(c.power[3]) == 0 || c.endpoints[0] == nil || c.endpoints[1] == nil {
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
	defer func() { r.Operations = b.operations }()
	normal := false
	var polynomial rationalPolynomial
	for axis, x := range plane {
		if x == nil {
			x = new(big.Rat)
		}
		if e := b.check(x); e != nil {
			return r, e
		}
		normal = normal || axis < 3 && x.Sign() != 0
		for len(polynomial) < len(c.power[axis]) {
			polynomial = append(polynomial, new(big.Rat))
		}
		for i, value := range c.power[axis] {
			polynomial[i].Add(polynomial[i], new(big.Rat).Mul(x, value))
			if e := b.check(polynomial[i]); e != nil {
				return r, e
			}
		}
	}
	if !normal {
		return r, ErrInvalid
	}
	polynomial = trimPolynomial(polynomial)
	return c.intersectComposedPolynomial(ctx, polynomial, o, b)
}

func (c *ExactComposedCurveCell) intersectComposedPolynomial(ctx context.Context, polynomial rationalPolynomial, o ExactCurvePlaneEventOptions, b *polynomialBudget) (r ComposedCurvePlaneIntersection, err error) {
	defer func() { r.Operations = b.operations }()
	r.Domain = c.EndpointIntervals()
	cloneRoot := func(source *ExactGraphLineRoot) *ExactGraphLineRoot {
		copy := &ExactGraphLineRoot{bounds: [2]*big.Rat{new(big.Rat).Set(source.bounds[0]), new(big.Rat).Set(source.bounds[1])}, polynomial: make(rationalPolynomial, len(source.polynomial))}
		for i, x := range source.polynomial {
			copy.polynomial[i] = new(big.Rat).Set(x)
		}
		return copy
	}
	if len(polynomial) == 0 {
		for side, root := range c.endpoints {
			r.Boundary[side] = &ExactComposedPlaneEvent{root: cloneRoot(root), cell: c, endpointMask: 1 << uint(side)}
		}
		r.Complete = true
		r.Coincident = true
		return r, nil
	}

	var candidates []*ExactComposedPlaneEvent
	steps, e := isolatePolynomialRoots(polynomial, c.endpoints[0].bounds[0], c.endpoints[1].bounds[1], b, o.MaxSubdivisions, func(lo, hi *big.Rat, m int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		if lo.Cmp(hi) != 0 {
			for _, bound := range []*big.Rat{lo, hi} {
				value, e := polynomialValue(polynomial, bound, b)
				if e != nil {
					return false, e
				}
				if value.Sign() == 0 {
					return false, nil
				}
			}
		}
		root := &ExactGraphLineRoot{polynomial: polynomial, bounds: [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}}
		candidates = append(candidates, &ExactComposedPlaneEvent{root: root, cell: c, multiplicity: m})
		return true, nil
	})
	r.Subdivisions = steps
	if e != nil {
		return r, e
	}
	for _, event := range candidates {
		inside := true
		for side, endpoint := range c.endpoints {
			remaining := o.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			comparison, e := compareExactPolynomialEvents(ctx, [2]rationalPolynomial{event.root.polynomial, endpoint.polynomial}, [2][2]*big.Rat{event.root.bounds, endpoint.bounds}, b, remaining)
			r.Subdivisions += comparison.Subdivisions
			if e != nil {
				return r, e
			}
			if !comparison.Proven {
				return r, ErrConvergence
			}
			event.root.bounds = comparison.Intervals[0]
			if comparison.Order == 0 {
				event.endpointMask |= 1 << uint(side)
			}
			if side == 0 && comparison.Order < 0 || side == 1 && comparison.Order > 0 {
				inside = false
				break
			}
		}
		if inside {
			r.Events = append(r.Events, event)
		}
	}
	// Endpoint roots may be emitted before interior roots by isolation.
	// Sort rational enclosures, then independently prove every adjacent order.
	sort.SliceStable(r.Events, func(i, j int) bool { return r.Events[i].root.bounds[0].Cmp(r.Events[j].root.bounds[0]) < 0 })
	for i := 1; i < len(r.Events); i++ {
		left, right := r.Events[i-1], r.Events[i]
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		comparison, e := compareExactPolynomialEvents(ctx, [2]rationalPolynomial{left.root.polynomial, right.root.polynomial}, [2][2]*big.Rat{left.root.bounds, right.root.bounds}, b, remaining)
		r.Subdivisions += comparison.Subdivisions
		if e != nil {
			return r, e
		}
		if !comparison.Proven || comparison.Order != -1 {
			return r, ErrConvergence
		}
		left.root.bounds = comparison.Intervals[0]
		right.root.bounds = comparison.Intervals[1]
	}
	r.Complete = true
	return r, nil
}
