package geom

import (
	"context"
	"math/big"
)

// BoxDualCircleLineSide describes the open one-sided line germ at an event.
// Known=false means this side lies outside the requested closed parameter range.
// Boundary is a continuous line-on-boundary germ, not material interior.
type BoxDualCircleLineSide struct {
	Known    bool
	Location BorePointLocation
	Support  BoxDualCircleSupport
}

// BoxDualCircleTransition retains independent one-sided material evidence.
// It deliberately does not infer a crossing from multiplicity or a zero normal
// dot product. Inside/Outside is a crossing; equal non-boundary sides are a
// touch; any Boundary side is a continuous-contact incidence.
type BoxDualCircleTransition struct {
	Parameter                *ExactAlgebraicRoot
	Support                  BoxDualCircleSupport
	Before, After            BoxDualCircleLineSide
	AtRangeStart, AtRangeEnd bool
}

type BoxDualCircleBoundaryTransitions struct {
	Boundary BoxDualCircleBoundaryLine
	Events   []BoxDualCircleTransition // strictly ordered, exact duplicate roots merged
}

// IntersectBoxDualCircleBoundaryTransitionsRational augments the complete
// boundary report with exact polynomial germs at isolated contacts and span
// endpoints. For every constraint, the first nonzero derivative at the immutable
// algebraic root determines its two signs; identically zero constraints remain
// zero. No floating sample or bracket midpoint establishes a side classification.
// Work comprises the bounded boundary pass plus at most thirteen event insertions
// and three sign tests per quadratic constraint/event. Arithmetic limits apply
// to the additional germ work cumulatively; failure returns an empty report.
func IntersectBoxDualCircleBoundaryTransitionsRational(ctx context.Context, min, max [3]*big.Rat, xs, radii [2]*big.Rat, stopZ *big.Rat, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (BoxDualCircleBoundaryTransitions, error) {
	return intersectBoxDualCircleBoundaryTransitionsRational(ctx, min, max, xs, radii, min[1], stopZ, origin, direction, minT, maxT, o)
}

func intersectBoxDualCircleBoundaryTransitionsRational(ctx context.Context, min, max [3]*big.Rat, xs, radii [2]*big.Rat, centerY, stopZ *big.Rat, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o ToroidLineOptions) (BoxDualCircleBoundaryTransitions, error) {
	var zero BoxDualCircleBoundaryTransitions
	line, e := intersectBoxDualCircleBoundaryRational(ctx, min, max, xs, radii, centerY, stopZ, origin, direction, minT, maxT, o)
	if e != nil {
		return zero, e
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return zero, e
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	out := BoxDualCircleBoundaryTransitions{Boundary: line}
	add := func(root *ExactAlgebraicRoot, mask BoxDualCircleSupport) error {
		at := len(out.Events)
		for i, old := range out.Events {
			c, e := root.Compare(ctx, old.Parameter, ro)
			if e != nil {
				return e
			}
			if !c.Proven {
				return ErrConvergence
			}
			if c.Order == 0 {
				out.Events[i].Support |= mask
				return nil
			}
			if c.Order < 0 {
				at = i
				break
			}
		}
		out.Events = append(out.Events, BoxDualCircleTransition{})
		copy(out.Events[at+1:], out.Events[at:])
		out.Events[at] = BoxDualCircleTransition{Parameter: root, Support: mask}
		return nil
	}
	for _, h := range line.Hits {
		if e = add(h.Parameter, h.Support); e != nil {
			return zero, e
		}
	}
	for _, iv := range line.Intervals {
		if e = add(iv.Start, iv.StartSupport|iv.Support); e != nil {
			return zero, e
		}
		if e = add(iv.End, iv.EndSupport|iv.Support); e != nil {
			return zero, e
		}
	}
	polys := make([]rationalPolynomial, 9)
	for j := 0; j < 3; j++ {
		polys[j*2] = trimPolynomial(rationalPolynomial{new(big.Rat).Sub(min[j], origin[j]), new(big.Rat).Neg(direction[j])})
		polys[j*2+1] = trimPolynomial(rationalPolynomial{new(big.Rat).Sub(origin[j], max[j]), new(big.Rat).Set(direction[j])})
	}
	polys[8] = trimPolynomial(rationalPolynomial{new(big.Rat).Sub(origin[2], stopZ), new(big.Rat).Set(direction[2])})
	for i := 0; i < 2; i++ {
		p := rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Mul(radii[i], radii[i])), new(big.Rat), new(big.Rat)}
		for j, c := range [2]*big.Rat{xs[i], centerY} {
			t := rationalPolynomial{new(big.Rat).Sub(origin[j], c), direction[j]}
			prod, e := polynomialProduct(t, t, budget)
			if e != nil {
				return zero, e
			}
			for k, v := range prod {
				p[k].Add(p[k], v)
			}
		}
		polys[6+i] = trimPolynomial(p)
	}
	classify := func(s []int) BoxDualCircleLineSide {
		out := BoxDualCircleLineSide{Known: true, Location: BoreInside}
		for j := 0; j < 3; j++ {
			if s[2*j] > 0 || s[2*j+1] > 0 {
				out.Location = BoreOutside
				out.Support = 0
				return out
			}
			if s[2*j] == 0 || s[2*j+1] == 0 {
				out.Support |= BoxDualCircleStockX << j
			}
		}
		if s[8] <= 0 {
			u := s[6]
			if s[7] < u {
				u = s[7]
			}
			through := stopZ.Cmp(max[2]) == 0
			if u < 0 && (through || s[8] < 0) {
				out.Location = BoreOutside
				out.Support = 0
				return out
			}
			if u == 0 {
				if s[6] == 0 {
					out.Support |= BoxDualCircleFirstWall
				}
				if s[7] == 0 {
					out.Support |= BoxDualCircleSecondWall
				}
			}
			if !through && s[8] == 0 && u <= 0 {
				out.Support |= BoxDualCircleBlindFloor
			}
		}
		if out.Support != 0 {
			out.Location = BoreBoundary
		}
		return out
	}
	for i := range out.Events {
		root := out.Events[i].Parameter
		before, after := make([]int, 9), make([]int, 9)
		for j, p := range polys {
			for order := 0; len(p) > 0; order++ {
				sign, _, e := composedPolynomialRootSign(ctx, root.root, p, budget, o.MaxSubdivisions)
				if e != nil {
					return zero, e
				}
				if sign != 0 {
					after[j] = sign
					before[j] = sign
					if order%2 != 0 {
						before[j] = -sign
					}
					break
				}
				p = polynomialDerivative(p)
			}
		}
		lower, _, e := composedPolynomialRootSign(ctx, root.root, trimPolynomial(rationalPolynomial{new(big.Rat).Neg(minT), big.NewRat(1, 1)}), budget, o.MaxSubdivisions)
		if e != nil {
			return zero, e
		}
		upper, _, e := composedPolynomialRootSign(ctx, root.root, trimPolynomial(rationalPolynomial{new(big.Rat).Neg(maxT), big.NewRat(1, 1)}), budget, o.MaxSubdivisions)
		if e != nil {
			return zero, e
		}
		out.Events[i].AtRangeStart = lower == 0
		out.Events[i].AtRangeEnd = upper == 0
		if lower > 0 {
			out.Events[i].Before = classify(before)
		}
		if upper < 0 {
			out.Events[i].After = classify(after)
		}
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
