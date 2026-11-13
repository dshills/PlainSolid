package geom

import "math"

// ApproximationOptions specifies a fixed approximation space. PoleCount is
// required; Degree=0 selects min(3,PoleCount-1). Knots=nil creates a uniform
// clamped knot vector. Weights fixes positive rational pole weights (nil means
// polynomial); weights are not optimized. Parameters=nil uses normalized chord
// length. Explicit parameters may repeat but must be nondecreasing with a
// nonzero domain. SampleWeights weights squared sample residuals and must be
// positive. MaxResidual=0 only reports error; positive values enforce a cap.
// Zero limits select 4096 samples, 256 poles, 50M estimated work units and
// RConditionLimit=1e12. This is approximation, not endpoint interpolation.
type ApproximationOptions struct {
	Degree, PoleCount                         int
	Parameters, Knots, Weights, SampleWeights []float64
	MaxResidual                               float64
	MaxSamples, MaxPoles                      int
	MaxWork                                   int64
	RConditionLimit                           float64
}

// ApproximationDiagnostics exposes conditioning and physical residuals.
// RConditionEstimate is ||R||inf*||inverse(R)||inf for pivoted QR, not the
// infinity-norm condition of the rectangular design matrix. It is a numerical
// estimate, not a certified bound. WeightedRMSResidual uses supplied sample
// weights; RMSResidual gives every sample equal weight. Rank counts nonzero
// QR pivots; the condition gate can reject a system even when that count is full.
type ApproximationDiagnostics struct {
	Degree, PoleCount, Rank                                                       int
	Parameters                                                                    []float64
	RConditionEstimate, PivotRatio, MaxResidual, RMSResidual, WeightedRMSResidual float64
}

// ApproximateBSpline minimizes weighted squared point residuals for fixed knot
// and rational-weight data using column-pivoted Householder QR. It never forms
// normal equations. Inputs are copied; returned geometry is immutable. Singular
// or ill-conditioned systems fail explicitly; a residual-cap failure returns
// no curve and preserves diagnostics. Periodicity and endpoint constraints are
// not inferred, and rational weights are not fitted nonlinearly.
func ApproximateBSpline(points []Vec3, options ApproximationOptions) (*BSpline, ApproximationDiagnostics, error) {
	diagnostics := ApproximationDiagnostics{}
	o := options
	if o.MaxSamples == 0 {
		o.MaxSamples = 4096
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 256
	}
	if o.MaxWork == 0 {
		o.MaxWork = 50000000
	}
	if o.RConditionLimit == 0 {
		o.RConditionLimit = 1e12
	}
	n, m := len(points), o.PoleCount
	if n < 2 || m < 2 || m > n || o.MaxSamples < 2 || o.MaxPoles < 2 || o.MaxWork < 1 || !finite(o.RConditionLimit) || o.RConditionLimit < 1 || !finite(o.MaxResidual) || o.MaxResidual < 0 {
		return nil, diagnostics, ErrInvalid
	}
	if n > o.MaxSamples || m > o.MaxPoles || int64(n) > o.MaxWork/int64(m)/int64(m) {
		return nil, diagnostics, ErrConvergence
	}
	if o.Degree == 0 {
		o.Degree = min(3, m-1)
	}
	if o.Degree < 1 || o.Degree >= m || o.Degree > 64 {
		return nil, diagnostics, ErrInvalid
	}
	for _, point := range points {
		if !point.IsFinite() {
			return nil, diagnostics, ErrInvalid
		}
	}
	parameters := append([]float64(nil), o.Parameters...)
	if parameters == nil {
		parameters = make([]float64, n)
		for i := 1; i < n; i++ {
			distance := points[i].Distance(points[i-1])
			if !finite(distance) {
				return nil, diagnostics, ErrInvalid
			}
			parameters[i] = parameters[i-1] + distance
			if !finite(parameters[i]) {
				return nil, diagnostics, ErrInvalid
			}
		}
		total := parameters[n-1]
		if total == 0 {
			return nil, diagnostics, ErrDegenerate
		}
		for i := range parameters {
			parameters[i] /= total
		}
	}
	if len(parameters) != n {
		return nil, diagnostics, ErrInvalid
	}
	for i, value := range parameters {
		if !finite(value) || (i > 0 && value < parameters[i-1]) {
			return nil, diagnostics, ErrInvalid
		}
	}
	a, b := parameters[0], parameters[n-1]
	if b <= a || !finite(b-a) {
		return nil, diagnostics, ErrInvalid
	}
	knots := append([]float64(nil), o.Knots...)
	if knots == nil {
		knots = make([]float64, m+o.Degree+1)
		for i := 0; i <= o.Degree; i++ {
			knots[i] = a
			knots[len(knots)-1-i] = b
		}
		for j := 1; j < m-o.Degree; j++ {
			knots[j+o.Degree] = a + (b-a)*(float64(j)/float64(m-o.Degree))
		}
	}
	if e := validateKnots(o.Degree, m, knots); e != nil {
		return nil, diagnostics, e
	}
	if knots[o.Degree] != a || knots[m] != b {
		return nil, diagnostics, ErrDomain
	}
	for i := 0; i <= o.Degree; i++ {
		if knots[i] != a || knots[len(knots)-1-i] != b {
			return nil, diagnostics, ErrInvalid
		}
	}
	poleWeights, e := copyWeights(m, o.Weights)
	if e != nil {
		return nil, diagnostics, e
	}
	sampleWeights, e := copyWeights(n, o.SampleWeights)
	if e != nil {
		return nil, diagnostics, e
	}
	rowScales := make([]float64, n)
	matrix := make([][]float64, n)
	for i, t := range parameters {
		basis, e := interpolationBasis(o.Degree, m, knots, t)
		if e != nil {
			return nil, diagnostics, e
		}
		denominator := 0.0
		for j := range basis {
			basis[j] *= poleWeights[j]
			denominator += basis[j]
		}
		if denominator <= 0 || !finite(denominator) {
			return nil, diagnostics, ErrIllConditioned
		}
		rowScales[i] = math.Sqrt(sampleWeights[i])
		for j := range basis {
			basis[j] = rowScales[i] * (basis[j] / denominator)
		}
		matrix[i] = basis
	}
	diagnostics.Degree, diagnostics.PoleCount = o.Degree, m
	diagnostics.Parameters = append([]float64(nil), parameters...)
	qr, e := factorApproximationQR(matrix)
	diagnostics.Rank, diagnostics.PivotRatio = qr.rank, qr.pivotRatio
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
	origin := points[0]
	local := make([]Vec3, n)
	for i, point := range points {
		local[i] = point.Sub(origin)
		if !local[i].IsFinite() {
			return nil, diagnostics, ErrInvalid
		}
	}
	coordinates := [3][]float64{}
	for axis := 0; axis < 3; axis++ {
		scale := 0.0
		for _, point := range local {
			scale = math.Max(scale, math.Abs(coordinate(point, axis)))
		}
		if scale == 0 {
			coordinates[axis] = make([]float64, m)
			continue
		}
		rhs := make([]float64, n)
		for i, point := range local {
			rhs[i] = rowScales[i] * (coordinate(point, axis) / scale)
		}
		solution, e := qr.solve(rhs)
		if e != nil {
			return nil, diagnostics, e
		}
		for i := range solution {
			solution[i] *= scale
		}
		coordinates[axis] = solution
	}
	poles := make([]Vec3, m)
	for i := range poles {
		poles[i] = origin.Add(Vec3{coordinates[0][i], coordinates[1][i], coordinates[2][i]})
		if !poles[i].IsFinite() {
			return nil, diagnostics, ErrInvalid
		}
	}
	curve, e := NewBSpline(o.Degree, poles, knots, poleWeights)
	if e != nil {
		return nil, diagnostics, e
	}
	residuals := make([]float64, n)
	for i, t := range parameters {
		point, e := curve.Eval(t)
		if e != nil {
			return nil, diagnostics, e
		}
		residuals[i] = point.Distance(points[i])
		if !finite(residuals[i]) {
			return nil, diagnostics, ErrInvalid
		}
		diagnostics.MaxResidual = math.Max(diagnostics.MaxResidual, residuals[i])
	}
	if diagnostics.MaxResidual > 0 {
		sum, weightedSum, weightSum := 0.0, 0.0, 0.0
		for i, residual := range residuals {
			scaled := residual / diagnostics.MaxResidual
			sum += scaled * scaled
			weightedSum += sampleWeights[i] * scaled * scaled
			weightSum += sampleWeights[i]
		}
		diagnostics.RMSResidual = diagnostics.MaxResidual * math.Sqrt(sum/float64(n))
		diagnostics.WeightedRMSResidual = diagnostics.MaxResidual * math.Sqrt(weightedSum/weightSum)
	}
	if o.MaxResidual > 0 && diagnostics.MaxResidual > o.MaxResidual {
		return nil, diagnostics, ErrConvergence
	}
	return curve, diagnostics, nil
}
