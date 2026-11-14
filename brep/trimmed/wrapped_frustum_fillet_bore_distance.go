package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumBoreDistanceOptions adds at most16 bore projections to one
// existing bounded query. Zero MaxBoreChecks selects16; exhaustion is atomic.
type FilletedFrustumBoreDistanceOptions struct {
	Original      FilletedFrustumNestedDistanceOptions
	MaxBoreChecks int
}

// FilletedFrustumBoreWitness is Center+Numerator/sqrt(Radicand), using the
// positive root enclosed in Root. Center.Z is the exact query height and
// Numerator.Z=0. Face is the original retained cylindrical bore wall.
type FilletedFrustumBoreWitness struct {
	Center, Numerator [3]*big.Rat
	Radicand          *big.Rat
	Root              [2]*big.Rat
	Face              int
}

// FilletedFrustumBoreDistance supplies a retained bore-interior witness only
// when it improves Baseline. NearestProven additionally separates all other
// original faces; Converged only compares the squared interval to tolerance.
type FilletedFrustumBoreDistance struct {
	Baseline                 FilletedFrustumConeDistance
	Lower, Upper             *big.Rat
	Bore                     *FilletedFrustumBoreWitness
	NearestProven, Converged bool
	BoreChecks               int
}

// BoundBoundaryBoreDistanceRational minimizes azimuth on a full cylinder,
// then requires query height strictly between the original bore ends. Native
// construction proves the entire circle retained and disjoint from other
// cuts, cone and blends. Cap/floor equality and outside-height projections
// never become bore-interior witnesses. Axis/rational azimuth keeps fallback.
func (q FilletedFrustumMaterial) BoundBoundaryBoreDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumBoreDistanceOptions) (out FilletedFrustumBoreDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumBoreDistance{}
		}
	}()
	if o.MaxBoreChecks == 0 {
		o.MaxBoreChecks = 16
	}
	if o.MaxBoreChecks < 1 || o.MaxBoreChecks > 16 {
		return out, geom.ErrInvalid
	}
	baseline, e := q.BoundBoundaryConeDistanceRational(ctx, p, o.Original)
	if e != nil {
		return out, e
	}
	out.Baseline = baseline
	out.Lower = new(big.Rat).Set(baseline.Lower)
	out.Upper = new(big.Rat).Set(baseline.Upper)
	out.NearestProven = baseline.NearestProven
	out.Converged = baseline.Converged
	bits := o.Original.Original.Rims.Support.SqrtBits
	if bits == 0 {
		bits = 64
	}
	for _, c := range q.cuts {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if out.BoreChecks >= o.MaxBoreChecks {
			return out, geom.ErrConvergence
		}
		out.BoreChecks++
		if p[2].Cmp(c.z[0]) <= 0 || p[2].Cmp(c.z[1]) >= 0 {
			continue
		}
		dx, dy := fdSub(p[0], c.center[0]), fdSub(p[1], c.center[1])
		S := fdAdd(fdSquare(dx), fdSquare(dy))
		lo, hi := fdSqrt(S, bits)
		if lo.Sign() <= 0 || lo.Cmp(hi) == 0 {
			continue
		}
		_, upper := fdSquareRange(fdSub(lo, c.radius), fdSub(hi, c.radius))
		if upper.Cmp(out.Upper) >= 0 {
			continue
		}
		out.Bore = &FilletedFrustumBoreWitness{Center: [3]*big.Rat{new(big.Rat).Set(c.center[0]), new(big.Rat).Set(c.center[1]), new(big.Rat).Set(p[2])}, Numerator: [3]*big.Rat{fdMul(c.radius, dx), fdMul(c.radius, dy), new(big.Rat)}, Radicand: S, Root: [2]*big.Rat{lo, hi}, Face: c.wall}
		out.Upper = upper
		out.NearestProven = true
		for _, f := range baseline.Baseline.Baseline.Baseline.Baseline.PerFace {
			if f.Face != c.wall && f.SquaredLower.Cmp(upper) < 0 {
				out.NearestProven = false
				break
			}
		}
	}
	if out.Lower.Cmp(out.Upper) > 0 {
		return out, geom.ErrConvergence
	}
	tolerance := new(big.Rat)
	if o.Original.Original.Rims.Support.SquaredTolerance != nil {
		tolerance.Set(o.Original.Original.Rims.Support.SquaredTolerance)
	}
	out.Converged = fdSub(out.Upper, out.Lower).Cmp(tolerance) <= 0
	return out, nil
}
