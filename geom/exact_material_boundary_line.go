package geom

import (
	"context"
	"math/big"
)

// intersectMaterialBoundary partitions a closed finite range at every support
// root. The caller supplies exact polynomial predicates and their set semantics.
// No classification is inferred from approximate endpoint brackets.
func intersectMaterialBoundary(ctx context.Context, polynomials []rationalPolynomial, minT, maxT float64, o ToroidLineOptions, budget *polynomialBudget, ro ExactCurvePlaneEventOptions, classify func([]int) (BorePointLocation, BoreBoundaryMask)) (BoreLineBoundary, error) {
	return intersectMaterialBoundaryRational(ctx, polynomials, new(big.Rat).SetFloat64(minT), new(big.Rat).SetFloat64(maxT), o, budget, ro, classify)
}

func intersectMaterialBoundaryRational(ctx context.Context, polynomials []rationalPolynomial, minT, maxT *big.Rat, o ToroidLineOptions, budget *polynomialBudget, ro ExactCurvePlaneEventOptions, classify func([]int) (BorePointLocation, BoreBoundaryMask)) (BoreLineBoundary, error) {
	fail := func(e error) (BoreLineBoundary, error) { return BoreLineBoundary{}, e }
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	var e error
	type event struct {
		root         *ExactAlgebraicRoot
		mask         BoreBoundaryMask
		location     BorePointLocation
		multiplicity int
	}
	var events []event
	insert := func(root *ExactAlgebraicRoot, m int) error {
		at := len(events)
		for i, old := range events {
			cmp, e := root.Compare(ctx, old.root, ro)
			if e != nil {
				return e
			}
			if !cmp.Proven {
				return ErrConvergence
			}
			if cmp.Order == 0 {
				if m > events[i].multiplicity {
					events[i].multiplicity = m
				}
				return nil
			}
			if cmp.Order < 0 {
				at = i
				break
			}
		}
		events = append(events, event{})
		copy(events[at+1:], events[at:])
		events[at] = event{root: root, multiplicity: m}
		return nil
	}
	for _, v := range []*big.Rat{minT, maxT} {
		root, _, e := NewExactRationalRoot(ctx, v, ro)
		if e != nil {
			return fail(e)
		}
		if e := insert(root, 0); e != nil {
			return fail(e)
		}
	}
	for _, p := range polynomials {
		if len(p) < 2 {
			continue
		}
		coeff := make([]string, len(p))
		for i, x := range p {
			coeff[i] = x.RatString()
		}
		_, e := isolatePolynomialRoots(p, minT, maxT, budget, o.MaxSubdivisions, func(lo, hi *big.Rat, m int) (bool, error) {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if new(big.Rat).Sub(hi, lo).Cmp(q(o.ParameterTolerance)) > 0 {
				return false, nil
			}
			root, e := ImportExactRoot(ctx, ExactRootRecipe{Version: 1, Coefficients: coeff, Lower: lo.RatString(), Upper: hi.RatString()}, ExactRootCodecOptions{ExactCurvePlaneEventOptions: ro})
			if e != nil {
				return false, e
			}
			return true, insert(root, m)
		})
		if e != nil {
			return fail(e)
		}
	}
	for i := range events {
		signs := make([]int, len(polynomials))
		for j, p := range polynomials {
			if len(p) == 0 {
				continue
			}
			signs[j], _, e = composedPolynomialRootSign(ctx, events[i].root.root, p, budget, o.MaxSubdivisions)
			if e != nil {
				return fail(e)
			}
		}
		events[i].location, events[i].mask = classify(signs)
	}
	covered := make([]bool, len(events))
	var out BoreLineBoundary
	for i := 0; i+1 < len(events); i++ {
		left, right := events[i], events[i+1]
		a, b := left.root.Interval(), right.root.Interval()
		width := q(o.ParameterTolerance)
		proved := false
		for step := 0; step < o.MaxSubdivisions; step++ {
			if a.Upper.Cmp(b.Lower) < 0 {
				proved = true
				break
			}
			width.Quo(width, big.NewRat(2, 1))
			for j, root := range []*ExactAlgebraicRoot{left.root, right.root} {
				enc, e := root.Enclose(ctx, width, ro)
				if e != nil {
					return fail(e)
				}
				if !enc.Converged {
					return fail(ErrConvergence)
				}
				if j == 0 {
					a = enc.Interval
				} else {
					b = enc.Interval
				}
			}
		}
		if !proved {
			return fail(ErrConvergence)
		}
		mid := new(big.Rat).Quo(new(big.Rat).Add(a.Upper, b.Lower), big.NewRat(2, 1))
		signs := make([]int, len(polynomials))
		for j, p := range polynomials {
			value := new(big.Rat)
			for k := len(p) - 1; k >= 0; k-- {
				value.Mul(value, mid)
				value.Add(value, p[k])
				if e := budget.check(value); e != nil {
					return fail(e)
				}
			}
			signs[j] = value.Sign()
		}
		loc, mask := classify(signs)
		if loc != BoreBoundary {
			continue
		}
		if left.location != BoreBoundary || right.location != BoreBoundary {
			return fail(ErrConvergence)
		}
		previousCovered := covered[i]
		covered[i], covered[i+1] = true, true
		if previousCovered && left.mask == mask && len(out.Intervals) > 0 && out.Intervals[len(out.Intervals)-1].Boundary == mask {
			last := &out.Intervals[len(out.Intervals)-1]
			last.End = right.root
			last.EndBounds = b
			last.EndBoundary = right.mask
		} else {
			out.Intervals = append(out.Intervals, BoreBoundaryInterval{Start: left.root, End: right.root, StartBounds: a, EndBounds: b, Boundary: mask, StartBoundary: left.mask, EndBoundary: right.mask})
		}
	}
	for i, event := range events {
		if event.location == BoreBoundary && !covered[i] {
			m := event.multiplicity
			if m == 0 {
				m = 1
			}
			out.Hits = append(out.Hits, BoreLineHit{Parameter: event.root, Bounds: event.root.Interval(), Multiplicity: m, Boundary: event.mask})
		}
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	return out, nil
}
