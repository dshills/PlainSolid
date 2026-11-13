package geom

import "math/big"

// RemoveUKnot removes occurrences of a strictly interior U knot. The returned
// surface is accepted only after exact rational reinsertion proves a global
// Euclidean error bound <= options.Tolerance. Options/diagnostics share the
// curve operation's meaning; MaxPoles limits the entire tensor net, and work
// and exact-operation budgets aggregate all row solves and final verification.
// The source is immutable and failure returns no candidate.
func (s *BSplineSurface) RemoveUKnot(t float64, count int, o KnotRemovalOptions) (*BSplineSurface, KnotRemovalDiagnostics, error) {
	return s.removeSurfaceKnot(t, count, true, o)
}

// RemoveVKnot is RemoveUKnot in the V direction. Positive rational weights and
// clamping in both directions are required. The bound concerns ideal surfaces
// defined by stored floats; subsequent floating evaluation error is additional.
func (s *BSplineSurface) RemoveVKnot(t float64, count int, o KnotRemovalOptions) (*BSplineSurface, KnotRemovalDiagnostics, error) {
	return s.removeSurfaceKnot(t, count, false, o)
}
func (s *BSplineSurface) removeSurfaceKnot(t float64, count int, uDirection bool, o KnotRemovalOptions) (candidate *BSplineSurface, d KnotRemovalDiagnostics, err error) {
	if s == nil || len(s.poles) == 0 || !finite(t) || count < 0 {
		return nil, d, ErrInvalid
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-9
	}
	if o.RConditionLimit == 0 {
		o.RConditionLimit = 1e12
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 256
	}
	if o.MaxWork == 0 {
		o.MaxWork = 50000000
	}
	if o.MaxExactOperations == 0 {
		o.MaxExactOperations = 1000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.Tolerance) || o.Tolerance <= 0 || !finite(o.RConditionLimit) || o.RConditionLimit < 1 || o.MaxPoles < 4 || o.MaxWork < 1 || o.MaxExactOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, d, ErrInvalid
	}
	if len(s.poles) > o.MaxPoles {
		return nil, d, ErrConvergence
	}
	u0, u1, v0, v1 := s.Domain()
	for i := 0; i <= s.degreeU; i++ {
		if s.knotsU[i] != u0 || s.knotsU[len(s.knotsU)-1-i] != u1 {
			return nil, d, ErrInvalid
		}
	}
	for i := 0; i <= s.degreeV; i++ {
		if s.knotsV[i] != v0 || s.knotsV[len(s.knotsV)-1-i] != v1 {
			return nil, d, ErrInvalid
		}
	}
	n, rows, degree, knots := s.countU, s.countV, s.degreeU, s.knotsU
	if !uDirection {
		n, rows, degree, knots = s.countV, s.countU, s.degreeV, s.knotsV
	}
	if t <= knots[degree] || t >= knots[n] {
		return nil, d, ErrDomain
	}
	if count == 0 {
		candidate, err = NewBSplineSurface(s.degreeU, s.degreeV, s.countU, s.countV, s.poles, s.knotsU, s.knotsV, s.weights)
		d.CandidatePoles = len(s.poles)
		return candidate, d, err
	}
	m := n - count
	if m <= degree {
		return nil, d, ErrInvalid
	}
	if int64(n) > o.MaxWork/int64(rows)/int64(m)/int64(m) {
		return nil, d, ErrConvergence
	}
	newU, newV := m, rows
	if !uDirection {
		newU, newV = rows, m
	}
	poles := make([]Vec3, newU*newV)
	weights := make([]float64, len(poles))
	var newKnots []float64
	for row := 0; row < rows; row++ {
		cp := make([]Vec3, n)
		cw := make([]float64, n)
		for i := 0; i < n; i++ {
			index := i*s.countV + row
			if !uDirection {
				index = row*s.countV + i
			}
			cp[i], cw[i] = s.poles[index], s.weights[index]
		}
		// Preserve the global weight scale when building each immutable row curve.
		original := &BSpline{degree: degree, poles: cp, weights: cw, knots: knots}
		rowOptions := o
		rowOptions.MaxExactOperations = o.MaxExactOperations - d.ExactOperations
		if rowOptions.MaxExactOperations < 1 {
			return nil, d, ErrConvergence
		}
		removed, rd, e := original.RemoveKnot(t, count, rowOptions)
		d.ExactOperations += rd.ExactOperations
		d.RConditionEstimate = max(d.RConditionEstimate, rd.RConditionEstimate)
		if e != nil {
			return nil, d, e
		}
		newKnots = removed.knots
		// Restore row-relative denominator scale. Final tensor proof includes the
		// rounding in this endpoint-anchored scale and constructor normalization.
		scale := cw[0] / removed.weights[0]
		for i := 0; i < m; i++ {
			index := i*newV + row
			if !uDirection {
				index = row*newV + i
			}
			poles[index], weights[index] = removed.poles[i], removed.weights[i]*scale
		}
	}
	ku, kv := newKnots, s.knotsV
	if !uDirection {
		ku, kv = s.knotsU, newKnots
	}
	candidate, err = NewBSplineSurface(s.degreeU, s.degreeV, newU, newV, poles, ku, kv, weights)
	if err != nil {
		return nil, d, err
	}
	d.CandidatePoles = len(poles)
	budget := &polynomialBudget{maxOperations: o.MaxExactOperations - d.ExactOperations, maxBits: o.MaxCoefficientBits}
	if budget.maxOperations < 1 {
		return nil, d, ErrConvergence
	}
	defer func() { d.ExactOperations += budget.operations }()
	bound, he, we, e := surfaceRemovalBound(s, candidate, t, count, uDirection, budget)
	d.GlobalErrorBound, d.HomogeneousError, d.WeightError = bound, he, we
	if e != nil {
		return nil, d, e
	}
	if !finite(bound) || bound > o.Tolerance {
		return nil, d, ErrKnotRemoval
	}
	d.Removed = count
	return candidate, d, nil
}
func surfaceRemovalBound(original, candidate *BSplineSurface, t float64, count int, uDirection bool, budget *polynomialBudget) (float64, float64, float64, error) {
	origin := original.poles[0]
	source := &BSpline{poles: original.poles, weights: original.weights}
	target := &BSpline{poles: candidate.poles, weights: candidate.weights}
	a, e := centeredExactNet(source, origin, big.NewRat(1, 1), budget)
	if e != nil {
		return 0, 0, 0, e
	}
	scale := new(big.Rat).Quo(new(big.Rat).SetFloat64(original.weights[0]), new(big.Rat).SetFloat64(candidate.weights[0]))
	net, e := centeredExactNet(target, origin, scale, budget)
	if e != nil {
		return 0, 0, 0, e
	}
	aligned := make([]exactHomogeneous, len(a))
	n, rows, degree, knots := candidate.countU, candidate.countV, candidate.degreeU, candidate.knotsU
	if !uDirection {
		n, rows, degree, knots = candidate.countV, candidate.countU, candidate.degreeV, candidate.knotsV
	}
	for row := 0; row < rows; row++ {
		column := make([]exactHomogeneous, n)
		for i := 0; i < n; i++ {
			index := i*candidate.countV + row
			if !uDirection {
				index = row*candidate.countV + i
			}
			column[i] = net[index]
		}
		refined, e := refineExactNet(column, knots, degree, t, count, budget)
		if e != nil {
			return 0, 0, 0, e
		}
		for i, h := range refined {
			index := i*original.countV + row
			if !uDirection {
				index = row*original.countV + i
			}
			aligned[index] = h
		}
	}
	maxNumerator, maxWeight, minWeight, radius := new(big.Rat), new(big.Rat), new(big.Rat).Set(a[0][3]), new(big.Rat)
	for i, h := range a {
		sum := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			sum.Add(sum, new(big.Rat).Abs(new(big.Rat).Sub(h[axis], aligned[i][axis])))
		}
		delta := new(big.Rat).Abs(new(big.Rat).Sub(h[3], aligned[i][3]))
		if sum.Cmp(maxNumerator) > 0 {
			maxNumerator.Set(sum)
		}
		if delta.Cmp(maxWeight) > 0 {
			maxWeight.Set(delta)
		}
		if h[3].Cmp(minWeight) < 0 {
			minWeight.Set(h[3])
		}
		if e := budget.check(sum); e != nil {
			return 0, 0, 0, e
		}
		if e := budget.check(delta); e != nil {
			return 0, 0, 0, e
		}
	}
	for _, p := range candidate.poles {
		sum := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			sum.Add(sum, new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(p, axis)), new(big.Rat).SetFloat64(coordinate(origin, axis)))))
		}
		if sum.Cmp(radius) > 0 {
			radius.Set(sum)
		}
		if e := budget.check(sum); e != nil {
			return 0, 0, 0, e
		}
	}
	// Positive tensor Bernstein/B-spline bases form a partition of unity. Hence
	// ||P-Q|| <= (maxDeltaA + maxDeltaW*max||Q-origin||)/minOriginalW.
	bound := new(big.Rat).Quo(new(big.Rat).Add(maxNumerator, new(big.Rat).Mul(maxWeight, radius)), minWeight)
	if e := budget.check(bound); e != nil {
		return 0, 0, 0, e
	}
	return rationalFloatBound(bound, false), rationalFloatBound(maxNumerator, false), rationalFloatBound(maxWeight, false), nil
}
