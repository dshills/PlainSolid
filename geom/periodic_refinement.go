package geom

import (
	"context"
	"math/big"
)

// RefineUniformKnots bisects every interval of the original cyclic knot
// lattice at each level. The original degree, physical start and period
// are preserved. Homogeneous subdivision is exact. Unrepresentable
// resulting poles or weights return ErrUnrepresentable and no curve.
func (c *PeriodicBSpline) RefineUniformKnots(ctx context.Context, levels int, o SurfacePatchOptions) (result *PeriodicBSpline, operations int, err error) {
	if ctx == nil || c == nil || c.curve == nil {
		return nil, 0, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	counts := [2]int{len(c.poles), 1}
	levelsByAxis := [2]int{levels, 0}
	degrees := [2]int{c.degree, 0}
	limits, want, err := periodicRefinementLimits(o, counts, levelsByAxis)
	if err != nil {
		return nil, 0, err
	}
	if _, err := uniformPeriodicEnd(c.degree, want[0], c.start, c.period); err != nil {
		return nil, 0, ErrUnrepresentable
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	net, err := periodicRefinementNet(ctx, c.poles, c.weights, b)
	if err != nil {
		return nil, 0, err
	}
	net, err = refineUniformPeriodicNet(ctx, net, counts, degrees, levelsByAxis, b)
	if err != nil {
		return nil, 0, err
	}
	poles, weights, err := exportPeriodicRefinement(ctx, net, b)
	if err != nil {
		return nil, 0, err
	}
	result, err = NewPeriodicBSplineWithExactWeights(c.degree, poles, weights, c.start, c.period)
	return result, 0, err
}

// RefineUniformKnots bisects the selected cyclic axes of a tensor surface.
// A noncyclic axis requires zero levels and retains its original knot
// vector, including any repeated ordinary knots. Both directions share
// the same exact operation and pole budgets.
func (s *PeriodicBSplineSurface) RefineUniformKnots(ctx context.Context, levelsU, levelsV int,
	o SurfacePatchOptions) (result *PeriodicBSplineSurface, operations int, err error) {
	if ctx == nil || s == nil || s.surface == nil {
		return nil, 0, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	levels := [2]int{levelsU, levelsV}
	for axis, n := range levels {
		if n > 0 && s.axis[axis].Period == 0 {
			return nil, 0, ErrDomain
		}
	}
	limits, want, err := periodicRefinementLimits(o, s.count, levels)
	if err != nil {
		return nil, 0, err
	}
	degrees := [2]int{s.surface.degreeU, s.surface.degreeV}
	for axis, a := range s.axis {
		if a.Period > 0 {
			if _, err := uniformPeriodicEnd(degrees[axis], want[axis], a.Start, a.Period); err != nil {
				return nil, 0, ErrUnrepresentable
			}
		}
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { operations = b.operations }()
	data, u, v := s.Data()
	net, err := periodicRefinementNet(ctx, data.Poles, data.Weights, b)
	if err != nil {
		return nil, 0, err
	}
	net, err = refineUniformPeriodicNet(ctx, net, s.count, degrees, levels, b)
	if err != nil {
		return nil, 0, err
	}
	data.CountU, data.CountV = want[0], want[1]
	data.Poles, data.Weights, err = exportPeriodicRefinement(ctx, net, b)
	if err != nil {
		return nil, 0, err
	}
	result, err = NewPeriodicBSplineSurface(data, u, v)
	return result, 0, err
}

func periodicRefinementLimits(o SurfacePatchOptions, counts, levels [2]int) (SurfacePatchOptions, [2]int, error) {
	o, err := patchOptions(o)
	if err != nil {
		return o, counts, err
	}
	if levels[0] < 0 || levels[1] < 0 {
		return o, counts, ErrInvalid
	}
	// Four exact values are inspected per output control even without
	// any subdivision, so the work limit also bounds allocation.
	maximum := min(o.MaxPoles, o.MaxOperations/4)
	if counts[0] > maximum/counts[1] {
		return o, counts, ErrConvergence
	}
	for axis, n := range levels {
		for range n {
			if counts[axis] > maximum/counts[axis^1]/2 {
				return o, counts, ErrConvergence
			}
			counts[axis] *= 2
		}
	}
	return o, counts, nil
}

func periodicRefinementNet(ctx context.Context, poles []Vec3, weights []float64, b *polynomialBudget) ([]exactHomogeneous, error) {
	net := make([]exactHomogeneous, len(poles))
	for i, p := range poles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		w := new(big.Rat).SetFloat64(weights[i])
		net[i][3] = w
		for axis := 0; axis < 3; axis++ {
			net[i][axis] = new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(p, axis)), w)
		}
		for _, q := range net[i] {
			if err := b.check(q); err != nil {
				return nil, err
			}
		}
	}
	return net, nil
}

func refineUniformPeriodicNet(ctx context.Context, net []exactHomogeneous,
	counts, degrees, levels [2]int, b *polynomialBudget) ([]exactHomogeneous, error) {
	for axis, n := range levels {
		if n == 0 {
			continue
		}
		p := degrees[axis]
		denom := new(big.Int).Lsh(big.NewInt(1), uint(p))
		mask := make([]*big.Rat, p+2)
		for k := range mask {
			mask[k] = new(big.Rat).SetFrac(new(big.Int).Binomial(int64(p+1), int64(k)), denom)
			if err := b.check(mask[k]); err != nil {
				return nil, err
			}
		}
		for range n {
			fine := counts
			fine[axis] *= 2
			// Charge the minimum future work before allocating the next net.
			if len(net) > (b.maxOperations-b.operations)/8 {
				return nil, ErrConvergence
			}
			out := make([]exactHomogeneous, len(net)*2)
			for i, h := range net {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				ij := [2]int{i / counts[1], i % counts[1]}
				for k, q := range mask {
					// Cardinal B-spline refinement: basis i has
					// start i-p, so new basis starts at 2*(i-p)+k.
					j := (2*ij[axis] - p + k) % fine[axis]
					if j < 0 {
						j += fine[axis]
					}
					dst := ij
					dst[axis] = j
					slot := &out[dst[0]*fine[1]+dst[1]]
					for component, v := range h {
						term := new(big.Rat).Mul(q, v)
						if err := b.check(term); err != nil {
							return nil, err
						}
						if (*slot)[component] == nil {
							(*slot)[component] = term
						} else {
							(*slot)[component].Add((*slot)[component], term)
							if err := b.check((*slot)[component]); err != nil {
								return nil, err
							}
						}
					}
				}
			}
			net, counts = out, fine
		}
	}
	return net, nil
}

func exportPeriodicRefinement(ctx context.Context, net []exactHomogeneous, b *polynomialBudget) ([]Vec3, []float64, error) {
	poles, weights := make([]Vec3, len(net)), make([]float64, len(net))
	for i, h := range net {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		weight, err := RepresentedExactScalar(h[3])
		if err != nil {
			return nil, nil, err
		}
		weights[i] = weight
		var xyz [3]float64
		for j := range xyz {
			q := new(big.Rat).Quo(h[j], h[3])
			if err := b.check(q); err != nil {
				return nil, nil, err
			}
			xyz[j], err = RepresentedExactScalar(q)
			if err != nil {
				return nil, nil, err
			}
		}
		poles[i] = Vec3{xyz[0], xyz[1], xyz[2]}
	}
	return poles, weights, nil
}
