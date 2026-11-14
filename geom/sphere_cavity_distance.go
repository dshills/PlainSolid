package geom

import (
	"context"
	"math/big"
)

// SphereCavityDistanceOptions bounds all arithmetic in one query. SqrtBits
// controls only outward witness/distance enclosures, not exact owner ordering.
// Zero selects64; range8..256. Arithmetic uses standard patch work/bit limits.
type SphereCavityDistanceOptions struct {
	Arithmetic SurfacePatchOptions
	SqrtBits   int
}

// SphereCavityDistanceWitness describes a full retained original sphere's
// attained minimum. Unless WholeSphere, Point=Center+Numerator/sqrt(Radicand).
// At its center, WholeSphere explicitly denotes every point of that sphere;
// the division formula is then undefined and must not be used. Material unit
// normal is MaterialNormalSign*(Point-Center)/Radius: inner cavity normals point
// inward toward its center. A candidate is globally nearest only if Owner is
// included in SphereCavityNearest.Owners.
type SphereCavityDistanceWitness struct {
	Center, Numerator  [3]*big.Rat
	Radius, Radicand   *big.Rat
	Root               [2]*big.Rat
	Owner              BoreBoundaryMask
	MaterialNormalSign int
	WholeSphere        bool
}

// SphereCavityNearest completely classifies the two original support owners.
// Comparison is sign(D_first-D_second); equality proves both attained minima,
// not equality of numerical enclosures. Squared bounds refer to the global
// boundary distance. No mesh or tolerance-based containment is used.
type SphereCavityNearest struct {
	Lower, Upper           *big.Rat
	Owners                 BoreBoundaryMask
	Comparison, Operations int
	Candidates             [2]SphereCavityDistanceWitness
}

func sphereCavityRadicalSign(a, b, d *big.Rat, budget *polynomialBudget) (int, error) {
	for _, v := range []*big.Rat{a, b, d} {
		if e := budget.check(v); e != nil {
			return 0, e
		}
	}
	if b.Sign() == 0 || d.Sign() == 0 {
		return a.Sign(), nil
	}
	if a.Sign() == 0 {
		return -b.Sign(), nil
	}
	if a.Sign() != b.Sign() {
		return a.Sign(), nil
	}
	aa := new(big.Rat).Mul(a, a)
	if e := budget.check(aa); e != nil {
		return 0, e
	}
	bb := new(big.Rat).Mul(b, b)
	if e := budget.check(bb); e != nil {
		return 0, e
	}
	bb.Mul(bb, d)
	if e := budget.check(bb); e != nil {
		return 0, e
	}
	sign := aa.Cmp(bb)
	if a.Sign() < 0 {
		sign = -sign
	}
	return sign, nil
}

// Sign of A-B√S+C√T for B,C>0 and S,T>=0, with guarded squaring.
func sphereCavityCompare(a, b, s, c, t *big.Rat, budget *polynomialBudget) (int, error) {
	left, e := sphereCavityRadicalSign(a, new(big.Rat).Neg(c), t, budget)
	if e != nil {
		return 0, e
	}
	if s.Sign() == 0 {
		return left, nil
	}
	if left <= 0 {
		return -1, nil
	}
	aa := new(big.Rat).Mul(a, a)
	if e = budget.check(aa); e != nil {
		return 0, e
	}
	ct := new(big.Rat).Mul(c, c)
	if e = budget.check(ct); e != nil {
		return 0, e
	}
	ct.Mul(ct, t)
	if e = budget.check(ct); e != nil {
		return 0, e
	}
	bs := new(big.Rat).Mul(b, b)
	if e = budget.check(bs); e != nil {
		return 0, e
	}
	bs.Mul(bs, s)
	if e = budget.check(bs); e != nil {
		return 0, e
	}
	aa.Add(aa, ct)
	if e = budget.check(aa); e != nil {
		return 0, e
	}
	aa.Sub(aa, bs)
	if e = budget.check(aa); e != nil {
		return 0, e
	}
	coefficient := new(big.Rat).Mul(a, c)
	if e = budget.check(coefficient); e != nil {
		return 0, e
	}
	coefficient.Mul(coefficient, big.NewRat(-2, 1))
	if e = budget.check(coefficient); e != nil {
		return 0, e
	}
	return sphereCavityRadicalSign(aa, coefficient, t, budget)
}

// NearestCavityBoundaryRational supports precisely A-minus-B with the closed
// inner ball STRICTLY inside the outer ball. It re-proves exact containment;
// tangent, transverse, empty and other operations return ErrDomain. Both full
// sphere surfaces are retained, so each radial projection (or center family)
// is attained. One shared arithmetic budget includes containment, witnesses,
// enclosures and exact two-radical owner comparison. Every failure is atomic.
func (s SpherePairMaterial) NearestCavityBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereCavityDistanceOptions) (out SphereCavityNearest, err error) {
	return s.nearestFullSphereBoundaryRational(ctx, p, o, true, nil)
}

func (s SpherePairMaterial) nearestFullSphereBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereCavityDistanceOptions, cavity bool, shared *polynomialBudget) (out SphereCavityNearest, err error) {
	defer func() {
		if err != nil {
			out = SphereCavityNearest{}
		}
	}()
	if ctx == nil || s.radius2[0] == nil || s.radius2[1] == nil {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if cavity && s.op != SphereSetDifference || !cavity && s.op != SphereSetUnion {
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
	for _, v := range p {
		if v == nil {
			return out, ErrInvalid
		}
		if e = budget.check(v); e != nil {
			return out, e
		}
	}
	var radii [2]*big.Rat
	for i := 0; i < 2; i++ {
		lo, hi, e := rationalSqrtBounds(s.radius2[i], o.SqrtBits, budget)
		if e != nil {
			return out, e
		}
		if lo.Sign() <= 0 || lo.Cmp(hi) != 0 {
			return out, ErrDomain
		}
		radii[i] = lo
	}
	gap := new(big.Rat)
	if cavity {
		gap.Sub(radii[0], radii[1])
	} else {
		gap.Add(radii[0], radii[1])
	}
	if e = budget.check(gap); e != nil {
		return out, e
	}
	if gap.Sign() <= 0 {
		return out, ErrDomain
	}
	gap.Mul(gap, gap)
	if e = budget.check(gap); e != nil {
		return out, e
	}
	centerD := new(big.Rat)
	for j := 0; j < 3; j++ {
		v := new(big.Rat).Sub(s.center[0][j], s.center[1][j])
		if e = budget.check(v); e != nil {
			return out, e
		}
		v.Mul(v, v)
		if e = budget.check(v); e != nil {
			return out, e
		}
		centerD.Add(centerD, v)
		if e = budget.check(centerD); e != nil {
			return out, e
		}
	}
	if cavity && centerD.Cmp(gap) >= 0 || !cavity && centerD.Cmp(gap) < 0 {
		return out, ErrDomain
	}
	var S, loD, hiD [2]*big.Rat
	for i := 0; i < 2; i++ {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		w := SphereCavityDistanceWitness{Radius: new(big.Rat).Set(radii[i]), Owner: 1 << uint(i), MaterialNormalSign: 1}
		if cavity && i == 1 {
			w.MaterialNormalSign = -1
		}
		S[i] = new(big.Rat)
		for j := 0; j < 3; j++ {
			w.Center[j] = new(big.Rat).Set(s.center[i][j])
			d := new(big.Rat).Sub(p[j], s.center[i][j])
			if e = budget.check(d); e != nil {
				return out, e
			}
			sq := new(big.Rat).Mul(d, d)
			if e = budget.check(sq); e != nil {
				return out, e
			}
			S[i].Add(S[i], sq)
			if e = budget.check(S[i]); e != nil {
				return out, e
			}
			w.Numerator[j] = new(big.Rat).Mul(radii[i], d)
			if e = budget.check(w.Numerator[j]); e != nil {
				return out, e
			}
		}
		rl, rh, e := rationalSqrtBounds(S[i], o.SqrtBits, budget)
		if e != nil {
			return out, e
		}
		w.Radicand = new(big.Rat).Set(S[i])
		w.Root = [2]*big.Rat{rl, rh}
		w.WholeSphere = S[i].Sign() == 0
		out.Candidates[i] = w
		a, b := new(big.Rat).Sub(rl, radii[i]), new(big.Rat).Sub(rh, radii[i])
		for _, v := range []*big.Rat{a, b} {
			if e = budget.check(v); e != nil {
				return out, e
			}
		}
		lo, hi := new(big.Rat).Mul(a, a), new(big.Rat).Mul(b, b)
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if a.Sign() <= 0 && b.Sign() >= 0 {
			lo = new(big.Rat)
		}
		for _, v := range []*big.Rat{lo, hi} {
			if e = budget.check(v); e != nil {
				return out, e
			}
		}
		loD[i], hiD[i] = lo, hi
	}
	a := new(big.Rat).Add(S[0], s.radius2[0])
	if e = budget.check(a); e != nil {
		return out, e
	}
	a.Sub(a, S[1])
	if e = budget.check(a); e != nil {
		return out, e
	}
	a.Sub(a, s.radius2[1])
	if e = budget.check(a); e != nil {
		return out, e
	}
	b, c := new(big.Rat).Mul(big.NewRat(2, 1), radii[0]), new(big.Rat).Mul(big.NewRat(2, 1), radii[1])
	for _, v := range []*big.Rat{b, c} {
		if e = budget.check(v); e != nil {
			return out, e
		}
	}
	comparison, e := sphereCavityCompare(a, b, S[0], c, S[1], budget)
	if e != nil {
		return out, e
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	selected := 0
	out.Owners = SpherePairFirst
	if comparison > 0 {
		selected = 1
		out.Owners = SpherePairSecond
	} else if comparison == 0 {
		out.Owners = SpherePairFirst | SpherePairSecond
	}
	out.Comparison = comparison
	out.Lower = new(big.Rat).Set(loD[selected])
	out.Upper = new(big.Rat).Set(hiD[selected])
	out.Operations = budget.operations
	return out, nil
}
