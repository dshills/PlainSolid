package geom

import (
	"math/big"
	"sort"
)

type exactKnotBlend struct {
	left, right, denominator big.Int
}

type exactKnotInsertion struct {
	first int
	blend []exactKnotBlend
}

// The knot clock is shared by every strip of a tensor control net.
func exactKnotRefinement(knots []float64, degree int, t float64, count int) []exactKnotInsertion {
	steps := make([]exactKnotInsertion, count)
	cut := new(big.Rat).SetFloat64(t)
	for insertion := range steps {
		first := sort.SearchFloat64s(knots, t)
		k := sort.Search(len(knots), func(i int) bool { return knots[i] > t }) - 1
		start := k - degree + 1
		step := exactKnotInsertion{first: start, blend: make([]exactKnotBlend, first-start)}
		var knot, alpha, width big.Rat
		for j := range step.blend {
			i := start + j
			knot.SetFloat64(knots[i])
			alpha.Sub(cut, &knot)
			width.Sub(width.SetFloat64(knots[i+degree]), &knot)
			alpha.Quo(&alpha, &width)
			weight := &step.blend[j]
			weight.right.Set(alpha.Num())
			weight.denominator.Set(alpha.Denom())
			weight.left.Sub(&weight.denominator, &weight.right)
		}
		steps[insertion] = step
		knots = insertionKnots(knots, t, 1)
	}
	return steps
}

func applyExactKnotRefinement(net []exactHomogeneous, steps []exactKnotInsertion, budget *polynomialBudget) ([]exactHomogeneous, error) {
	var left, right, denominator big.Int
	for _, step := range steps {
		next := make([]exactHomogeneous, len(net)+1)
		end := step.first + len(step.blend)
		copy(next[:step.first], net[:step.first])
		copy(next[end:], net[end-1:])
		for j := range step.blend {
			i := step.first + j
			weight := &step.blend[j]
			for axis := range next[i] {
				a, b := net[i-1][axis], net[i][axis]
				left.Mul(&weight.left, a.Num())
				right.Mul(&weight.right, b.Num())
				// Blend in the common denominator and reduce once, after
				// the whole exact sum. The old budget also checks that sum.
				if a.Denom().Cmp(b.Denom()) == 0 {
					denominator.Mul(&weight.denominator, a.Denom())
				} else {
					left.Mul(&left, b.Denom())
					right.Mul(&right, a.Denom())
					denominator.Mul(a.Denom(), b.Denom())
					denominator.Mul(&denominator, &weight.denominator)
				}
				left.Add(&left, &right)
				value := new(big.Rat).SetFrac(&left, &denominator)
				if e := budget.check(value); e != nil {
					return nil, e
				}
				next[i][axis] = value
			}
		}
		net = next
	}
	return net, nil
}
