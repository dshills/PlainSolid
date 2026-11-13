package geom

import (
	"math/big"
	"sort"
)

type exactHomogeneous [4]*big.Rat

func centeredExactNet(curve *BSpline, origin Vec3, scale *big.Rat, budget *polynomialBudget) ([]exactHomogeneous, error) {
	net := make([]exactHomogeneous, len(curve.poles))
	for i, p := range curve.poles {
		weight := new(big.Rat).Mul(new(big.Rat).SetFloat64(curve.weights[i]), scale)
		net[i][3] = weight
		for axis := 0; axis < 3; axis++ {
			difference := new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(p, axis)), new(big.Rat).SetFloat64(coordinate(origin, axis)))
			net[i][axis] = new(big.Rat).Mul(weight, difference)
			if e := budget.check(net[i][axis]); e != nil {
				return nil, e
			}
		}
		if e := budget.check(weight); e != nil {
			return nil, e
		}
	}
	return net, nil
}
func refineExactNet(net []exactHomogeneous, knots []float64, degree int, t float64, count int, budget *polynomialBudget) ([]exactHomogeneous, error) {
	u := append([]float64(nil), knots...)
	for insertion := 0; insertion < count; insertion++ {
		first := sort.SearchFloat64s(u, t)
		k := sort.Search(len(u), func(i int) bool { return u[i] > t }) - 1
		s := k - first + 1
		next := make([]exactHomogeneous, len(net)+1)
		for i := 0; i <= k-degree; i++ {
			next[i] = net[i]
		}
		for i := k - s; i < len(net); i++ {
			next[i+1] = net[i]
		}
		for i := k - degree + 1; i <= k-s; i++ {
			numerator := new(big.Rat).Sub(new(big.Rat).SetFloat64(t), new(big.Rat).SetFloat64(u[i]))
			denominator := new(big.Rat).Sub(new(big.Rat).SetFloat64(u[i+degree]), new(big.Rat).SetFloat64(u[i]))
			alpha := new(big.Rat).Quo(numerator, denominator)
			complement := new(big.Rat).Sub(big.NewRat(1, 1), alpha)
			for axis := 0; axis < 4; axis++ {
				value := new(big.Rat).Mul(complement, net[i-1][axis])
				value.Add(value, new(big.Rat).Mul(alpha, net[i][axis]))
				if e := budget.check(value); e != nil {
					return nil, e
				}
				next[i][axis] = value
			}
		}
		knots := make([]float64, len(u)+1)
		copy(knots, u[:k+1])
		knots[k+1] = t
		copy(knots[k+2:], u[k+1:])
		u, net = knots, next
	}
	return net, nil
}

// removalGlobalBound proves a global Euclidean error bound for stored float64
// geometry using exact rational control-net arithmetic. L1 norms conservatively
// bound Euclidean norms and avoid an uncertified floating square root.
//
// In the common original basis let A/W and B/V be centered rational curves.
// P-Q = ((A-B)+(V-W)*(Q-origin))/W. Nonnegative partition-of-unity basis
// functions bound ||A-B|| by the largest control numerator L1 difference,
// |V-W| by the largest weight difference, W below by the smallest original
// weight, and ||Q-origin|| by the candidate's positive-weight control hull.
// Thus (maxDeltaA + maxDeltaW*maxCandidateRadius)/minOriginalWeight bounds
// every parameter, including both one-sided limits at discontinuous knots.
func removalGlobalBound(original, candidate *BSpline, t float64, count int, scale float64, budget *polynomialBudget) (bound, homogeneousError, weightError float64, err error) {
	origin := original.poles[0]
	a, e := centeredExactNet(original, origin, big.NewRat(1, 1), budget)
	if e != nil {
		return 0, 0, 0, e
	}
	b, e := centeredExactNet(candidate, origin, new(big.Rat).SetFloat64(scale), budget)
	if e != nil {
		return 0, 0, 0, e
	}
	b, e = refineExactNet(b, candidate.knots, candidate.degree, t, count, budget)
	if e != nil {
		return 0, 0, 0, e
	}
	if len(a) != len(b) {
		return 0, 0, 0, ErrInvalid
	}
	maximumNumerator, maximumWeight := new(big.Rat), new(big.Rat)
	minimumWeight := new(big.Rat).Set(a[0][3])
	for i := range a {
		sum := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			difference := new(big.Rat).Abs(new(big.Rat).Sub(a[i][axis], b[i][axis]))
			sum.Add(sum, difference)
		}
		if sum.Cmp(maximumNumerator) > 0 {
			maximumNumerator.Set(sum)
		}
		difference := new(big.Rat).Abs(new(big.Rat).Sub(a[i][3], b[i][3]))
		if difference.Cmp(maximumWeight) > 0 {
			maximumWeight.Set(difference)
		}
		if a[i][3].Cmp(minimumWeight) < 0 {
			minimumWeight.Set(a[i][3])
		}
		if e := budget.check(sum); e != nil {
			return 0, 0, 0, e
		}
		if e := budget.check(difference); e != nil {
			return 0, 0, 0, e
		}
	}
	hullRadius := new(big.Rat)
	for _, pole := range candidate.poles {
		sum := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			difference := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).SetFloat64(coordinate(pole, axis)), new(big.Rat).SetFloat64(coordinate(origin, axis))))
			sum.Add(sum, difference)
		}
		if sum.Cmp(hullRadius) > 0 {
			hullRadius.Set(sum)
		}
		if e := budget.check(sum); e != nil {
			return 0, 0, 0, e
		}
	}
	errorBound := new(big.Rat).Add(maximumNumerator, new(big.Rat).Mul(hullRadius, maximumWeight))
	errorBound.Quo(errorBound, minimumWeight)
	if e := budget.check(errorBound); e != nil {
		return 0, 0, 0, e
	}
	return rationalFloatBound(errorBound, false), rationalFloatBound(maximumNumerator, false), rationalFloatBound(maximumWeight, false), nil
}
