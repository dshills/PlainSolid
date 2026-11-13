package geom

import (
	"errors"
	"math"
	"sort"
)

var ErrIllConditioned = errors.New("geom: interpolation system is ill-conditioned")

// InterpolationOptions controls global polynomial B-spline interpolation.
// Degree=0 selects min(3,len(points)-1). Parameters=nil selects normalized
// chord-length parameters. Explicit parameters must be finite and strictly
// increasing; their original domain is retained. Zero numeric limits select
// MaxPoints=256, ConditionLimit=1e12, AbsoluteTolerance=1e-9 and
// RelativeTolerance=1e-12. Residual tolerance uses the data's local scale.
type InterpolationOptions struct {
	Degree                                               int
	Parameters                                           []float64
	MaxPoints                                            int
	ConditionLimit, AbsoluteTolerance, RelativeTolerance float64
}

// InterpolationDiagnostics reports a full infinity-norm condition estimate
// ||A||inf*||inv(A)||inf, a scaled LU pivot ratio, and evaluated point residuals.
// The estimate is computed in float64 and is not a rigorous condition bound.
type InterpolationDiagnostics struct {
	Degree                                     int
	Parameters                                 []float64
	ConditionEstimate, PivotRatio, MaxResidual float64
}

func interpolationBasis(degree, count int, knots []float64, t float64) ([]float64, error) {
	span := sort.SearchFloat64s(knots, t) - 1
	if t < knots[count] {
		span = sort.Search(len(knots), func(i int) bool { return knots[i] > t }) - 1
	}
	basis := make([]float64, degree+1)
	left, right := make([]float64, degree+1), make([]float64, degree+1)
	basis[0] = 1
	for j := 1; j <= degree; j++ {
		left[j] = t - knots[span+1-j]
		right[j] = knots[span+j] - t
		saved := 0.0
		for r := 0; r < j; r++ {
			den := right[r+1] + left[j-r]
			if den == 0 || !finite(den) {
				return nil, ErrIllConditioned
			}
			temp := basis[r] / den
			basis[r] = saved + right[r+1]*temp
			saved = left[j-r] * temp
		}
		basis[j] = saved
	}
	row := make([]float64, count)
	copy(row[span-degree:span+1], basis)
	return row, nil
}

type interpolationLU struct {
	a           [][]float64
	permutation []int
	pivotRatio  float64
}

func factorInterpolation(a [][]float64) (interpolationLU, error) {
	n := len(a)
	lu := interpolationLU{a: make([][]float64, n), permutation: make([]int, n)}
	scale := make([]float64, n)
	for i := range a {
		lu.a[i] = append([]float64(nil), a[i]...)
		lu.permutation[i] = i
		for _, x := range a[i] {
			scale[i] = math.Max(scale[i], math.Abs(x))
		}
		if scale[i] == 0 {
			return lu, ErrIllConditioned
		}
	}
	minimum, maximum := math.Inf(1), 0.0
	for k := 0; k < n; k++ {
		pivot := k
		for i := k + 1; i < n; i++ {
			if math.Abs(lu.a[i][k])/scale[i] > math.Abs(lu.a[pivot][k])/scale[pivot] {
				pivot = i
			}
		}
		if lu.a[pivot][k] == 0 {
			return lu, ErrIllConditioned
		}
		lu.a[k], lu.a[pivot] = lu.a[pivot], lu.a[k]
		lu.permutation[k], lu.permutation[pivot] = lu.permutation[pivot], lu.permutation[k]
		scale[k], scale[pivot] = scale[pivot], scale[k]
		relative := math.Abs(lu.a[k][k]) / scale[k]
		minimum = math.Min(minimum, relative)
		maximum = math.Max(maximum, relative)
		for i := k + 1; i < n; i++ {
			lu.a[i][k] /= lu.a[k][k]
			if !finite(lu.a[i][k]) {
				return lu, ErrIllConditioned
			}
			for j := k + 1; j < n; j++ {
				lu.a[i][j] -= lu.a[i][k] * lu.a[k][j]
				if !finite(lu.a[i][j]) {
					return lu, ErrIllConditioned
				}
			}
		}
	}
	lu.pivotRatio = minimum / maximum
	return lu, nil
}
func (lu interpolationLU) solve(b []float64) ([]float64, error) {
	n := len(b)
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = b[lu.permutation[i]]
		for j := 0; j < i; j++ {
			x[i] -= lu.a[i][j] * x[j]
		}
	}
	for i := n - 1; i >= 0; i-- {
		for j := i + 1; j < n; j++ {
			x[i] -= lu.a[i][j] * x[j]
		}
		x[i] /= lu.a[i][i]
		if !finite(x[i]) {
			return nil, ErrIllConditioned
		}
	}
	return x, nil
}

// Interpolate constructs a polynomial (unit-weight) global interpolating spline
// with averaged interior knots. Scaled partial-pivot LU solves all coordinates;
// explicit condition and residual gates reject unreliable results. There are no
// endpoint derivative constraints, periodic closure inference, or smoothing.
func Interpolate(points []Vec3, options InterpolationOptions) (*BSpline, InterpolationDiagnostics, error) {
	diagnostics := InterpolationDiagnostics{}
	o := options
	if o.MaxPoints == 0 {
		o.MaxPoints = 256
	}
	if o.ConditionLimit == 0 {
		o.ConditionLimit = 1e12
	}
	if o.AbsoluteTolerance == 0 {
		o.AbsoluteTolerance = 1e-9
	}
	if o.RelativeTolerance == 0 {
		o.RelativeTolerance = 1e-12
	}
	n := len(points)
	if n < 2 || o.MaxPoints < 2 || o.MaxPoints > 4096 || !finite(o.ConditionLimit) || o.ConditionLimit < 1 || !finite(o.AbsoluteTolerance) || !finite(o.RelativeTolerance) || o.AbsoluteTolerance < 0 || o.RelativeTolerance < 0 {
		return nil, diagnostics, ErrInvalid
	}
	if n > o.MaxPoints {
		return nil, diagnostics, ErrConvergence
	}
	if o.Degree == 0 {
		o.Degree = min(3, n-1)
	}
	if o.Degree < 1 || o.Degree >= n || o.Degree > 64 {
		return nil, diagnostics, ErrInvalid
	}
	for _, p := range points {
		if !p.IsFinite() {
			return nil, diagnostics, ErrInvalid
		}
	}
	parameters := append([]float64(nil), o.Parameters...)
	if parameters == nil {
		parameters = make([]float64, n)
		for i := 1; i < n; i++ {
			distance := points[i].Distance(points[i-1])
			if !finite(distance) || distance == 0 {
				return nil, diagnostics, ErrDegenerate
			}
			parameters[i] = parameters[i-1] + distance
			if !finite(parameters[i]) {
				return nil, diagnostics, ErrInvalid
			}
		}
		total := parameters[n-1]
		for i := range parameters {
			parameters[i] /= total
		}
	}
	if len(parameters) != n {
		return nil, diagnostics, ErrInvalid
	}
	for i, t := range parameters {
		if !finite(t) || (i > 0 && t <= parameters[i-1]) {
			return nil, diagnostics, ErrInvalid
		}
	}
	domain := parameters[n-1] - parameters[0]
	if !finite(domain) || domain <= 0 {
		return nil, diagnostics, ErrInvalid
	}
	diagnostics.Degree = o.Degree
	diagnostics.Parameters = append([]float64(nil), parameters...)
	knots := make([]float64, n+o.Degree+1)
	for i := 0; i <= o.Degree; i++ {
		knots[i] = parameters[0]
		knots[len(knots)-1-i] = parameters[n-1]
	}
	for j := 1; j < n-o.Degree; j++ {
		sum := 0.0
		for i := j; i < j+o.Degree; i++ {
			sum += (parameters[i] - parameters[0]) / float64(o.Degree)
		}
		knots[j+o.Degree] = parameters[0] + sum
	}
	if e := validateKnots(o.Degree, n, knots); e != nil {
		return nil, diagnostics, e
	}
	matrix := make([][]float64, n)
	normA := 0.0
	for i, t := range parameters {
		row, e := interpolationBasis(o.Degree, n, knots, t)
		if e != nil {
			return nil, diagnostics, e
		}
		matrix[i] = row
		sum := 0.0
		for _, x := range row {
			sum += math.Abs(x)
		}
		normA = math.Max(normA, sum)
	}
	lu, e := factorInterpolation(matrix)
	if e != nil {
		return nil, diagnostics, e
	}
	diagnostics.PivotRatio = lu.pivotRatio
	inverseRowSums := make([]float64, n)
	for column := 0; column < n; column++ {
		rhs := make([]float64, n)
		rhs[column] = 1
		solution, e := lu.solve(rhs)
		if e != nil {
			return nil, diagnostics, e
		}
		for row, x := range solution {
			inverseRowSums[row] += math.Abs(x)
		}
	}
	for _, sum := range inverseRowSums {
		diagnostics.ConditionEstimate = math.Max(diagnostics.ConditionEstimate, normA*sum)
	}
	if !finite(diagnostics.ConditionEstimate) || diagnostics.ConditionEstimate > o.ConditionLimit {
		return nil, diagnostics, ErrIllConditioned
	}
	origin := points[0]
	local := make([]Vec3, n)
	localScale := 0.0
	for i, p := range points {
		local[i] = p.Sub(origin)
		length := local[i].Length()
		if !finite(length) {
			return nil, diagnostics, ErrInvalid
		}
		localScale = math.Max(localScale, length)
	}
	coordinates := [3][]float64{}
	for axis := 0; axis < 3; axis++ {
		rhs := make([]float64, n)
		scale := 0.0
		for i, p := range local {
			rhs[i] = coordinate(p, axis)
			scale = math.Max(scale, math.Abs(rhs[i]))
		}
		if scale == 0 {
			coordinates[axis] = rhs
			continue
		}
		for i := range rhs {
			rhs[i] /= scale
		}
		solution, e := lu.solve(rhs)
		if e != nil {
			return nil, diagnostics, e
		}
		for i := range solution {
			solution[i] *= scale
		}
		coordinates[axis] = solution
	}
	poles := make([]Vec3, n)
	for i := range poles {
		poles[i] = origin.Add(Vec3{coordinates[0][i], coordinates[1][i], coordinates[2][i]})
		if !poles[i].IsFinite() {
			return nil, diagnostics, ErrInvalid
		}
	}
	curve, e := NewBSpline(o.Degree, poles, knots, nil)
	if e != nil {
		return nil, diagnostics, e
	}
	for i, t := range parameters {
		p, e := curve.Eval(t)
		if e != nil {
			return nil, diagnostics, e
		}
		residual := p.Distance(points[i])
		if !finite(residual) {
			return nil, diagnostics, ErrInvalid
		}
		diagnostics.MaxResidual = math.Max(diagnostics.MaxResidual, residual)
	}
	threshold := o.AbsoluteTolerance + o.RelativeTolerance*localScale
	if !finite(threshold) || diagnostics.MaxResidual > threshold {
		return nil, diagnostics, ErrConvergence
	}
	return curve, diagnostics, nil
}
