package geom

import (
	"context"
	"math/big"
)

type ComposedCurveLineOptions struct {
	ExactCurvePlaneEventOptions
	ParameterTolerance float64
}

// ComposedCurveLineHit retains an exact model contact event and a rational
// enclosure in line units origin+t*direction. BoundaryMask 1/2 proves equality
// with the lower/upper supplied cap; zero means strict interior in line t.
type ComposedCurveLineHit struct {
	Event         *ExactComposedPlaneEvent
	LineParameter ExactParameterInterval
	BoundaryMask  uint8
}

// ComposedCurveLinePortion is a closed coincident portion between two exact
// curve-parameter events. Every model point is on the supplied finite line.
// No monotonicity in line t is asserted; adjacent included portions may meet.
type ComposedCurveLinePortion struct{ Start, End *ExactComposedPlaneEvent }

// ComposedCurveLineIntersection keeps events ordered by original curve parameter.
// Coincident refers to the entire supporting line; only Portions/Hits satisfy
// the finite range. Complete is required for exhaustive coverage.
type ComposedCurveLineIntersection struct {
	Complete, Coincident     bool
	Hits                     []ComposedCurveLineHit
	Portions                 []ComposedCurveLinePortion
	Operations, Subdivisions int
}

// IntersectFiniteLine uses an unnormalized represented direction and exact
// rational closed parameter limits. Isolated contacts are filtered by exact
// algebraic sign tests. A line-coincident cell is partitioned by all cap roots;
// rational witnesses classify entire root-free intervals, with isolated cap
// touches retained. Zero-width line ranges are supported as point queries.
func (c *ExactComposedCurveCell) IntersectFiniteLine(ctx context.Context, origin, direction Vec3, limits [2]*big.Rat, o ComposedCurveLineOptions) (r ComposedCurveLineIntersection, err error) {
	if !origin.IsFinite() || !direction.IsFinite() {
		return r, ErrInvalid
	}
	origins, directions := [3]*big.Rat{}, [3]*big.Rat{}
	for i := 0; i < 3; i++ {
		origins[i] = new(big.Rat).SetFloat64(coordinate(origin, i))
		directions[i] = new(big.Rat).SetFloat64(coordinate(direction, i))
	}
	return c.IntersectRationalFiniteLine(ctx, origins, directions, limits, o)
}

// IntersectRationalFiniteLine is the exact-rational line counterpart of
// IntersectFiniteLine. Origin/direction coefficients are copied or read-only.
func (c *ExactComposedCurveCell) IntersectRationalFiniteLine(ctx context.Context, origins, directions [3]*big.Rat, limits [2]*big.Rat, o ComposedCurveLineOptions) (r ComposedCurveLineIntersection, err error) {
	for i := 0; i < 3; i++ {
		if origins[i] == nil || directions[i] == nil {
			return r, ErrInvalid
		}
	}

	if ctx == nil || c == nil || limits[0] == nil || limits[1] == nil || limits[0].Cmp(limits[1]) > 0 {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := graphRootOptions(o.ExactCurvePlaneEventOptions)
	if e != nil {
		return r, e
	}
	if o.ParameterTolerance == 0 {
		o.ParameterTolerance = 1e-9
	}
	if !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return r, ErrInvalid
	}

	pivot := 0
	for i := 1; i < 3; i++ {
		if new(big.Rat).Abs(directions[i]).Cmp(new(big.Rat).Abs(directions[pivot])) > 0 {
			pivot = i
		}
	}
	if directions[pivot].Sign() == 0 {
		return r, ErrDegenerate
	}
	b := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.Operations = b.operations }()
	for _, x := range limits {
		if e := b.check(x); e != nil {
			return r, e
		}
	}
	for i := 0; i < 3; i++ {
		for _, q := range []*big.Rat{origins[i], directions[i]} {
			if e := b.check(q); e != nil {
				return r, e
			}
		}
	}
	var planes [2][4]*big.Rat
	dp, op := directions[pivot], origins[pivot]
	other := 0
	for axis := 0; axis < 3; axis++ {
		if axis == pivot {
			continue
		}
		da, oa := directions[axis], origins[axis]
		for k := range planes[other] {
			planes[other][k] = new(big.Rat)
		}
		planes[other][pivot].Neg(da)
		planes[other][axis].Set(dp)
		planes[other][3].Sub(new(big.Rat).Mul(da, op), new(big.Rat).Mul(dp, oa))
		other++
	}
	childOptions := func() (ExactCurvePlaneEventOptions, error) {
		child := options
		child.MaxOperations -= b.operations
		child.MaxSubdivisions -= r.Subdivisions
		if child.MaxOperations < 1 || child.MaxSubdivisions < 1 {
			return child, ErrConvergence
		}
		return child, nil
	}
	child, e := childOptions()
	if e != nil {
		return r, e
	}
	line, e := c.IntersectRawPlanePair(ctx, planes, child)
	b.operations += line.Operations
	r.Subdivisions += line.Subdivisions
	if e != nil {
		return r, e
	}
	if !line.Complete {
		return r, ErrConvergence
	}
	r.Coincident = line.Coincident
	var capPolynomials [2]rationalPolynomial
	var capPlanes [2][4]*big.Rat
	for side, limit := range limits {
		level := new(big.Rat).Add(op, new(big.Rat).Mul(dp, limit))
		capPolynomials[side], e = subtractScaledPolynomial(c.power[pivot], c.power[3], level, b)
		if e != nil {
			return r, e
		}
		capPlanes[side][pivot] = big.NewRat(1, 1)
		capPlanes[side][3] = new(big.Rat).Neg(level)
	}
	classify := func(event *ExactComposedPlaneEvent) (bool, uint8, error) {
		mask := uint8(0)
		for side, polynomial := range capPolynomials {
			remaining := options.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return false, 0, ErrConvergence
			}
			sign, steps, e := composedPolynomialRootSign(ctx, event.root, polynomial, b, remaining)
			r.Subdivisions += steps
			if e != nil {
				return false, 0, e
			}
			sign *= dp.Sign()
			if sign == 0 {
				mask |= 1 << uint(side)
			}
			if side == 0 && sign < 0 || side == 1 && sign > 0 {
				return false, 0, nil
			}
		}
		return true, mask, nil
	}
	boundHit := func(event *ExactComposedPlaneEvent, mask uint8) (ComposedCurveLineHit, error) {
		hit := ComposedCurveLineHit{Event: event, BoundaryMask: mask}
		if mask != 0 {
			side := 0
			if mask&1 == 0 {
				side = 1
			}
			hit.LineParameter = ExactParameterInterval{Lower: new(big.Rat).Set(limits[side]), Upper: new(big.Rat).Set(limits[side])}
			return hit, nil
		}
		numerator, e := subtractScaledPolynomial(c.power[pivot], c.power[3], op, b)
		if e != nil {
			return hit, e
		}
		denominator := make(rationalPolynomial, len(c.power[3]))
		for i, x := range c.power[3] {
			denominator[i] = new(big.Rat).Mul(x, dp)
		}
		remaining := options.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return hit, ErrConvergence
		}
		tolerance := new(big.Rat).SetFloat64(o.ParameterTolerance)
		found := false
		steps, e := isolatePolynomialRoots(event.root.polynomial, event.root.bounds[0], event.root.bounds[1], b, remaining, func(lo, hi *big.Rat, _ int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			parameter := exactInterval{lo: lo, hi: hi}
			n, e := polynomialInterval(numerator, parameter, b)
			if e != nil {
				return false, e
			}
			w, e := polynomialInterval(denominator, parameter, b)
			if e != nil {
				return false, e
			}
			if w.lo.Sign() <= 0 && w.hi.Sign() >= 0 {
				return false, nil
			}
			value, e := intervalDivide(n, w, b)
			if e != nil {
				return false, e
			}
			if value.lo.Cmp(limits[0]) < 0 {
				value.lo = new(big.Rat).Set(limits[0])
			}
			if value.hi.Cmp(limits[1]) > 0 {
				value.hi = new(big.Rat).Set(limits[1])
			}
			if new(big.Rat).Sub(value.hi, value.lo).Cmp(tolerance) > 0 {
				return false, nil
			}
			hit.LineParameter = ExactParameterInterval{Lower: new(big.Rat).Set(value.lo), Upper: new(big.Rat).Set(value.hi)}
			found = true
			return true, nil
		})
		r.Subdivisions += steps
		if e != nil {
			return hit, e
		}
		if !found {
			return hit, ErrConvergence
		}
		return hit, nil
	}
	if !line.Coincident {
		for _, event := range line.Events {
			inside, mask, e := classify(event)
			if e != nil {
				return r, e
			}
			if inside {
				hit, e := boundHit(event, mask)
				if e != nil {
					return r, e
				}
				r.Hits = append(r.Hits, hit)
			}
		}
		r.Complete = true
		return r, nil
	}
	events := []*ExactComposedPlaneEvent{line.Boundary[0], line.Boundary[1]}
	add := func(event *ExactComposedPlaneEvent) error {
		for i := len(events) - 1; i >= 0; i-- {
			remaining := options.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return ErrConvergence
			}
			comparison, e := compareExactPolynomialEvents(ctx, [2]rationalPolynomial{event.root.polynomial, events[i].root.polynomial}, [2][2]*big.Rat{event.root.bounds, events[i].root.bounds}, b, remaining)
			r.Subdivisions += comparison.Subdivisions
			if e != nil {
				return e
			}
			if !comparison.Proven {
				return ErrConvergence
			}
			event.root.bounds = comparison.Intervals[0]
			events[i].root.bounds = comparison.Intervals[1]
			if comparison.Order == 0 {
				return nil
			}
			if comparison.Order > 0 {
				events = append(events, nil)
				copy(events[i+2:], events[i+1:])
				events[i+1] = event
				return nil
			}
		}
		events = append(events, nil)
		copy(events[1:], events[:len(events)-1])
		events[0] = event
		return nil
	}
	for _, plane := range capPlanes {
		child, e := childOptions()
		if e != nil {
			return r, e
		}
		caps, e := c.IntersectRawPlane(ctx, plane, child)
		b.operations += caps.Operations
		r.Subdivisions += caps.Subdivisions
		if e != nil {
			return r, e
		}
		if !caps.Complete {
			return r, ErrConvergence
		}
		for _, event := range caps.Events {
			if e := add(event); e != nil {
				return r, e
			}
		}
	}
	covered := make([]bool, len(events))
	for i := 0; i < len(events)-1; i++ {
		left, right := events[i].root.bounds[1], events[i+1].root.bounds[0]
		if left.Cmp(right) >= 0 {
			return r, ErrConvergence
		}
		sample := new(big.Rat).Quo(new(big.Rat).Add(left, right), big.NewRat(2, 1))
		n, e := polynomialValue(c.power[pivot], sample, b)
		if e != nil {
			return r, e
		}
		w, e := polynomialValue(c.power[3], sample, b)
		if e != nil {
			return r, e
		}
		if w.Sign() <= 0 {
			return r, ErrConvergence
		}
		parameter := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Quo(n, w), op), dp)
		if e := b.check(parameter); e != nil {
			return r, e
		}
		if parameter.Cmp(limits[0]) >= 0 && parameter.Cmp(limits[1]) <= 0 {
			r.Portions = append(r.Portions, ComposedCurveLinePortion{Start: events[i], End: events[i+1]})
			covered[i] = true
			covered[i+1] = true
		}
	}
	for i, event := range events {
		if covered[i] {
			continue
		}
		inside, mask, e := classify(event)
		if e != nil {
			return r, e
		}
		if inside {
			hit, e := boundHit(event, mask)
			if e != nil {
				return r, e
			}
			r.Hits = append(r.Hits, hit)
		}
	}
	r.Complete = true
	return r, nil
}

func composedPolynomialRootSign(ctx context.Context, root *ExactGraphLineRoot, polynomial rationalPolynomial, b *polynomialBudget, maxSubdivisions int) (sign, subdivisions int, err error) {
	if len(trimPolynomial(polynomial)) == 0 {
		return 0, 0, nil
	}
	gcd, e := polynomialGCD(root.polynomial, polynomial, b)
	if e != nil {
		return 0, 0, e
	}
	count := 0
	steps, e := isolatePolynomialRoots(gcd, root.bounds[0], root.bounds[1], b, maxSubdivisions, func(_, _ *big.Rat, _ int) (bool, error) { count++; return true, ctx.Err() })
	subdivisions += steps
	if e != nil {
		return 0, subdivisions, e
	}
	if count == 1 {
		return 0, subdivisions, nil
	}
	remaining := maxSubdivisions - subdivisions
	if remaining < 1 {
		return 0, subdivisions, ErrConvergence
	}
	steps, e = isolatePolynomialRoots(root.polynomial, root.bounds[0], root.bounds[1], b, remaining, func(lo, hi *big.Rat, _ int) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		value, e := polynomialInterval(polynomial, exactInterval{lo: lo, hi: hi}, b)
		if e != nil {
			return false, e
		}
		if value.lo.Sign() > 0 {
			sign = 1
			return true, nil
		}
		if value.hi.Sign() < 0 {
			sign = -1
			return true, nil
		}
		return false, nil
	})
	subdivisions += steps
	if e != nil {
		return 0, subdivisions, e
	}
	if sign == 0 {
		return 0, subdivisions, ErrConvergence
	}
	return sign, subdivisions, nil
}
