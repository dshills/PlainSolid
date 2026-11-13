package geom

import (
	"context"
	"math/big"
)

// clipComposedIdentityGraph uses exact scalar knot roots to cover the portions
// of an already positive-denominator curve whose projection lies in the graph.
func clipComposedIdentityGraph(ctx context.Context, source *ExactComposedCurveCell, target *SurfacePatch, o SurfaceCurveAgreementOptions, b *polynomialBudget) (r IdentityGraphCurveIntersection, err error) {
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxEvents < 2 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	events := []*ExactGraphLineRoot{copyExactRoot(source.endpoints[0]), copyExactRoot(source.endpoints[1])}
	compare := func(a, c *ExactGraphLineRoot) (int, error) {
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return 0, ErrConvergence
		}
		q, e := compareRootBudget(ctx, a, c, b, remaining)
		r.Subdivisions += q.Subdivisions
		if e != nil {
			return 0, e
		}
		if !q.Proven {
			return 0, ErrConvergence
		}
		a.bounds = q.Intervals[0]
		c.bounds = q.Intervals[1]
		return q.Order, nil
	}
	add := func(root *ExactGraphLineRoot) error {
		for i := len(events) - 1; i >= 0; i-- {
			order, e := compare(root, events[i])
			if e != nil {
				return e
			}
			if order == 0 {
				return nil
			}
			if order > 0 {
				if len(events) >= o.MaxEvents {
					return ErrConvergence
				}
				events = append(events, nil)
				copy(events[i+2:], events[i+1:])
				events[i+1] = root
				return nil
			}
		}
		return ErrDomain
	}
	for axis, knots := range [][]float64{target.knotsU, target.knotsV} {
		previous := 0.
		havePrevious := false
		for _, knot := range knots {
			if havePrevious && knot == previous {
				continue
			}
			previous = knot
			havePrevious = true
			polynomial, e := subtractScaledPolynomial(source.power[axis], source.power[3], new(big.Rat).SetFloat64(knot), b)
			if e != nil {
				return r, e
			}
			if len(polynomial) == 0 {
				continue
			}
			remaining := o.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			hits, e := source.intersectComposedPolynomial(ctx, polynomial, ExactCurvePlaneEventOptions{MaxOperations: b.maxOperations, MaxCoefficientBits: b.maxBits, MaxSubdivisions: remaining}, b)
			r.Subdivisions += hits.Subdivisions
			if e != nil {
				return r, e
			}
			for _, event := range hits.Events {
				if e := add(copyExactRoot(event.root)); e != nil {
					return r, e
				}
			}
		}
	}
	// Refine adjacent identities to obtain a strictly interior rational witness.
	for i := 0; i+1 < len(events); i++ {
		order, e := compare(events[i], events[i+1])
		if e != nil {
			return r, e
		}
		if order != -1 {
			return r, ErrConvergence
		}
	}
	coordinates := [3]rationalPolynomial{source.power[0], source.power[1], source.power[3]}
	selectCell := func(sign func(int, float64) (int, error)) (ParameterRectangle, bool, error) {
		var ranges [2][2]float64
		for axis, knots := range [][]float64{target.knotsU, target.knotsV} {
			found := false
			for i := 0; i+1 < len(knots); i++ {
				lo, hi := knots[i], knots[i+1]
				if lo >= hi {
					continue
				}
				lower, e := sign(axis, lo)
				if e != nil {
					return ParameterRectangle{}, false, e
				}
				upper, e := sign(axis, hi)
				if e != nil {
					return ParameterRectangle{}, false, e
				}
				if lower >= 0 && upper <= 0 {
					ranges[axis] = [2]float64{lo, hi}
					found = true
					break
				}
			}
			if !found {
				return ParameterRectangle{}, false, nil
			}
		}
		return ParameterRectangle{UMin: ranges[0][0], UMax: ranges[0][1], VMin: ranges[1][0], VMax: ranges[1][1]}, true, nil
	}
	residual := func(domain ParameterRectangle) (rationalPolynomial, error) {
		patch, e := trimPatchBudget(target, domain, b)
		if e != nil {
			return nil, e
		}
		model, e := symbolicSurfaceCurvePower(ctx, patch, coordinates, b)
		if e != nil {
			return nil, e
		}
		left, e := polynomialProduct(source.power[2], model[3], b)
		if e != nil {
			return nil, e
		}
		right, e := polynomialProduct(model[2], source.power[3], b)
		if e != nil {
			return nil, e
		}
		return subtractScaledPolynomial(left, right, big.NewRat(1, 1), b)
	}
	incident := make([]bool, len(events))
	orders := []int{}
	for i := 0; i+1 < len(events); i++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if events[i].bounds[1].Cmp(events[i+1].bounds[0]) >= 0 {
			return r, ErrConvergence
		}
		witness := new(big.Rat).Quo(new(big.Rat).Add(events[i].bounds[1], events[i+1].bounds[0]), big.NewRat(2, 1))
		var point [2]*big.Rat
		weight, e := polynomialValue(source.power[3], witness, b)
		if e != nil {
			return r, e
		}
		if weight.Sign() <= 0 {
			return r, ErrConvergence
		}
		for axis := 0; axis < 2; axis++ {
			value, e := polynomialValue(source.power[axis], witness, b)
			if e != nil {
				return r, e
			}
			point[axis] = new(big.Rat).Quo(value, weight)
		}
		domain, inside, e := selectCell(func(axis int, value float64) (int, error) {
			return point[axis].Cmp(new(big.Rat).SetFloat64(value)), nil
		})
		if e != nil {
			return r, e
		}
		if !inside {
			continue
		}
		polynomial, e := residual(domain)
		if e != nil {
			return r, e
		}
		model := *source
		model.endpoints = [2]*ExactGraphLineRoot{events[i], events[i+1]}
		model.cell = domain
		remaining := o.MaxSubdivisions - r.Subdivisions
		if remaining < 1 {
			return r, ErrConvergence
		}
		contacts, e := model.intersectComposedPolynomial(ctx, polynomial, ExactCurvePlaneEventOptions{MaxOperations: b.maxOperations, MaxCoefficientBits: b.maxBits, MaxSubdivisions: remaining}, b)
		r.Subdivisions += contacts.Subdivisions
		if e != nil {
			return r, e
		}
		report := IdentityGraphCurveCellContact{SurfaceCell: domain, Intersection: contacts}
		for _, event := range contacts.Events {
			derivative := polynomial
			for j := 0; j < event.Multiplicity(); j++ {
				derivative = polynomialDerivative(derivative)
			}
			remaining := o.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			sign, steps, e := composedPolynomialRootSign(ctx, event.root, derivative, b, remaining)
			r.Subdivisions += steps
			if e != nil {
				return r, e
			}
			if sign == 0 {
				return r, ErrConvergence
			}
			sides := [2]int{sign, sign}
			if event.Multiplicity()%2 == 1 {
				sides[0] = -sign
			}
			if event.EndpointMask()&1 != 0 {
				sides[0] = 0
			}
			if event.EndpointMask()&2 != 0 {
				sides[1] = 0
			}
			report.EventSigns = append(report.EventSigns, sides)
		}
		r.Cells = append(r.Cells, report)
		orders = append(orders, 2*i+1)
		incident[i], incident[i+1] = true, true
	}
	// An outside-to-outside tangent can touch a chart edge/corner at one isolated
	// parameter. Check such unclaimed events against the closed chart exactly.
	for i, root := range events {
		if incident[i] {
			continue
		}
		domain, inside, e := selectCell(func(axis int, value float64) (int, error) {
			p, e := subtractScaledPolynomial(source.power[axis], source.power[3], new(big.Rat).SetFloat64(value), b)
			if e != nil {
				return 0, e
			}
			if len(p) == 0 {
				return 0, nil
			}
			remaining := o.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return 0, ErrConvergence
			}
			sign, steps, e := composedPolynomialRootSign(ctx, root, p, b, remaining)
			r.Subdivisions += steps
			return sign, e
		})
		if e != nil {
			return r, e
		}
		if !inside {
			continue
		}
		polynomial, e := residual(domain)
		if e != nil {
			return r, e
		}
		if len(polynomial) > 0 {
			remaining := o.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return r, ErrConvergence
			}
			sign, steps, e := composedPolynomialRootSign(ctx, root, polynomial, b, remaining)
			r.Subdivisions += steps
			if e != nil {
				return r, e
			}
			if sign != 0 {
				continue
			}
		}
		model := *source
		model.endpoints = [2]*ExactGraphLineRoot{root, root}
		model.cell = domain
		event := &ExactComposedPlaneEvent{root: copyExactRoot(root), cell: &model, endpointMask: 3}
		contacts := ComposedCurvePlaneIntersection{Complete: true, Events: []*ExactComposedPlaneEvent{event}, Domain: [2]ExactParameterInterval{root.Interval(), root.Interval()}}
		r.Cells = append(r.Cells, IdentityGraphCurveCellContact{SurfaceCell: domain, Intersection: contacts, EventSigns: [][2]int{{0, 0}}})
		orders = append(orders, 2*i)
	}
	for i := 1; i < len(orders); i++ {
		for j := i; j > 0 && orders[j] < orders[j-1]; j-- {
			orders[j], orders[j-1] = orders[j-1], orders[j]
			r.Cells[j], r.Cells[j-1] = r.Cells[j-1], r.Cells[j]
		}
	}
	r.Complete = true
	return r, nil
}
