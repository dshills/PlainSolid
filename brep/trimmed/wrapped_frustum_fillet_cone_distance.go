//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumConeWitness is Base+RadicalVector*sqrt(Radicand), using the
// unique positive root enclosed in Root. Face is the original conical wall.
// Acceptance proves positive cone radius and strict retained axial interior;
// the unbounded meridian-line projection alone is never treated as retained.
type FilletedFrustumConeWitness struct {
	Base, RadicalVector [3]*big.Rat
	Radicand            *big.Rat
	Root                [2]*big.Rat
	Face                int
}

// FilletedFrustumConeDistance adds an irrational-azimuth conical-wall witness
// to the complete earlier support/rim/torus result. A nonnil Cone is retained;
// NearestProven additionally separates every other native face. A nil Cone
// retains the witness and nearest status of Baseline, without promoting it.
type FilletedFrustumConeDistance struct {
	Baseline                 FilletedFrustumNestedDistance
	Lower, Upper             *big.Rat
	Cone                     *FilletedFrustumConeWitness
	NearestProven, Converged bool
}

// BoundBoundaryConeDistanceRational adds one fixed-degree exact projection
// after the bounded nested query; source input limits and root precision are
// inherited unchanged. For rho=sqrt(x²+y²), radius R at query height and
// slope s, the meridian foot has radial coordinate (s²*rho+R)/(1+s²)
// and height z+s*(rho-R)/(1+s²). Exact radical signs prove that foot is in
// the actual original cone trim before it can supply an upper witness.
// Rotational symmetry minimizes azimuth at the query azimuth; the positive
// meridian foot is the global full-cone minimum. Native construction already
// proves every bore disjoint from that retained wall. Other original supports
// must still separate before NearestProven is set for this candidate.
func (q FilletedFrustumMaterial) BoundBoundaryConeDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedDistanceOptions) (out FilletedFrustumConeDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumConeDistance{}
		}
	}()
	baseline, e := q.BoundBoundaryNestedDistanceRational(ctx, p, o)
	if e != nil {
		return out, e
	}
	out.Baseline = baseline
	out.Lower = new(big.Rat).Set(baseline.Lower)
	out.Upper = new(big.Rat).Set(baseline.Upper)
	out.NearestProven = baseline.NearestProven
	out.Converged = baseline.Converged
	if e := ctx.Err(); e != nil {
		return out, e
	}
	bits := o.Original.Rims.Support.SqrtBits
	if bits == 0 {
		bits = 64
	}
	dx, dy := fdSub(p[0], q.center[0]), fdSub(p[1], q.center[1])
	S := fdAdd(fdSquare(dx), fdSquare(dy))
	rhoLo, rhoHi := fdSqrt(S, bits)
	if rhoLo.Sign() <= 0 || rhoLo.Cmp(rhoHi) == 0 {
		return out, nil
	} // existing rational/axis witnesses stay unchanged
	denominator := fdAdd(big.NewRat(1, 1), fdSquare(q.slope))
	R := fdAdd(q.r[0], fdMul(q.slope, fdSub(p[2], q.z[0])))
	radialConstant := new(big.Rat).Quo(R, denominator)
	radialCoefficient := new(big.Rat).Quo(fdSquare(q.slope), denominator)
	heightConstant := fdSub(p[2], fdMul(q.slope, radialConstant))
	heightCoefficient := new(big.Rat).Quo(q.slope, denominator)
	sign := func(constant, coefficient *big.Rat) int {
		return fdRadicalSign(constant, new(big.Rat).Neg(coefficient), S)
	}
	if sign(radialConstant, radialCoefficient) <= 0 || sign(fdSub(heightConstant, q.fillets.Circles[0].Tangency[1]), heightCoefficient) <= 0 || sign(fdSub(heightConstant, q.fillets.Circles[1].Tangency[1]), heightCoefficient) >= 0 {
		return out, nil
	}
	_, upper := fdSquareRange(fdSub(rhoLo, R), fdSub(rhoHi, R))
	upper.Quo(upper, denominator)
	if upper.Cmp(out.Upper) >= 0 {
		return out, nil
	}
	// Since rho>0,1/rho=rho/S. This expresses all three coordinates in
	// Q(sqrt(S)) without introducing another radical or any rounded point.
	vectorScale := new(big.Rat).Quo(radialConstant, S)
	witness := &FilletedFrustumConeWitness{Base: [3]*big.Rat{fdAdd(q.center[0], fdMul(dx, radialCoefficient)), fdAdd(q.center[1], fdMul(dy, radialCoefficient)), heightConstant}, RadicalVector: [3]*big.Rat{fdMul(dx, vectorScale), fdMul(dy, vectorScale), heightCoefficient}, Radicand: S, Root: [2]*big.Rat{rhoLo, rhoHi}, Face: q.fillets.Lateral}
	out.Cone = witness
	out.Upper = upper
	out.NearestProven = true
	for _, f := range baseline.Baseline.Baseline.Baseline.PerFace {
		if f.Face != witness.Face && f.SquaredLower.Cmp(upper) < 0 {
			out.NearestProven = false
			break
		}
	}
	if out.Lower.Cmp(out.Upper) > 0 {
		return out, geom.ErrConvergence
	}
	tolerance := new(big.Rat)
	if o.Original.Rims.Support.SquaredTolerance != nil {
		tolerance.Set(o.Original.Rims.Support.SquaredTolerance)
	}
	out.Converged = fdSub(out.Upper, out.Lower).Cmp(tolerance) <= 0
	if e := ctx.Err(); e != nil {
		return out, e
	}
	return out, nil
}
