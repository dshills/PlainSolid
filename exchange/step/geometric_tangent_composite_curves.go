package step

import (
	"io"
	"math/big"
)

// GeometricTangentCompositeCurveModel uses the same explicit transition matrix
// as TangentCompositeCurveModel but permits positive unequal parameter speeds.
// Each true declaration requires exact nonzero positively proportional directed
// endpoint derivatives. It asserts neither curvature continuity nor simplicity.
type GeometricTangentCompositeCurveModel struct{ TangentCompositeCurveModel }

// ReadGeometricTangentCompositeCurves accepts bounded direct spline composites
// with exactly proved geometric G1 joins, without normalizing source parameters.
func ReadGeometricTangentCompositeCurves(r io.Reader, l Limits) (GeometricTangentCompositeCurveModel, error) {
	var flags [][]bool
	m, e := readCompositeCurves(r, l, &flags, true)
	if e != nil {
		return GeometricTangentCompositeCurveModel{}, e
	}
	return GeometricTangentCompositeCurveModel{TangentCompositeCurveModel{m, flags}}, nil
}

// WriteGeometricTangentCompositeCurves stages exact source readback before I/O.
// Existing positional and C1 writers retain their stricter contracts.
func WriteGeometricTangentCompositeCurves(w io.Writer, m GeometricTangentCompositeCurveModel) error {
	if m.TangentTransitions == nil {
		return compositeFailure()
	}
	return writeCompositeCurves(w, m.CompositeCurveModel, m.TangentTransitions, true)
}

func positiveProportionalCompositeTangents(a, b [3]*big.Rat) bool {
	pivot := -1
	for i := range a {
		if a[i].Sign() != 0 {
			pivot = i
			break
		}
	}
	if pivot < 0 || a[pivot].Sign() != b[pivot].Sign() {
		return false
	}
	for i := range a {
		left := new(big.Rat).Mul(a[i], b[pivot])
		right := new(big.Rat).Mul(b[i], a[pivot])
		if left.Cmp(right) != 0 {
			return false
		}
	}
	return true
}
