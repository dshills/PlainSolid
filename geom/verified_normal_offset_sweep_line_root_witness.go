package geom

import (
	"context"
	"errors"
	"math/big"
)

func (w *sweepLineWork) acceptRulingRoot(ctx context.Context, axis int, ruling [3]*big.Rat,
	root *ExactAlgebraicRoot, p sweepLineRulingPolynomials, tSign int, exactT *big.Rat) (*NormalOffsetSweepLineSpan, error) {
	profileAxis := 1 - axis
	normalized, profile, exactProfile, err := w.normalFootProfileBounds(ctx, root, p, profileAxis)
	if err != nil {
		return nil, err
	}
	if exactProfile != nil {
		if b := w.optionalRulingBudget(); b != nil {
			query := *w
			query.budget = b
			span, err := query.tryRulingStation(ctx, axis, ruling, exactProfile)
			w.budget.operations += b.operations
			if span != nil || (err != nil && !errors.Is(err, ErrConvergence)) {
				return span, err
			}
		}
	}
	shift, err := w.encloseRulingShift(p, normalized, tSign, exactT)
	if err != nil {
		return nil, err
	}
	certificate := w.normalFootCertificate(p, profileAxis, normalized, profile, shift)
	return w.algebraicRulingSpan(axis, p.along, p.rate, exactProfile, exactT, certificate)
}

func (w *sweepLineWork) normalFootProfileBounds(ctx context.Context, root *ExactAlgebraicRoot, p sweepLineRulingPolynomials,
	profileAxis int) (normalized, profile exactInterval, exactProfile *big.Rat, err error) {
	normalized, err = w.encloseRulingRoot(ctx, root)
	if err != nil {
		return normalized, profile, nil, err
	}
	original := w.original[profileAxis]
	a := boundedRatArithmetic{b: w.budget}
	width := a.sub(original.hi, original.lo)
	raw := func(s *big.Rat) *big.Rat { return a.add(original.lo, a.mul(width, s)) }
	profile = exactInterval{raw(normalized.lo), raw(normalized.hi)}
	if a.err != nil {
		return normalized, profile, nil, a.err
	}
	var known *big.Rat
	if normalized.lo.Cmp(normalized.hi) == 0 {
		known = normalized.lo
	} else if b := w.optionalRulingBudget(); b != nil {
		known, err = recoverRationalPolynomialRoot(p.normalFoot, normalized.lo, normalized.hi, b)
		w.budget.operations += b.operations
		if err != nil && !errors.Is(err, ErrConvergence) {
			return normalized, profile, nil, err
		}
	}
	if known != nil {
		exactProfile = raw(known)
		if a.err != nil {
			return normalized, profile, nil, a.err
		}
		profile = exactInterval{exactProfile, exactProfile}
	}
	return normalized, profile, exactProfile, nil
}

func (w *sweepLineWork) normalFootCertificate(p sweepLineRulingPolynomials, profileAxis int, normalized, profile, shift exactInterval) *NormalOffsetSweepLineAlgebraicStation {
	recipe := ExactRootRecipe{
		Version: 1, Coefficients: make([]string, len(p.normalFoot)),
		Lower: normalized.lo.RatString(), Upper: normalized.hi.RatString(),
	}
	for i, q := range p.normalFoot {
		recipe.Coefficients[i] = q.RatString()
	}
	return &NormalOffsetSweepLineAlgebraicStation{
		ProfileAxis: profileAxis, OriginalProfile: copySweepInterval(w.original[profileAxis]), ProfileRoot: recipe,
		ProfileParameter: copySweepInterval(profile), SignedDisplacement: copySweepInterval(shift),
	}
}

func (w *sweepLineWork) optionalRulingBudget() *polynomialBudget {
	// Preserve work for exact profile conversion and rational model
	// endpoints even if the optional tighter enclosures exhaust work.
	remaining := w.budget.maxOperations - w.budget.operations - 64
	if remaining <= 0 {
		return nil
	}
	return &polynomialBudget{maxOperations: remaining, maxBits: w.budget.maxBits}
}

func (w *sweepLineWork) encloseRulingRoot(ctx context.Context, root *ExactAlgebraicRoot) (exactInterval, error) {
	raw := root.Interval()
	if b := w.optionalRulingBudget(); b != nil {
		narrowed, err := root.Enclose(ctx, big.NewRat(1, 1<<46),
			ExactCurvePlaneEventOptions{MaxOperations: b.maxOperations, MaxCoefficientBits: b.maxBits})
		w.budget.operations += narrowed.Operations
		if err != nil && !errors.Is(err, ErrConvergence) {
			return exactInterval{}, err
		}
		if narrowed.HasBounds {
			raw = narrowed.Interval
		}
	}
	return exactInterval{raw.Lower, raw.Upper}, nil
}

func (w *sweepLineWork) encloseRulingShift(p sweepLineRulingPolynomials, root exactInterval, sign int,
	exactT *big.Rat) (exactInterval, error) {
	if exactT != nil {
		return exactInterval{exactT, exactT}, nil
	}
	if b := w.optionalRulingBudget(); b != nil {
		query := *w
		query.budget = b
		shift, err := query.rulingShiftBounds(p, root, sign)
		w.budget.operations += b.operations
		if err == nil || !errors.Is(err, ErrConvergence) {
			return shift, err
		}
	}
	// Inclusion in the original displacement interval and its exact
	// sign were already proved by polynomial signs at the isolated root.
	t := w.original[2]
	if sign > 0 && t.lo.Sign() < 0 {
		t.lo = new(big.Rat)
	}
	if sign < 0 && t.hi.Sign() > 0 {
		t.hi = new(big.Rat)
	}
	return t, nil
}

func (w *sweepLineWork) rulingShiftBounds(p sweepLineRulingPolynomials, root exactInterval, sign int) (exactInterval, error) {
	b := w.budget
	length2, err := polynomialBernsteinInterval(p.squared, root, b)
	if err != nil {
		return exactInterval{}, err
	}
	weight, err := polynomialBernsteinInterval(p.weight, root, b)
	if err != nil {
		return exactInterval{}, err
	}
	if weight.lo.Cmp(p.weightHull.lo) < 0 {
		weight.lo = p.weightHull.lo
	}
	if length2.lo.Sign() < 0 {
		length2.lo = new(big.Rat)
	}
	if length2.hi.Sign() < 0 || weight.lo.Cmp(weight.hi) > 0 {
		return exactInterval{}, ErrConvergence
	}
	lo, _, err := rationalSqrtBounds(length2.lo, 96, b)
	if err != nil {
		return exactInterval{}, err
	}
	_, hi, err := rationalSqrtBounds(length2.hi, 96, b)
	if err != nil {
		return exactInterval{}, err
	}
	t, err := intervalDivide(exactInterval{lo, hi}, weight, b)
	if err != nil {
		return exactInterval{}, err
	}
	if sign < 0 {
		t.lo, t.hi = new(big.Rat).Neg(t.hi), new(big.Rat).Neg(t.lo)
	}
	if t.lo.Cmp(w.original[2].lo) < 0 {
		t.lo = w.original[2].lo
	}
	if t.hi.Cmp(w.original[2].hi) > 0 {
		t.hi = w.original[2].hi
	}
	if t.lo.Cmp(t.hi) > 0 {
		return exactInterval{}, ErrConvergence
	}
	return t, nil
}

func (w *sweepLineWork) algebraicRulingSpan(axis int, at, rate, exactProfile, exactT *big.Rat,
	certificate *NormalOffsetSweepLineAlgebraicStation) (*NormalOffsetSweepLineSpan, error) {
	a := boundedRatArithmetic{b: w.budget}
	cuts := [2]*big.Rat{a.quo(a.sub(new(big.Rat), at), rate), a.quo(a.sub(big.NewRat(1, 1), at), rate)}
	axisStations := [2]*big.Rat{w.original[axis].lo, w.original[axis].hi}
	if rate.Sign() < 0 {
		cuts[0], cuts[1] = cuts[1], cuts[0]
		axisStations[0], axisStations[1] = axisStations[1], axisStations[0]
	}
	var ends [2]NormalOffsetSweepLineEndpoint
	for i, t := range cuts {
		end := &ends[i]
		end.Parameter = t
		raw := [3]*big.Rat{nil, nil, exactT}
		raw[axis], raw[1-axis] = axisStations[i], exactProfile
		for model, station := range raw {
			end.Point[model] = a.add(w.origin[model], a.mul(t, w.direction[model]))
			if station == nil {
				continue
			}
			end.Station[model] = new(big.Rat).Set(station)
			original := w.original[model]
			switch {
			case station.Cmp(original.lo) == 0:
				end.Faces = append(end.Faces, NormalOffsetSweepSide(model*2))
			case station.Cmp(original.hi) == 0:
				end.Faces = append(end.Faces, NormalOffsetSweepSide(model*2+1))
			}
		}
	}
	if a.err != nil {
		return nil, a.err
	}
	result := &NormalOffsetSweepLineSpan{Start: ends[0], End: ends[1], AlgebraicStation: certificate}
	for _, a := range result.Start.Faces {
		for _, b := range result.End.Faces {
			if a == b {
				result.BoundaryFaces = append(result.BoundaryFaces, a)
			}
		}
	}
	return result, nil
}
