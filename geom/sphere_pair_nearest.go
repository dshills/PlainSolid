package geom

import (
	"context"
	"math/big"
)

// SphereNearestKind selects the one populated symbolic payload in SpherePairNearest.
type SphereNearestKind uint8

const (
	SphereNearestSeparatedUnion SphereNearestKind = iota + 1
	SphereNearestCavity
	SphereNearestTransverseIsolated
	SphereNearestTransverseAxis
	SphereNearestSingle
	SphereNearestTouchingCavity
)

// SpherePairNearest preserves the complete original-owner set and exact
// witnesses/families from one proved domain. Exactly one payload is nonnil.
// Bounds are squared Euclidean distance. Operations counts the single shared
// ledger, including exact dispatch; nested Operations has the same total.
type SpherePairNearest struct {
	Kind         SphereNearestKind
	Lower, Upper *big.Rat
	Owners       BoreBoundaryMask
	Operations   int
	Separated    *SphereSeparatedUnionNearest
	Cavity       *SphereCavityNearest
	Isolated     *SphereOverlapAlgebraicNearest
	Axis         *SphereVectorAxisNearest
	Single       *SphereSingleNearest
	Touching     *SphereTouchingCavityNearest
}

// NearestBoundaryRational selects one exact sphere-pair nearest proof without
// retries or sampled classification. Supported domains are separated-interior
// Union (including external tangency), strictly enclosed cavity Difference,
// and every transverse Union/Intersection/Difference with either isolated feet
// or complete axis families. Arbitrary represented center directions are valid.
// Containment, coincidence and disjoint/tangent subtraction that reduce to one
// sphere return Single. Coincident Union/Intersection retains both owners;
// discarded touching supports do not become owners. Empty regularized results
// return ErrDomain. Internally tangent cavities return an analytic Touching
// report without changing native manifold assembly rejection.
// All selection, candidate construction and comparison share one work/bit
// ledger. Every failure returns a zero result, never a partially proved payload.
func (s SpherePairMaterial) NearestBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions) (out SpherePairNearest, err error) {
	defer func() {
		if err != nil {
			out = SpherePairNearest{}
		}
	}()
	if ctx == nil || s.radius2[0] == nil || s.radius2[1] == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return
	}
	if o.SqrtBits == 0 {
		o.SqrtBits = 64
	}
	if o.SqrtBits < 8 || o.SqrtBits > 256 {
		return out, ErrInvalid
	}
	limits, e := patchOptions(o.Arithmetic)
	if e != nil {
		return out, e
	}
	budget := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := sphereOverlapArithmetic{ctx: ctx, b: budget}
	for _, v := range p {
		if v == nil {
			return out, ErrInvalid
		}
		a.check(v)
	}
	for i := 0; i < 2; i++ {
		a.check(s.radius2[i])
		for _, v := range s.center[i] {
			a.check(v)
		}
	}
	if a.err != nil {
		return out, a.err
	}
	if s.mode == 3 {
		return out, ErrDomain
	}
	if s.mode == 1 || s.mode == 2 || s.mode == 4 {
		q, e := s.nearestSingleSphereBoundaryRational(ctx, p, o, budget)
		if e != nil {
			return out, e
		}
		out.Kind = SphereNearestSingle
		out.Single = &q
		out.Lower, out.Upper, out.Owners = q.Lower, q.Upper, q.Owners
	} else if s.transverse {
		d, v := [3]*big.Rat{}, [3]*big.Rat{}
		for j := 0; j < 3; j++ {
			d[j] = a.sub(s.center[1][j], s.center[0][j])
			v[j] = a.sub(p[j], s.center[0][j])
		}
		axis := true
		for j := 0; j < 3; j++ {
			k, l := (j+1)%3, (j+2)%3
			cross := a.sub(a.mul(d[k], v[l]), a.mul(d[l], v[k]))
			if a.err != nil {
				return out, a.err
			}
			if cross.Sign() != 0 {
				axis = false
			}
		}
		if axis {
			q, e := s.nearestTransverseAxisBoundaryRational(ctx, p, o, budget)
			if e != nil {
				return out, e
			}
			out.Kind = SphereNearestTransverseAxis
			out.Axis = &q
			out.Lower, out.Upper, out.Owners = q.Lower, q.Upper, q.Owners
		} else {
			q, e := s.nearestOverlapAlgebraicBoundaryRational(ctx, p, o, true, budget)
			if e != nil {
				return out, e
			}
			out.Kind = SphereNearestTransverseIsolated
			out.Isolated = &q
			out.Lower, out.Upper, out.Owners = q.Lower, q.Upper, q.Owners
		}
	} else {
		switch s.op {
		case SphereSetUnion:
			q, e := s.nearestFullSphereBoundaryRational(ctx, p, o, false, false, budget)
			if e != nil {
				return out, e
			}
			out.Kind = SphereNearestSeparatedUnion
			out.Separated = &SphereSeparatedUnionNearest{Lower: q.Lower, Upper: q.Upper, Owners: q.Owners, Comparison: q.Comparison, Operations: q.Operations, Candidates: q.Candidates}
			out.Lower, out.Upper, out.Owners = q.Lower, q.Upper, q.Owners
		case SphereSetDifference:
			R, r := a.sqrt(s.radius2[0]), a.sqrt(s.radius2[1])
			gap := a.sub(R, r)
			limit := a.mul(gap, gap)
			D2 := a.distance(s.center[0], s.center[1])
			if a.err != nil {
				return out, a.err
			}
			if gap.Sign() > 0 && D2.Cmp(limit) == 0 {
				q, e := s.nearestTouchingCavityBoundaryRational(ctx, p, o, budget)
				if e != nil {
					return out, e
				}
				out.Kind = SphereNearestTouchingCavity
				out.Touching = &q
				out.Lower, out.Upper, out.Owners = q.Boundary.Lower, q.Boundary.Upper, q.Boundary.Owners
				break
			}
			q, e := s.nearestFullSphereBoundaryRational(ctx, p, o, true, false, budget)
			if e != nil {
				return out, e
			}
			out.Kind = SphereNearestCavity
			out.Cavity = &q
			out.Lower, out.Upper, out.Owners = q.Lower, q.Upper, q.Owners
		default:
			return out, ErrDomain
		}
	}
	if err = ctx.Err(); err != nil {
		return
	}
	out.Lower = new(big.Rat).Set(out.Lower)
	out.Upper = new(big.Rat).Set(out.Upper)
	out.Operations = budget.operations
	return out, nil
}
