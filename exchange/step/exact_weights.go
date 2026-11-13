package step

import (
	"math/big"
	"plainsolid/geom"
)

// Preserve represented source weight ratios before exact geometry certification.
func exactSplineWeightNormalization(f *File, id int64, kind string) error {
	record, ok := component(f.Entities[id], kind)
	if !ok {
		return nil
	}
	if len(record.Args) != 1 {
		return geom.ErrInvalid
	}
	weights := []float64{}
	maximum := 0.
	var visit func(Value) error
	visit = func(v Value) error {
		if v.Kind == Aggregate {
			for _, x := range v.Items {
				if e := visit(x); e != nil {
					return e
				}
			}
			return nil
		}
		x, e := numeric(v)
		if e != nil || x <= 0 {
			return geom.ErrInvalid
		}
		weights = append(weights, x)
		maximum = max(maximum, x)
		return nil
	}
	if e := visit(record.Args[0]); e != nil {
		return e
	}
	if maximum == 0 {
		return geom.ErrInvalid
	}
	divisor := new(big.Rat).SetFloat64(maximum)
	for _, w := range weights {
		want := new(big.Rat).Quo(new(big.Rat).SetFloat64(w), divisor)
		actual := new(big.Rat).SetFloat64(w / maximum)
		if actual == nil || want.Cmp(actual) != 0 {
			return geom.ErrUnrepresentable
		}
	}
	return nil
}
