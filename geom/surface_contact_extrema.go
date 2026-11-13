package geom

import "math/big"

// weakSurfaceContactBounds confines all possible contacts to exact exposed
// parameter faces when coordinate hull extrema touch. For a positive-weight
// Bezier patch, a weakly signed scalar numerator can vanish only on zero
// Bernstein edges/corners unless every coefficient is zero. Closed endpoint
// contacts are retained as singleton parameter ranges, never discarded.
func weakSurfaceContactBounds(a, b *SurfacePatch, box SurfacePairParameterBox, maxOperations, maxBits int) (ranges [4][2]float64, excluded bool, operations int, err error) {
	ranges = pairParameterRanges(box)
	original := ranges
	budget := &polynomialBudget{maxOperations: maxOperations, maxBits: maxBits}
	defer func() { operations = budget.operations }()
	patches := [2]*SurfacePatch{a, b}
	restrict := func(side int) ([]exactHomogeneous, error) {
		p := patches[side]
		base := side * 2
		local := func(axis int, x float64) *big.Rat {
			return new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).SetFloat64(x), new(big.Rat).SetFloat64(original[axis][0])), new(big.Rat).Sub(new(big.Rat).SetFloat64(original[axis][1]), new(big.Rat).SetFloat64(original[axis][0])))
		}
		return restrictComparisonTensor(comparisonTensor{nu: p.countU, nv: p.countV, net: p.net}, local(base, ranges[base][0]), local(base, ranges[base][1]), local(base+1, ranges[base+1][0]), local(base+1, ranges[base+1][1]), budget)
	}
	for pass := 0; pass < 4; pass++ {
		before := ranges
		for axis := 0; axis < 3; axis++ {
			var coefficients [2][]*big.Rat
			var lower, upper [2]*big.Rat
			for side := 0; side < 2; side++ {
				net, e := restrict(side)
				if e != nil {
					return ranges, false, 0, e
				}
				coefficients[side] = make([]*big.Rat, len(net))
				for i, h := range net {
					v := new(big.Rat).Quo(h[axis], h[3])
					if e := budget.check(v); e != nil {
						return ranges, false, 0, e
					}
					coefficients[side][i] = v
					if lower[side] == nil || v.Cmp(lower[side]) < 0 {
						lower[side] = v
					}
					if upper[side] == nil || v.Cmp(upper[side]) > 0 {
						upper[side] = v
					}
				}
			}
			if upper[0].Cmp(lower[1]) < 0 || upper[1].Cmp(lower[0]) < 0 {
				return ranges, true, 0, nil
			}
			var contact *big.Rat
			if upper[0].Cmp(lower[1]) == 0 {
				contact = upper[0]
			} else if upper[1].Cmp(lower[0]) == 0 {
				contact = upper[1]
			} else {
				continue
			}
			for side, p := range patches {
				values := coefficients[side]
				if lower[side].Cmp(contact) == 0 && upper[side].Cmp(contact) == 0 {
					continue
				}
				base := 2 * side
				u, v := ranges[base], ranges[base+1]
				var bound [2][2]float64
				found := false
				include := func(x, y float64) {
					if !found {
						bound = [2][2]float64{{x, x}, {y, y}}
						found = true
					} else {
						bound[0] = [2]float64{min(bound[0][0], x), max(bound[0][1], x)}
						bound[1] = [2]float64{min(bound[1][0], y), max(bound[1][1], y)}
					}
				}
				corners := [4]int{0, (p.countU - 1) * p.countV, len(values) - 1, p.countV - 1}
				points := [4][2]float64{{u[0], v[0]}, {u[1], v[0]}, {u[1], v[1]}, {u[0], v[1]}}
				for corner, index := range corners {
					if values[index].Cmp(contact) == 0 {
						include(points[corner][0], points[corner][1])
					}
				}
				for edge := 0; edge < 4; edge++ {
					zero := true
					count := p.countV
					if edge >= 2 {
						count = p.countU
					}
					for i := 0; i < count; i++ {
						index := i
						if edge == 1 {
							index = (p.countU-1)*p.countV + i
						} else if edge == 2 {
							index = i * p.countV
						} else if edge == 3 {
							index = i*p.countV + p.countV - 1
						}
						if values[index].Cmp(contact) != 0 {
							zero = false
							break
						}
					}
					if zero {
						switch edge {
						case 0:
							include(u[0], v[0])
							include(u[0], v[1])
						case 1:
							include(u[1], v[0])
							include(u[1], v[1])
						case 2:
							include(u[0], v[0])
							include(u[1], v[0])
						case 3:
							include(u[0], v[1])
							include(u[1], v[1])
						}
					}
				}
				if !found {
					return ranges, true, 0, nil
				}
				ranges[base], ranges[base+1] = bound[0], bound[1]
			}
		}
		if ranges == before {
			break
		}
	}
	return ranges, false, 0, nil
}
