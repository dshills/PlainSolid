package geom

import (
	"context"
	"math/big"
)

// BoreBoundaryInterval is a closed original-parameter interval of continuous
// boundary contact. Boundary identifies the support along its relative interior;
// endpoint masks can additionally include rim/floor supports. Immutable roots
// and caller-owned bounds are independent of subsequent query calls.
type BoreBoundaryInterval struct {
	Start, End                           *ExactAlgebraicRoot
	StartBounds, EndBounds               ExactParameterInterval
	Boundary, StartBoundary, EndBoundary BoreBoundaryMask
}
type BoreLineBoundary struct {
	Hits      []BoreLineHit
	Intervals []BoreBoundaryInterval
}

// IntersectLineBoundary extends isolated-line queries with floor/cylinder
// coincidence. It partitions the finite range at every exact support root,
// proves signs on rational interior witnesses, and merges adjacent intervals
// of the same boundary support. Overlap endpoints are not duplicated in Hits.
// The input tolerance bounds isolated root brackets, not material classification.
// Arithmetic/root-isolation work is bounded; individual algebraic comparisons
// and refinements each receive the supplied options (at most seven endpoints).
func (s OffAxisSphereBore) IntersectLineBoundary(ctx context.Context, origin, direction Vec3, minT, maxT float64, o ToroidLineOptions) (BoreLineBoundary, error) {
	fail := func(e error) (BoreLineBoundary, error) { return BoreLineBoundary{}, e }
	if ctx == nil || s.R == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (Vec3{}) || !finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 {
		return fail(ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if o.MaxSubdivisions == 0 {
		o.MaxSubdivisions = 1024
	}
	if o.MaxSubdivisions < 1 || o.MaxSubdivisions > 65536 {
		return fail(ErrInvalid)
	}
	opts, e := patchOptions(o.Surface)
	if e != nil {
		return fail(e)
	}
	budget := &polynomialBudget{maxOperations: opts.MaxOperations, maxBits: opts.MaxCoefficientBits}
	ro := ExactCurvePlaneEventOptions{MaxOperations: opts.MaxOperations, MaxCoefficientBits: opts.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	var polynomials [3]rationalPolynomial
	for i := 0; i < 2; i++ {
		polynomials[i] = rationalPolynomial{new(big.Rat), new(big.Rat), new(big.Rat)}
	}
	values, derivatives := [3]float64{origin.X, origin.Y, origin.Z}, [3]float64{direction.X, direction.Y, direction.Z}
	for axis := 0; axis < 3; axis++ {
		linear := rationalPolynomial{q(values[axis]), q(derivatives[axis])}
		square, e := polynomialProduct(linear, linear, budget)
		if e != nil {
			return fail(e)
		}
		for j, v := range square {
			polynomials[0][j].Add(polynomials[0][j], v)
		}
	}
	polynomials[0][0].Sub(polynomials[0][0], new(big.Rat).Mul(s.R, s.R))
	for axis, center := range []*big.Rat{s.cx, s.cy} {
		linear := rationalPolynomial{new(big.Rat).Sub(q(values[axis]), center), q(derivatives[axis])}
		square, e := polynomialProduct(linear, linear, budget)
		if e != nil {
			return fail(e)
		}
		for j, v := range square {
			polynomials[1][j].Add(polynomials[1][j], v)
		}
	}
	polynomials[1][0].Sub(polynomials[1][0], new(big.Rat).Mul(s.r, s.r))
	polynomials[2] = rationalPolynomial{big.NewRat(1, 1)}
	if s.floor != nil {
		polynomials[2] = rationalPolynomial{new(big.Rat).Sub(q(origin.Z), s.floor), q(direction.Z)}
	}
	for i := range polynomials {
		polynomials[i] = trimPolynomial(polynomials[i])
		for _, x := range polynomials[i] {
			if e := budget.check(x); e != nil {
				return fail(e)
			}
		}
	}
	if len(polynomials[1]) > 0 && len(polynomials[2]) > 0 {
		hits, e := s.IntersectLine(ctx, origin, direction, minT, maxT, o)
		if e != nil {
			return fail(e)
		}
		return BoreLineBoundary{Hits: hits}, nil
	}
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
	for _, v := range []float64{minT, maxT} {
		root, _, e := NewExactRationalRoot(ctx, q(v), ro)
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
		_, e := isolatePolynomialRoots(p, q(minT), q(maxT), budget, o.MaxSubdivisions, func(lo, hi *big.Rat, m int) (bool, error) {
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
		var signs [3]int
		for j, p := range polynomials {
			if len(p) == 0 {
				continue
			}
			signs[j], _, e = composedPolynomialRootSign(ctx, events[i].root.root, p, budget, o.MaxSubdivisions)
			if e != nil {
				return fail(e)
			}
		}
		events[i].location, events[i].mask = boreLocation(signs[0], signs[1], signs[2])
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
		var signs [3]int
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
		loc, mask := boreLocation(signs[0], signs[1], signs[2])
		if loc != BoreBoundary {
			continue
		}
		if left.location != BoreBoundary || right.location != BoreBoundary {
			return fail(ErrConvergence)
		}
		previousCovered := covered[i]
		covered[i], covered[i+1] = true, true
		if previousCovered && len(out.Intervals) > 0 && out.Intervals[len(out.Intervals)-1].Boundary == mask {
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
