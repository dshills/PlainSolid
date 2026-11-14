package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumNestedDistanceOptions adds two bounded nested-radical
// projections after the existing exact-rational-azimuth query. Root precision
// is inherited from Original.Rims.Support.SqrtBits; uncertain strict trim
// inequalities keep the previous conservative result rather than rounding.
type FilletedFrustumNestedDistanceOptions struct {
	Original        FilletedFrustumToroidDistanceOptions
	MaxNestedChecks int
}

// FilletedFrustumNestedToroidWitness is the exact original point defined by
// rho=sqrt(AzimuthSquared)>0, d=sqrt((rho-MajorRadius)^2+HeightOffset^2)>0,
// R=MajorRadius+MinorRadius*(rho-MajorRadius)/d, and
// (CenterX+R*AzimuthX/rho, CenterY+R*AzimuthY/rho,
// CenterZ+MinorRadius*HeightOffset/d). Both square roots are their positive
// roots; the intervals enclose them but do not replace the exact definition.
// Face owns the strictly interior retained torus point, not a sampled point.
type FilletedFrustumNestedToroidWitness struct {
	Center                                 [3]*big.Rat
	MajorRadius, MinorRadius, HeightOffset *big.Rat
	Azimuth                                [2]*big.Rat
	AzimuthSquared                         *big.Rat
	AzimuthRoot, MeridianRoot              [2]*big.Rat
	Face                                   int
}

// FilletedFrustumNestedDistance chooses Nested if its exact stationary point
// tightens Baseline. NearestProven additionally separates every other original
// face using independent lower bounds. Otherwise only the interval and retained
// witness are certified. A nil Nested retains the original selected witness.
type FilletedFrustumNestedDistance struct {
	Baseline                 FilletedFrustumToroidDistance
	Lower, Upper             *big.Rat
	Nested                   *FilletedFrustumNestedToroidWitness
	NearestProven, Converged bool
	NestedChecks             int
}

// BoundBoundaryNestedDistanceRational supports genuinely irrational local
// azimuth with nested exact positive radicals. Rational interval arithmetic
// proves the entire enclosure stays on the outer torus branch and strictly
// between the original trim heights. This finite precision check is sufficient,
// not necessary: unseparated trim signs preserve the previous safe baseline.
func (q FilletedFrustumMaterial) BoundBoundaryNestedDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumNestedDistanceOptions) (out FilletedFrustumNestedDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumNestedDistance{}
		}
	}()
	if o.MaxNestedChecks == 0 {
		o.MaxNestedChecks = 2
	}
	if o.MaxNestedChecks < 1 || o.MaxNestedChecks > 2 {
		return out, geom.ErrInvalid
	}
	baseline, e := q.BoundBoundaryToroidDistanceRational(ctx, p, o.Original)
	if e != nil {
		return out, e
	}
	out.Baseline = baseline
	out.Lower = new(big.Rat).Set(baseline.Lower)
	out.Upper = new(big.Rat).Set(baseline.Upper)
	out.NearestProven = baseline.NearestProven
	bits := o.Original.Rims.Support.SqrtBits
	if bits == 0 {
		bits = 64
	}
	dx, dy := fdSub(p[0], q.center[0]), fdSub(p[1], q.center[1])
	rho2 := fdAdd(fdSquare(dx), fdSquare(dy))
	rhoLo, rhoHi := fdSqrt(rho2, bits)
	for end, c := range q.fillets.Circles {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if out.NestedChecks >= o.MaxNestedChecks {
			return out, geom.ErrConvergence
		}
		out.NestedChecks++
		// Rational azimuth already has the stronger exact radical-sign path.
		if rhoLo.Sign() <= 0 || rhoLo.Cmp(rhoHi) == 0 {
			continue
		}
		aLo, aHi := fdSub(rhoLo, c.Center[0]), fdSub(rhoHi, c.Center[0])
		if aLo.Sign() <= 0 {
			continue
		}
		b := fdSub(p[2], c.Center[1])
		dLo, _ := fdSqrt(fdAdd(fdSquare(aLo), fdSquare(b)), bits)
		_, dHi := fdSqrt(fdAdd(fdSquare(aHi), fdSquare(b)), bits)
		if dLo.Sign() <= 0 {
			continue
		}
		rb := fdMul(c.Radius, b)
		offsetLo, offsetHi := new(big.Rat).Quo(rb, dHi), new(big.Rat).Quo(rb, dLo)
		if rb.Sign() < 0 {
			offsetLo, offsetHi = offsetHi, offsetLo
		}
		zLo, zHi := fdAdd(c.Center[1], offsetLo), fdAdd(c.Center[1], offsetHi)
		z0, z1 := q.z[0], c.Tangency[1]
		if end == 1 {
			z0, z1 = c.Tangency[1], q.z[1]
		}
		if zLo.Cmp(z0) <= 0 || zHi.Cmp(z1) >= 0 {
			continue
		}
		// Equal-azimuth minimization followed by radial meridian-circle projection
		// gives the closest full-torus point. The strict interval guards above
		// prove that exact point is also retained by this original native face.
		_, upper := fdSquareRange(fdSub(dLo, c.Radius), fdSub(dHi, c.Radius))
		if upper.Cmp(out.Upper) >= 0 {
			continue
		}
		witness := &FilletedFrustumNestedToroidWitness{Center: [3]*big.Rat{new(big.Rat).Set(q.center[0]), new(big.Rat).Set(q.center[1]), new(big.Rat).Set(c.Center[1])}, MajorRadius: new(big.Rat).Set(c.Center[0]), MinorRadius: new(big.Rat).Set(c.Radius), HeightOffset: b, Azimuth: [2]*big.Rat{new(big.Rat).Set(dx), new(big.Rat).Set(dy)}, AzimuthSquared: new(big.Rat).Set(rho2), AzimuthRoot: [2]*big.Rat{new(big.Rat).Set(rhoLo), new(big.Rat).Set(rhoHi)}, MeridianRoot: [2]*big.Rat{dLo, dHi}, Face: q.fillets.Blends[end]}
		out.Nested = witness
		out.Upper = upper
		out.NearestProven = true
		for _, f := range baseline.Baseline.Baseline.PerFace {
			if f.Face != witness.Face && f.SquaredLower.Cmp(upper) < 0 {
				out.NearestProven = false
				break
			}
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
	return out, nil
}
