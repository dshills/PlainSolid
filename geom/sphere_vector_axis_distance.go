package geom

import (
	"context"
	"math/big"
)

// SphereAxisAlgebraicPole denotes exactly Center+Numerator/sqrt(Radicand).
// Radicand is positive, Root encloses its square root, and no rounded point is
// substituted for this isolated original-sphere foot.
type SphereAxisAlgebraicPole struct {
	Center, Numerator [3]*big.Rat
	Radicand          *big.Rat
	Root              [2]*big.Rat
}

// SphereVectorAxisFamily is a complete minimum on one retained original cap
// (slots 0/1) or the common circle (slot 2). SourceOwner identifies that cap.
// Circle: |x-Center|²=RadiusSquared and dot(Normal,x-PlanePoint)=0.
// Cap: |x-Center|²=RadiusSquared and
// CapSide*dot(Normal,x-PlanePoint)>=0. Normal is exact and unnormalized.
// Owners refers to relative-interior points; RimOwners identifies the boundary
// of a cap. Pole is nonnil only for a singleton; non-singleton families have no
// representative point. Distance=Constant-Coefficient*sqrt(Radicand); rational
// distances use zero Coefficient and Radicand. Nearest is an exact global flag.
type SphereVectorAxisFamily struct {
	Kind                                          SphereAxisMinimumKind
	SourceOwner, Owners, RimOwners                BoreBoundaryMask
	Center, Normal, PlanePoint                    [3]*big.Rat
	RadiusSquared                                 *big.Rat
	CapSide                                       int
	Pole                                          *SphereAxisAlgebraicPole
	Constant, Coefficient, Radicand, Lower, Upper *big.Rat
	Nearest                                       bool
}

type SphereVectorAxisNearest struct {
	Lower, Upper        *big.Rat
	Owners              BoreBoundaryMask
	Families            [3]SphereVectorAxisFamily
	MaterialNormalSigns [2]int
	Operations          int
}

// NearestTransverseAxisBoundaryRational accepts exact rational queries on the
// arbitrary represented center axis of transverse sphere pairs. It returns
// complete retained-cap, circle and algebraic-pole families, with all original
// owners and exact tie ordering. Off-axis queries and nontransverse sources are
// rejected atomically. SqrtBits controls enclosures only, not topology/order.
// One arithmetic budget covers source proof, every family and comparisons.
func (s SpherePairMaterial) NearestTransverseAxisBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions) (out SphereVectorAxisNearest, err error) {
	return s.nearestTransverseAxisBoundaryRational(ctx, p, o, nil)
}

func (s SpherePairMaterial) nearestTransverseAxisBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions, shared *polynomialBudget) (out SphereVectorAxisNearest, err error) {
	defer func() {
		if err != nil {
			out = SphereVectorAxisNearest{}
		}
	}()
	if ctx == nil || s.radius2[0] == nil || s.radius2[1] == nil {
		return out, ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return
	}
	if !s.transverse {
		return out, ErrDomain
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
	budget := shared
	if budget == nil {
		budget = &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	}
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
	d := [3]*big.Rat{}
	D2 := a.distance(s.center[0], s.center[1])
	dot := new(big.Rat)
	for j := 0; j < 3; j++ {
		d[j] = a.sub(s.center[1][j], s.center[0][j])
		dot = a.add(dot, a.mul(a.sub(p[j], s.center[0][j]), d[j]))
	}
	lambda := a.div(dot, D2)
	for j := 0; j < 3; j++ {
		v := a.sub(a.sub(p[j], s.center[0][j]), a.mul(lambda, d[j]))
		if a.err != nil {
			return out, a.err
		}
		if v.Sign() != 0 {
			return out, ErrDomain
		}
	}
	fraction := a.div(a.add(a.sub(s.radius2[0], s.radius2[1]), D2), a.mul(big.NewRat(2, 1), D2))
	h2 := a.sub(s.radius2[0], a.mul(a.mul(fraction, fraction), D2))
	circleCenter := [3]*big.Rat{}
	for j := 0; j < 3; j++ {
		circleCenter[j] = a.add(s.center[0][j], a.mul(fraction, d[j]))
	}
	if a.err != nil {
		return out, a.err
	}
	if h2.Sign() <= 0 {
		return out, ErrDomain
	}
	for i := 0; i < 2; i++ {
		v := a.add(a.distance(circleCenter, s.center[i]), h2)
		if a.err != nil {
			return out, a.err
		}
		if v.Cmp(s.radius2[i]) != 0 {
			return out, ErrDomain
		}
	}
	rimDistance := a.add(a.distance(p, circleCenter), h2)
	family := func(source BoreBoundaryMask) SphereVectorAxisFamily {
		f := SphereVectorAxisFamily{Kind: SphereAxisSharedCircle, SourceOwner: source, Owners: 3, RadiusSquared: new(big.Rat).Set(h2), Constant: new(big.Rat).Set(rimDistance), Coefficient: new(big.Rat), Radicand: new(big.Rat)}
		for j := 0; j < 3; j++ {
			f.Center[j] = new(big.Rat).Set(circleCenter[j])
			f.PlanePoint[j] = new(big.Rat).Set(circleCenter[j])
			f.Normal[j] = new(big.Rat).Set(d[j])
		}
		return f
	}
	out.MaterialNormalSigns = [2]int{1, 1}
	if s.op == SphereSetDifference {
		out.MaterialNormalSigns[1] = -1
	}
	for i := 0; i < 2; i++ {
		owner := BoreBoundaryMask(1 << uint(i))
		S := a.distance(p, s.center[i])
		R := a.sqrt(s.radius2[i])
		if a.err != nil {
			return out, a.err
		}
		outside := s.op == SphereSetUnion || s.op == SphereSetDifference && i == 0
		if S.Sign() == 0 {
			f := family(owner)
			f.Kind = SphereAxisRetainedCap
			f.Owners = owner
			f.RimOwners = 3
			f.RadiusSquared = new(big.Rat).Set(s.radius2[i])
			f.Constant = new(big.Rat).Set(s.radius2[i])
			f.CapSide = 1
			if i == 1 {
				f.CapSide = -1
			}
			if outside {
				f.CapSide = -f.CapSide
			}
			for j := 0; j < 3; j++ {
				f.Center[j] = new(big.Rat).Set(s.center[i][j])
			}
			out.Families[i] = f
		} else {
			f := SphereVectorAxisFamily{Kind: SphereAxisPole, SourceOwner: owner, Owners: owner, Constant: a.add(S, s.radius2[i]), Coefficient: a.mul(big.NewRat(2, 1), R), Radicand: S, Pole: &SphereAxisAlgebraicPole{Radicand: new(big.Rat).Set(S)}}
			dot := new(big.Rat)
			for j := 0; j < 3; j++ {
				f.Pole.Center[j] = new(big.Rat).Set(s.center[i][j])
				v := a.sub(p[j], s.center[i][j])
				f.Pole.Numerator[j] = a.mul(R, v)
				dot = a.add(dot, a.mul(a.sub(s.center[i][j], s.center[1-i][j]), v))
			}
			C := a.add(D2, a.sub(s.radius2[i], s.radius2[1-i]))
			E := a.mul(f.Coefficient, dot)
			if a.err != nil {
				return out, a.err
			}
			sign, e := sphereCavityRadicalSign(E, new(big.Rat).Neg(C), S, a.b)
			if e != nil {
				return out, e
			}
			retained := sign >= 0
			if !outside {
				retained = sign <= 0
			}
			if retained {
				if sign == 0 {
					f.Owners = 3
				}
				out.Families[i] = f
			} else {
				out.Families[i] = family(owner)
			}
		}
	}
	out.Families[2] = family(0)
	if a.err != nil {
		return out, a.err
	}
	for i := range out.Families {
		f := &out.Families[i]
		if f.Coefficient.Sign() == 0 {
			f.Lower = new(big.Rat).Set(f.Constant)
			f.Upper = new(big.Rat).Set(f.Constant)
		} else {
			lo, hi, e := rationalSqrtBounds(f.Radicand, o.SqrtBits, a.b)
			if e != nil {
				return out, e
			}
			f.Pole.Root = [2]*big.Rat{lo, hi}
			f.Lower = a.sub(f.Constant, a.mul(f.Coefficient, hi))
			f.Upper = a.sub(f.Constant, a.mul(f.Coefficient, lo))
			if f.Lower.Sign() < 0 {
				f.Lower = new(big.Rat)
			}
		}
	}
	if a.err != nil {
		return out, a.err
	}
	compare := func(i, j int) (int, error) {
		x, y := out.Families[i], out.Families[j]
		v := a.sub(x.Constant, y.Constant)
		if a.err != nil {
			return 0, a.err
		}
		if x.Coefficient.Sign() == 0 {
			if y.Coefficient.Sign() == 0 {
				return v.Sign(), nil
			}
			return sphereCavityRadicalSign(v, new(big.Rat).Neg(y.Coefficient), y.Radicand, a.b)
		}
		if y.Coefficient.Sign() == 0 {
			return sphereCavityRadicalSign(v, x.Coefficient, x.Radicand, a.b)
		}
		return sphereCavityCompare(v, x.Coefficient, x.Radicand, y.Coefficient, y.Radicand, a.b)
	}
	best := 2
	for i := 0; i < 2; i++ {
		c, e := compare(i, best)
		if e != nil {
			return out, e
		}
		if c < 0 {
			best = i
		}
	}
	for i := range out.Families {
		c, e := compare(i, best)
		if e != nil {
			return out, e
		}
		if c == 0 {
			f := &out.Families[i]
			f.Nearest = true
			out.Owners |= f.Owners | f.RimOwners
		}
	}
	out.Lower = new(big.Rat).Set(out.Families[best].Lower)
	out.Upper = new(big.Rat).Set(out.Families[best].Upper)
	if err = ctx.Err(); err != nil {
		return
	}
	out.Operations = a.b.operations
	return out, nil
}
