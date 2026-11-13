package geom

import "math"

// pivotedQR stores A*P=Q*R with compact Householder reflectors below the diagonal.
type pivotedQR struct {
	matrix      [][]float64
	tau         []float64
	permutation []int
	rank        int
	pivotRatio  float64
}

func factorApproximationQR(a [][]float64) (pivotedQR, error) {
	rows, columns := len(a), len(a[0])
	qr := pivotedQR{matrix: make([][]float64, rows), tau: make([]float64, columns), permutation: make([]int, columns)}
	for i := range a {
		qr.matrix[i] = append([]float64(nil), a[i]...)
	}
	for j := range qr.permutation {
		qr.permutation[j] = j
	}
	maximumPivot, minimumPivot := 0.0, math.Inf(1)
	for k := 0; k < columns; k++ {
		pivot, norm := k, 0.0
		for j := k; j < columns; j++ {
			candidate := 0.0
			for i := k; i < rows; i++ {
				candidate = math.Hypot(candidate, qr.matrix[i][j])
			}
			if candidate > norm {
				pivot, norm = j, candidate
			}
		}
		if norm == 0 || !finite(norm) {
			return qr, ErrIllConditioned
		}
		if pivot != k {
			for i := 0; i < rows; i++ {
				qr.matrix[i][k], qr.matrix[i][pivot] = qr.matrix[i][pivot], qr.matrix[i][k]
			}
			qr.permutation[k], qr.permutation[pivot] = qr.permutation[pivot], qr.permutation[k]
		}
		original := qr.matrix[k][k]
		alpha := -math.Copysign(norm, original)
		denominator := original - alpha
		if denominator == 0 || !finite(denominator) {
			return qr, ErrIllConditioned
		}
		qr.tau[k] = (alpha - original) / alpha
		qr.matrix[k][k] = alpha
		for i := k + 1; i < rows; i++ {
			qr.matrix[i][k] /= denominator
		}
		for j := k + 1; j < columns; j++ {
			dot := qr.matrix[k][j]
			for i := k + 1; i < rows; i++ {
				dot += qr.matrix[i][k] * qr.matrix[i][j]
			}
			factor := qr.tau[k] * dot
			qr.matrix[k][j] -= factor
			for i := k + 1; i < rows; i++ {
				qr.matrix[i][j] -= qr.matrix[i][k] * factor
			}
		}
		minimumPivot = math.Min(minimumPivot, math.Abs(alpha))
		maximumPivot = math.Max(maximumPivot, math.Abs(alpha))
		qr.rank++
	}
	qr.pivotRatio = minimumPivot / maximumPivot
	return qr, nil
}
func (qr pivotedQR) solveR(rhs []float64) ([]float64, error) {
	columns := len(qr.tau)
	solution := append([]float64(nil), rhs[:columns]...)
	for i := columns - 1; i >= 0; i-- {
		for j := i + 1; j < columns; j++ {
			solution[i] -= qr.matrix[i][j] * solution[j]
		}
		solution[i] /= qr.matrix[i][i]
		if !finite(solution[i]) {
			return nil, ErrIllConditioned
		}
	}
	return solution, nil
}
func (qr pivotedQR) solve(rhs []float64) ([]float64, error) {
	projected := append([]float64(nil), rhs...)
	for k, tau := range qr.tau {
		dot := projected[k]
		for i := k + 1; i < len(projected); i++ {
			dot += qr.matrix[i][k] * projected[i]
		}
		factor := tau * dot
		projected[k] -= factor
		for i := k + 1; i < len(projected); i++ {
			projected[i] -= qr.matrix[i][k] * factor
		}
	}
	pivoted, e := qr.solveR(projected)
	if e != nil {
		return nil, e
	}
	solution := make([]float64, len(pivoted))
	for i, value := range pivoted {
		solution[qr.permutation[i]] = value
	}
	return solution, nil
}
func (qr pivotedQR) conditionEstimate() (float64, error) {
	columns := len(qr.tau)
	normR := 0.0
	for i := 0; i < columns; i++ {
		sum := 0.0
		for j := i; j < columns; j++ {
			sum += math.Abs(qr.matrix[i][j])
		}
		normR = math.Max(normR, sum)
	}
	inverseRows := make([]float64, columns)
	for j := 0; j < columns; j++ {
		rhs := make([]float64, columns)
		rhs[j] = 1
		column, e := qr.solveR(rhs)
		if e != nil {
			return 0, e
		}
		for i, value := range column {
			inverseRows[i] += math.Abs(value)
		}
	}
	normInverse := 0.0
	for _, sum := range inverseRows {
		normInverse = math.Max(normInverse, sum)
	}
	estimate := normR * normInverse
	if !finite(estimate) {
		return estimate, ErrIllConditioned
	}
	return estimate, nil
}
