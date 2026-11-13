package geom

import (
	"context"
	"math/big"
)

// SurfaceCurveAgreementOptions bounds complete algebraic event enumeration and
// symbolic comparison. Defaults: 256 events, 8192 subdivisions, standard patch
// arithmetic limits. Both UV paths must be single Béziers on normalized [0,1].
type SurfaceCurveAgreementOptions struct {
	SurfacePatchOptions
	MaxEvents, MaxSubdivisions int
}

// SurfaceCurveAgreementInterval identifies an open interval between two exact
// event identities (indices into Events). Witness is strictly between their
// rational isolating brackets. Equal means full polynomial identity, not samples.
type SurfaceCurveAgreementInterval struct {
	Events  [2]int
	Witness *big.Rat
	Cells   [2]ParameterRectangle
	Equal   bool
}

// SurfaceCurveAlgebraicAgreement establishes same-parameter model identity over
// the whole closed domain when Complete&&Equal. Complete&&!Equal proves failure
// of functional identity, not pointwise disjointness. Algebraic event brackets
// are not exported as exact rational cuts. Source continuity extends the open
// interval identities to the event parameters, including both endpoints.
type SurfaceCurveAlgebraicAgreement struct {
	Complete, Equal               bool
	Events                        []ExactParameterInterval
	Intervals                     []SurfaceCurveAgreementInterval
	ExactOperations, Subdivisions int
}

// CompareSurfaceCurveCompositions orders the union of all knot-level events
// independently from both owners. It requires exact positive-weight C0 patches
// and UV curves with Z=0. Rational witnesses select whole cells only after root
// completeness proves that no knot event lies between them. Homogeneous
// polynomial cross-identities then prove equality on each entire open interval.
func CompareSurfaceCurveCompositions(ctx context.Context, a *SurfacePatch, uvA *ExactSplineCurve, c *SurfacePatch, uvC *ExactSplineCurve, o SurfaceCurveAgreementOptions) (SurfaceCurveAlgebraicAgreement, error) {
	return compareSurfaceCurveCompositions(ctx, a, uvA, c, uvC, o, nil)
}
func compareSurfaceCurveCompositions(ctx context.Context, a *SurfacePatch, uvA *ExactSplineCurve, c *SurfacePatch, uvC *ExactSplineCurve, o SurfaceCurveAgreementOptions, retained *[]*ExactGraphLineRoot) (r SurfaceCurveAlgebraicAgreement, err error) {
	if ctx == nil || a == nil || c == nil || len(a.net) == 0 || len(c.net) == 0 || uvA == nil || uvC == nil || uvA.patch == nil || uvC.patch == nil {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	options, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return r, e
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 8192
	}
	if o.MaxEvents < 2 || o.MaxSubdivisions < 1 {
		return r, ErrInvalid
	}
	budget := &polynomialBudget{maxOperations: options.MaxOperations, maxBits: options.MaxCoefficientBits}
	defer func() { r.ExactOperations = budget.operations }()
	surfaces := [2]*SurfacePatch{a, c}
	curves := [2]*ExactSplineCurve{uvA, uvC}
	var coordinates [2][3]rationalPolynomial
	events := []*ExactGraphLineRoot{{polynomial: rationalPolynomial{new(big.Rat), big.NewRat(1, 1)}, bounds: [2]*big.Rat{new(big.Rat), new(big.Rat)}}, {polynomial: rationalPolynomial{big.NewRat(-1, 1), big.NewRat(1, 1)}, bounds: [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1)}}}
	add := func(event *ExactGraphLineRoot) error {
		for i := len(events) - 1; i >= 0; i-- {
			remaining := o.MaxSubdivisions - r.Subdivisions
			if remaining < 1 {
				return ErrConvergence
			}
			comparison, e := compareExactPolynomialEvents(ctx, [2]rationalPolynomial{event.polynomial, events[i].polynomial}, [2][2]*big.Rat{event.bounds, events[i].bounds}, budget, remaining)
			r.Subdivisions += comparison.Subdivisions
			if e != nil {
				return e
			}
			if !comparison.Proven {
				return ErrConvergence
			}
			event.bounds = comparison.Intervals[0]
			events[i].bounds = comparison.Intervals[1]
			if comparison.Order == 0 {
				return nil
			}
			if comparison.Order > 0 {
				if len(events) >= o.MaxEvents {
					return ErrConvergence
				}
				events = append(events, nil)
				copy(events[i+2:], events[i+1:])
				events[i+1] = event
				return nil
			}
		}
		if len(events) >= o.MaxEvents {
			return ErrConvergence
		}
		events = append(events, nil)
		copy(events[1:], events[:len(events)-1])
		events[0] = event
		return nil
	}
	for owner, curve := range curves {
		p := curve.patch
		lo, hi := curve.Domain()
		if lo != 0 || hi != 1 || p.countU != p.degreeU+1 || p.degreeU > 64 || len(p.net) > options.MaxPoles || len(surfaces[owner].net) > options.MaxPoles {
			return r, ErrConvergence
		}
		for _, h := range p.net {
			if h[2].Sign() != 0 {
				return r, ErrInvalid
			}
		}
		for axis, component := range []int{0, 1, 3} {
			values := make([]*big.Rat, p.countU)
			for i := range values {
				values[i] = p.net[i*p.countV][component]
			}
			coordinates[owner][axis], e = bernsteinToPower(values, budget)
			if e != nil {
				return r, e
			}
		}
		for axis, knots := range [][]float64{surfaces[owner].knotsU, surfaces[owner].knotsV} {
			have := false
			previous := 0.
			for _, knot := range knots {
				if have && knot == previous {
					continue
				}
				have = true
				previous = knot
				polynomial, e := subtractScaledPolynomial(coordinates[owner][axis], coordinates[owner][2], new(big.Rat).SetFloat64(knot), budget)
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
				// Stage events before sorting, so nested isolation cannot spend the same
				// subdivision allowance twice within an outer isolation callback.
				var staged []*ExactGraphLineRoot
				steps, e := isolatePolynomialRoots(polynomial, new(big.Rat), big.NewRat(1, 1), budget, remaining, func(lo, hi *big.Rat, _ int) (bool, error) {
					if e := ctx.Err(); e != nil {
						return false, e
					}
					if lo.Cmp(hi) != 0 {
						for _, bound := range []*big.Rat{lo, hi} {
							value, e := polynomialValue(polynomial, bound, budget)
							if e != nil {
								return false, e
							}
							if value.Sign() == 0 {
								return false, nil
							}
						}
					}
					if len(staged) >= o.MaxEvents {
						return false, ErrConvergence
					}
					staged = append(staged, &ExactGraphLineRoot{polynomial: polynomial, bounds: [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}})
					return true, nil
				})
				r.Subdivisions += steps
				if e != nil {
					return r, e
				}
				for _, event := range staged {
					if e := add(event); e != nil {
						return r, e
					}
				}
			}
		}
	}
	if retained != nil {
		*retained = events
	}
	for _, event := range events {
		r.Events = append(r.Events, event.Interval())
	}
	equal := true
	for i := 0; i < len(events)-1; i++ {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if events[i].bounds[1].Cmp(events[i+1].bounds[0]) >= 0 {
			return r, ErrConvergence
		}
		witness := new(big.Rat).Quo(new(big.Rat).Add(events[i].bounds[1], events[i+1].bounds[0]), big.NewRat(2, 1))
		interval := SurfaceCurveAgreementInterval{Events: [2]int{i, i + 1}, Witness: witness, Equal: true}
		var model [2][4]rationalPolynomial
		for owner, curve := range curves {
			point, e := exactCurveRationalPoint(curve.patch, witness, budget)
			if e != nil {
				return r, e
			}
			surface := surfaces[owner]
			d := surface.Domain()
			ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
			var cellBounds [2][2]float64
			for axis, knots := range [][]float64{surface.knotsU, surface.knotsV} {
				found := false
				for j := 0; j < len(knots)-1; j++ {
					lo, hi := knots[j], knots[j+1]
					if lo >= hi {
						continue
					}
					if point[axis].Cmp(new(big.Rat).SetFloat64(lo)) >= 0 && (point[axis].Cmp(new(big.Rat).SetFloat64(hi)) < 0 || hi == ranges[axis][1] && point[axis].Cmp(new(big.Rat).SetFloat64(hi)) == 0) {
						cellBounds[axis] = [2]float64{lo, hi}
						found = true
						break
					}
				}
				if !found {
					return r, ErrDomain
				}
			}
			cell := ParameterRectangle{UMin: cellBounds[0][0], UMax: cellBounds[0][1], VMin: cellBounds[1][0], VMax: cellBounds[1][1]}
			interval.Cells[owner] = cell
			patch, e := trimPatchBudget(surface, cell, budget)
			if e != nil {
				return r, e
			}
			model[owner], e = symbolicSurfaceCurvePower(ctx, patch, coordinates[owner], budget)
			if e != nil {
				return r, e
			}
		}
		for axis := 0; axis < 3; axis++ {
			left, e := polynomialProduct(model[0][axis], model[1][3], budget)
			if e != nil {
				return r, e
			}
			right, e := polynomialProduct(model[1][axis], model[0][3], budget)
			if e != nil {
				return r, e
			}
			difference, e := subtractScaledPolynomial(left, right, big.NewRat(1, 1), budget)
			if e != nil {
				return r, e
			}
			if len(difference) != 0 {
				interval.Equal = false
				equal = false
				break
			}
		}
		r.Intervals = append(r.Intervals, interval)
	}
	r.Complete = true
	r.Equal = equal
	return r, nil
}
