package iges

import "math/big"

// Compare the represented source ratios, not the already-normalized spline.
func exactWeightNormalization(weights []float64) bool {
	maximum := 0.
	for _, w := range weights {
		maximum = max(maximum, w)
	}
	divisor := new(big.Rat).SetFloat64(maximum)
	if divisor == nil || divisor.Sign() <= 0 {
		return false
	}
	for _, w := range weights {
		raw, normalized := new(big.Rat).SetFloat64(w), new(big.Rat).SetFloat64(w/maximum)
		if raw == nil || raw.Sign() <= 0 || normalized == nil || new(big.Rat).Quo(raw, divisor).Cmp(normalized) != 0 {
			return false
		}
	}
	return true
}

func (m Model) exactContourWeights(c CurveOnSurface) bool {
	for _, index := range []int{c.Surface, c.ParameterCurve, c.ModelCurve} {
		if index < 0 || index >= len(m.Entities) {
			return false
		}
		e := m.Entities[index]
		if e.sourceWeightsRounded || e.Surface != nil && e.Surface.periodic != nil {
			return false
		}
		if e.Composite != nil {
			for _, leaf := range e.Composite.Curves {
				if leaf < 0 || leaf >= len(m.Entities) || m.Entities[leaf].sourceWeightsRounded {
					return false
				}
			}
		}
	}
	return true
}
