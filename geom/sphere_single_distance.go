package geom

import (
	"context"
	"math/big"
	"runtime"
)

// SphereSingleNearest is the complete minimum on the one retained full sphere.
// Coincident Union/Intersection retains both original owners. Removed/redundant
// operands are in ExcludedOwners even if their support touches this sphere:
// regularized ownership follows SpherePairMaterial.Classify, not raw contact.
// Witness.WholeSphere explicitly preserves the entire center minimizer family.
type SphereSingleNearest struct {
	Lower, Upper           *big.Rat
	Owners, ExcludedOwners BoreBoundaryMask
	Witness                SphereFullBoundaryWitness
	Operations             int
}

func (s SpherePairMaterial) nearestSingleSphereBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions, budget *polynomialBudget) (out SphereSingleNearest, err error) {
	defer func() {
		if err != nil {
			out = SphereSingleNearest{}
		}
	}()
	a := sphereOverlapArithmetic{ctx: ctx, b: budget}
	R := [2]*big.Rat{a.sqrt(s.radius2[0]), a.sqrt(s.radius2[1])}
	D2 := a.distance(s.center[0], s.center[1])
	if a.err != nil {
		return out, a.err
	}
	index := 0
	owners := SpherePairFirst
	switch s.mode {
	case 4:
		if D2.Sign() != 0 || R[0].Cmp(R[1]) != 0 || s.op == SphereSetDifference {
			return out, ErrDomain
		}
		owners = 3
	case 1, 2:
		index = s.mode - 1
		owners = 1 << uint(index)
		if s.op == SphereSetDifference {
			sum := a.add(R[0], R[1])
			limit := a.mul(sum, sum)
			if a.err != nil {
				return out, a.err
			}
			if index != 0 || D2.Cmp(limit) < 0 {
				return out, ErrDomain
			}
		} else {
			outer, inner := index, 1-index
			if s.op == SphereSetIntersection {
				outer, inner = inner, outer
			}
			gap := a.sub(R[outer], R[inner])
			limit := a.mul(gap, gap)
			if a.err != nil {
				return out, a.err
			}
			if gap.Sign() <= 0 || D2.Cmp(limit) > 0 {
				return out, ErrDomain
			}
		}
	default:
		return out, ErrDomain
	}
	w := SphereFullBoundaryWitness{Radius: new(big.Rat).Set(R[index]), Owner: owners, MaterialNormalSign: 1}
	S := a.distance(p, s.center[index])
	for j := 0; j < 3; j++ {
		w.Center[j] = new(big.Rat).Set(s.center[index][j])
		w.Numerator[j] = a.mul(R[index], a.sub(p[j], s.center[index][j]))
	}
	if a.err != nil {
		return out, a.err
	}
	lo, hi, e := rationalSqrtBounds(S, o.SqrtBits, budget)
	if e != nil {
		return out, e
	}
	w.Radicand = new(big.Rat).Set(S)
	w.Root = [2]*big.Rat{lo, hi}
	w.WholeSphere = S.Sign() == 0
	constant := a.add(S, s.radius2[index])
	coefficient := a.mul(big.NewRat(2, 1), R[index])
	runtime.KeepAlive(&R) // golang/go#82026: keeps R in the stack map. Remove once the Go fix is in use.
	out.Lower = a.sub(constant, a.mul(coefficient, hi))
	out.Upper = a.sub(constant, a.mul(coefficient, lo))
	if a.err != nil {
		return out, a.err
	}
	if out.Lower.Sign() < 0 {
		out.Lower = new(big.Rat)
	}
	out.Owners = owners
	out.ExcludedOwners = 3 &^ owners
	out.Witness = w
	if err = ctx.Err(); err != nil {
		return
	}
	out.Operations = budget.operations
	return out, nil
}
