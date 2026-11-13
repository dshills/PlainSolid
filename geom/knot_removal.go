package geom

import (
	"errors"
	"math"
	"sort"
)

var ErrKnotRemoval = errors.New("geom: knot removal cannot satisfy the requested error bound")

// KnotRemovalOptions controls bounded homogeneous knot removal. Zero values
// select Tolerance=1e-9, RConditionLimit=1e12, MaxPoles=256, MaxWork=50M,
// MaxExactOperations=1M and MaxCoefficientBits=16384. MaxWork estimates dense
// QR work as originalPoleCount*candidatePoleCount^2.
type KnotRemovalOptions struct {
	Tolerance, RConditionLimit             float64
	MaxPoles                               int
	MaxWork                                int64
	MaxExactOperations, MaxCoefficientBits int
}

// KnotRemovalDiagnostics describes the acceptance proof and numerical solve.
// GlobalErrorBound is an outward-rounded conservative Euclidean bound, derived
// by exact rational reinsertion of the returned float64 candidate control net.
// HomogeneousError is a centered numerator L1 bound; WeightError bounds aligned
// denominator control differences. The bound applies to the ideal rational
// curves defined by the stored coefficients; subsequent floating evaluation
// error is additional. Removed is zero on failure.
type KnotRemovalDiagnostics struct {
	Removed, CandidatePoles, ExactOperations                            int
	GlobalErrorBound, HomogeneousError, WeightError, RConditionEstimate float64
}

// RemoveKnot removes count occurrences of a strictly interior knot from a
// clamped rational curve. A column-pivoted QR solve inverts the insertion map;
// positive candidate weights and a conservative global error proof are required
// before returning a curve. The verification includes constructor rounding and
// rational-weight renormalization. No geometric sampling is used for acceptance.
// Failure to meet the bound returns ErrKnotRemoval and no candidate; singular
// systems and exhausted work/arithmetic limits have distinct errors. A
// conservative bound can reject a candidate whose actual error is smaller.
func (c *BSpline) RemoveKnot(t float64, count int, options KnotRemovalOptions) (candidate *BSpline, diagnostics KnotRemovalDiagnostics, err error) {
	o := options
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
	if c == nil || len(c.poles) == 0 || !finite(t) || count < 0 || !finite(o.Tolerance) || o.Tolerance <= 0 || !finite(o.RConditionLimit) || o.RConditionLimit < 1 || o.MaxPoles < 2 || o.MaxWork < 1 || o.MaxExactOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, diagnostics, ErrInvalid
	}
	a, b := c.Domain()
	if t <= a || t >= b {
		return nil, diagnostics, ErrDomain
	}
	for i := 0; i <= c.degree; i++ {
		if c.knots[i] != a || c.knots[len(c.knots)-1-i] != b {
			return nil, diagnostics, ErrInvalid
		}
	}
	n := len(c.poles)
	if n > o.MaxPoles {
		return nil, diagnostics, ErrConvergence
	}
	if count == 0 {
		copy, e := NewBSpline(c.degree, c.poles, c.knots, c.weights)
		diagnostics.CandidatePoles = n
		return copy, diagnostics, e
	}
	first := sort.SearchFloat64s(c.knots, t)
	last := sort.Search(len(c.knots), func(i int) bool { return c.knots[i] > t })
	m := n - count
	diagnostics.CandidatePoles = m
	if count > last-first || m <= c.degree {
		return nil, diagnostics, ErrInvalid
	}
	if int64(n) > o.MaxWork/int64(m)/int64(m) {
		return nil, diagnostics, ErrConvergence
	}
	knots := make([]float64, 0, len(c.knots)-count)
	knots = append(knots, c.knots[:first]...)
	knots = append(knots, c.knots[first+count:]...)
	if e := validateKnots(c.degree, m, knots); e != nil {
		return nil, diagnostics, e
	}
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, m)
	}
	for column := 0; column < m; column++ {
		basis := make([]homogeneous, m)
		basis[column][0] = 1
		refined, aligned := refineHomogeneous(basis, knots, c.degree, t, count)
		if len(refined) != n || len(aligned) != len(c.knots) {
			return nil, diagnostics, ErrInvalid
		}
		for i, k := range aligned {
			if k != c.knots[i] {
				return nil, diagnostics, ErrInvalid
			}
		}
		for row := range refined {
			matrix[row][column] = refined[row][0]
		}
	}
	qr, e := factorApproximationQR(matrix)
	if e != nil {
		return nil, diagnostics, e
	}
	condition, e := qr.conditionEstimate()
	diagnostics.RConditionEstimate = condition
	if e != nil {
		return nil, diagnostics, e
	}
	if condition > o.RConditionLimit {
		return nil, diagnostics, ErrIllConditioned
	}
	origin := c.poles[0]
	source := make([]homogeneous, n)
	for i, p := range c.poles {
		delta := p.Sub(origin)
		if !delta.IsFinite() {
			return nil, diagnostics, ErrInvalid
		}
		w := c.weights[i]
		source[i] = homogeneous{delta.X * w, delta.Y * w, delta.Z * w, w}
	}
	solution := [4][]float64{}
	for axis := 0; axis < 4; axis++ {
		scale := 0.0
		for _, p := range source {
			scale = math.Max(scale, math.Abs(p[axis]))
		}
		if scale == 0 {
			solution[axis] = make([]float64, m)
			continue
		}
		rhs := make([]float64, n)
		for i, p := range source {
			rhs[i] = p[axis] / scale
		}
		values, e := qr.solve(rhs)
		if e != nil {
			return nil, diagnostics, e
		}
		for i := range values {
			values[i] *= scale
			if !finite(values[i]) {
				return nil, diagnostics, ErrKnotRemoval
			}
		}
		solution[axis] = values
	}
	poles := make([]Vec3, m)
	weights := solution[3]
	normalizationScale := 0.0
	for i, w := range weights {
		if w <= 0 || !finite(w) {
			return nil, diagnostics, ErrKnotRemoval
		}
		normalizationScale = math.Max(normalizationScale, w)
		poles[i] = origin.Add(Vec3{solution[0][i] / w, solution[1][i] / w, solution[2][i] / w})
		if !poles[i].IsFinite() {
			return nil, diagnostics, ErrKnotRemoval
		}
	}
	candidate, e = NewBSpline(c.degree, poles, knots, weights)
	if e != nil {
		return nil, diagnostics, e
	}
	budget := &polynomialBudget{maxOperations: o.MaxExactOperations, maxBits: o.MaxCoefficientBits}
	bound, numeratorError, weightError, e := removalGlobalBound(c, candidate, t, count, normalizationScale, budget)
	diagnostics.ExactOperations = budget.operations
	diagnostics.GlobalErrorBound, diagnostics.HomogeneousError, diagnostics.WeightError = bound, numeratorError, weightError
	if e != nil {
		return nil, diagnostics, e
	}
	if !finite(bound) || bound > o.Tolerance {
		return nil, diagnostics, ErrKnotRemoval
	}
	diagnostics.Removed = count
	return candidate, diagnostics, nil
}
