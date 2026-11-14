package geom

import (
	"context"
	"math/big"
)

// SphereOverlapAlgebraicCandidate gives Point=Center+Numerator/sqrt(Radicand).
// Radicand is strictly positive; Root brackets its square root. Squared distance
// equals Constant-Coefficient*sqrt(Radicand), with Coefficient positive. Center
// is the original sphere center for slots 0/1 and the circle center for slot 2.
// A buried radial point remains diagnostic only (Retained=false). Nearest is
// established by exact radical order, not by overlapping numerical bounds.
type SphereOverlapAlgebraicCandidate struct {
	Center, Numerator               [3]*big.Rat
	Radicand, Constant, Coefficient *big.Rat
	Root                            [2]*big.Rat
	Lower, Upper                    *big.Rat
	Owners                          BoreBoundaryMask
	Retained, Nearest               bool
}

// SphereOverlapAlgebraicNearest retains stable source A/B/circle candidate order
// and all exact nearest original owners. Several slots may describe the same
// point. MaterialNormalSigns refer to original sphere radial normals, including
// the inward cutter normal for Difference; no single normal is claimed at a rim.
type SphereOverlapAlgebraicNearest struct {
	Lower, Upper        *big.Rat
	Owners              BoreBoundaryMask
	Candidates          [3]SphereOverlapAlgebraicCandidate
	MaterialNormalSigns [2]int
	Operations          int
}

// NearestOverlapAlgebraicBoundaryRational extends the rational-foot query to
// irrational query norms. It still requires transverse coordinate-axis centers
// and an exact rational common-circle radius, and rejects axis/center families.
// Retained-cap signs and every candidate comparison are exact guarded radical
// comparisons; SqrtBits affects only reported enclosures. Each retained cap has
// its minimum at its full radial foot or on the shared circle, whose minimum is
// unique off-axis. Thus the returned owner set is complete in this subset.
// One work/bit budget covers source checks, witnesses, bounds and ordering.
func (s SpherePairMaterial) NearestOverlapAlgebraicBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions) (out SphereOverlapAlgebraicNearest, err error) {
	return s.nearestOverlapAlgebraicBoundaryRational(ctx, p, o, false, nil)
}

func (s SpherePairMaterial) nearestOverlapAlgebraicBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions, general bool, shared *polynomialBudget) (out SphereOverlapAlgebraicNearest, err error) {
	defer func() {
		if err != nil {
			out = SphereOverlapAlgebraicNearest{}
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
	axis := -1
	for j := 0; j < 3; j++ {
		for i := 0; i < 2; i++ {
			a.check(s.center[i][j])
		}
		if s.center[0][j].Cmp(s.center[1][j]) != 0 {
			if axis >= 0 && !general {
				return out, ErrDomain
			}
			axis = j
		}
	}
	if axis < 0 {
		return out, ErrDomain
	}
	for _, v := range s.radius2 {
		a.check(v)
	}

	var h2, h, axial2 *big.Rat
	center, radial := [3]*big.Rat{}, [3]*big.Rat{}
	rho2 := new(big.Rat)
	if general {
		axisVector := [3]*big.Rat{}
		D2 := a.distance(s.center[0], s.center[1])
		fraction := a.div(a.add(a.sub(s.radius2[0], s.radius2[1]), D2), a.mul(big.NewRat(2, 1), D2))
		h2 = a.sub(s.radius2[0], a.mul(a.mul(fraction, fraction), D2))
		dot := new(big.Rat)
		for j := 0; j < 3; j++ {
			axisVector[j] = a.sub(s.center[1][j], s.center[0][j])
			center[j] = a.add(s.center[0][j], a.mul(fraction, axisVector[j]))
			dot = a.add(dot, a.mul(a.sub(p[j], center[j]), axisVector[j]))
		}
		projection := a.div(dot, D2)
		axial2 = a.div(a.mul(dot, dot), D2)
		orthogonal := new(big.Rat)
		for j := 0; j < 3; j++ {
			radial[j] = a.sub(a.sub(p[j], center[j]), a.mul(projection, axisVector[j]))
			rho2 = a.add(rho2, a.mul(radial[j], radial[j]))
			orthogonal = a.add(orthogonal, a.mul(radial[j], axisVector[j]))
		}
		if a.err != nil {
			return out, a.err
		}
		if h2.Sign() <= 0 || orthogonal.Sign() != 0 {
			return out, ErrDomain
		}
	} else {
		d := a.sub(s.center[1][axis], s.center[0][axis])
		t := a.div(a.add(a.sub(s.radius2[0], s.radius2[1]), a.mul(d, d)), a.mul(big.NewRat(2, 1), d))
		h2 = a.sub(s.radius2[0], a.mul(t, t))
		h = a.sqrt(h2)

		for j := 0; j < 3; j++ {
			center[j] = new(big.Rat).Set(s.center[0][j])
			if j == axis {
				center[j] = a.add(center[j], t)
			} else {
				v := a.sub(p[j], center[j])
				rho2 = a.add(rho2, a.mul(v, v))
			}
		}
	}

	if a.err != nil {
		return out, a.err
	}
	if rho2.Sign() <= 0 {
		return out, ErrDomain
	}
	// Authenticate the entire circle, using radial/axial orthogonality.
	for i := 0; i < 2; i++ {
		q := a.add(a.distance(center, s.center[i]), h2)
		if a.err != nil {
			return out, a.err
		}
		if q.Cmp(s.radius2[i]) != 0 {
			return out, ErrDomain
		}
	}
	out.MaterialNormalSigns = [2]int{1, 1}
	if s.op == SphereSetDifference {
		out.MaterialNormalSigns[1] = -1
	}
	for i := 0; i < 2; i++ {
		S := a.distance(p, s.center[i])
		R := a.sqrt(s.radius2[i])
		if a.err != nil {
			return out, a.err
		}
		if S.Sign() <= 0 {
			return out, ErrDomain
		}
		w := SphereOverlapAlgebraicCandidate{Radicand: S, Constant: a.add(S, s.radius2[i]), Coefficient: a.mul(big.NewRat(2, 1), R), Owners: 1 << uint(i)}
		dot := new(big.Rat)
		for j := 0; j < 3; j++ {
			w.Center[j] = new(big.Rat).Set(s.center[i][j])
			v := a.sub(p[j], s.center[i][j])
			w.Numerator[j] = a.mul(R, v)
			dot = a.add(dot, a.mul(a.sub(s.center[i][j], s.center[1-i][j]), v))
		}
		C := a.add(a.distance(s.center[i], s.center[1-i]), a.sub(s.radius2[i], s.radius2[1-i]))
		D := a.mul(w.Coefficient, dot)
		if a.err != nil {
			return out, a.err
		}
		// Other-sphere residual C+D/sqrt(S) has sign C*sqrt(S)+D.
		sign, e := sphereCavityRadicalSign(D, new(big.Rat).Neg(C), S, a.b)
		if e != nil {
			return out, e
		}
		w.Retained = sign >= 0
		if s.op == SphereSetIntersection || s.op == SphereSetDifference && i == 1 {
			w.Retained = sign <= 0
		}
		if sign == 0 {
			w.Owners = 3
		}
		out.Candidates[i] = w
	}

	var rim SphereOverlapAlgebraicCandidate
	if general {
		rim = SphereOverlapAlgebraicCandidate{Radicand: a.mul(h2, rho2), Constant: a.add(a.add(axial2, rho2), h2), Coefficient: big.NewRat(2, 1), Owners: 3, Retained: true}
		for j := 0; j < 3; j++ {
			rim.Center[j] = new(big.Rat).Set(center[j])
			rim.Numerator[j] = a.mul(h2, radial[j])
		}
	} else {
		axial := a.sub(p[axis], center[axis])
		rim = SphereOverlapAlgebraicCandidate{Radicand: rho2, Constant: a.add(a.add(a.mul(axial, axial), rho2), h2), Coefficient: a.mul(big.NewRat(2, 1), h), Owners: 3, Retained: true}
		for j := 0; j < 3; j++ {
			rim.Center[j] = new(big.Rat).Set(center[j])
			rim.Numerator[j] = new(big.Rat)
			if j != axis {
				rim.Numerator[j] = a.mul(h, a.sub(p[j], center[j]))
			}
		}
	}

	out.Candidates[2] = rim
	if a.err != nil {
		return out, a.err
	}
	for i := range out.Candidates {
		w := &out.Candidates[i]
		lo, hi, e := rationalSqrtBounds(w.Radicand, o.SqrtBits, a.b)
		if e != nil {
			return out, e
		}
		w.Root = [2]*big.Rat{lo, hi}
		w.Lower = a.sub(w.Constant, a.mul(w.Coefficient, hi))
		w.Upper = a.sub(w.Constant, a.mul(w.Coefficient, lo))
		if w.Lower.Sign() < 0 {
			w.Lower = new(big.Rat)
		}
	}
	if a.err != nil {
		return out, a.err
	}
	compare := func(i, j int) (int, error) {
		x, y := out.Candidates[i], out.Candidates[j]
		v := a.sub(x.Constant, y.Constant)
		if a.err != nil {
			return 0, a.err
		}
		return sphereCavityCompare(v, x.Coefficient, x.Radicand, y.Coefficient, y.Radicand, a.b)
	}
	best := 2
	for i := 0; i < 2; i++ {
		if !out.Candidates[i].Retained {
			continue
		}
		c, e := compare(i, best)
		if e != nil {
			return out, e
		}
		if c < 0 {
			best = i
		}
	}
	for i := range out.Candidates {
		if !out.Candidates[i].Retained {
			continue
		}
		c, e := compare(i, best)
		if e != nil {
			return out, e
		}
		if c == 0 {
			out.Candidates[i].Nearest = true
			out.Owners |= out.Candidates[i].Owners
		}
	}
	out.Lower = new(big.Rat).Set(out.Candidates[best].Lower)
	out.Upper = new(big.Rat).Set(out.Candidates[best].Upper)
	if err = ctx.Err(); err != nil {
		return
	}
	out.Operations = a.b.operations
	return out, nil
}
