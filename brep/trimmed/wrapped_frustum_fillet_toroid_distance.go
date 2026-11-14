package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumToroidDistanceOptions adds at most two stationary torus
// projections to the independently bounded support/rim query. This first
// subset requires exact rational query azimuth radius (including arbitrary
// rational unit azimuth, not just an axial meridian plane).
type FilletedFrustumToroidDistanceOptions struct {
	Rims            FilletedFrustumRimDistanceOptions
	MaxToroidChecks int
}

// FilletedFrustumToroidWitness is Center+Numerator/sqrt(Radicand). Root encloses
// the unique positive square root. Face is the original retained toroidal
// face. Acceptance proves strict interior of its actual meridian trim and
// full-circle azimuth; no rounded witness or nearest-edge substitution occurs.
type FilletedFrustumToroidWitness struct {
	Center, Numerator [3]*big.Rat
	Radicand          *big.Rat
	Root              [2]*big.Rat
	Face              int
}

// FilletedFrustumToroidDistance selects Toroid when its certified projection
// improves Baseline. NearestProven then requires every other original face
// lower bound to separate from this upper bound. Without that separation it
// remains a valid retained witness and conservative distance interval only.
// If Toroid is nil, witness and nearest semantics remain those of Baseline.
type FilletedFrustumToroidDistance struct {
	Baseline                 FilletedFrustumRimDistance
	Lower, Upper             *big.Rat
	Toroid                   *FilletedFrustumToroidWitness
	NearestProven, Converged bool
	ToroidChecks             int
}

// fdRadicalSign gives sign(a-b*sqrt(d)) for d>=0 without root approximation.
func fdRadicalSign(a, b, d *big.Rat) int {
	if b.Sign() == 0 || d.Sign() == 0 {
		return a.Sign()
	}
	if a.Sign() == 0 {
		return -b.Sign()
	}
	if a.Sign() != b.Sign() {
		return a.Sign()
	}
	sign := fdSub(fdSquare(a), fdMul(fdSquare(b), d)).Sign()
	if a.Sign() < 0 {
		return -sign
	}
	return sign
}

// BoundBoundaryToroidDistanceRational proves the full torus stationary point
// and then checks its original trim by exact radical signs. Positive query
// radius and positive torus radial coordinates minimize azimuthal distance
// at equal azimuth; the meridian circle then has the unique radial projection
// away from its center. Thus the retained candidate minimizes its full support.
// The strict outer
// branch and height guards reject inner torus sheets, seams, centerlines and
// trimmed-away projections. Unsupported irrational azimuths keep the safe
// baseline rather than making an approximate contact claim. Errors are atomic.
func (q FilletedFrustumMaterial) BoundBoundaryToroidDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumToroidDistanceOptions) (out FilletedFrustumToroidDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumToroidDistance{}
		}
	}()
	if o.MaxToroidChecks == 0 {
		o.MaxToroidChecks = 2
	}
	if o.MaxToroidChecks < 1 || o.MaxToroidChecks > 2 {
		return out, geom.ErrInvalid
	}
	base, e := q.BoundBoundaryRimDistanceRational(ctx, p, o.Rims)
	if e != nil {
		return out, e
	}
	out.Baseline = base
	out.Lower = new(big.Rat).Set(base.Lower)
	out.Upper = new(big.Rat).Set(base.Upper)
	out.NearestProven = base.NearestProven
	bits := o.Rims.Support.SqrtBits
	if bits == 0 {
		bits = 64
	}
	dx, dy := fdSub(p[0], q.center[0]), fdSub(p[1], q.center[1])
	rhoLo, rhoHi := fdSqrt(fdAdd(fdSquare(dx), fdSquare(dy)), bits)
	for end, c := range q.fillets.Circles {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if out.ToroidChecks >= o.MaxToroidChecks {
			return out, geom.ErrConvergence
		}
		out.ToroidChecks++
		if rhoLo.Sign() == 0 || rhoLo.Cmp(rhoHi) != 0 {
			continue
		}
		a, b := fdSub(rhoLo, c.Center[0]), fdSub(p[2], c.Center[1])
		if a.Sign() <= 0 {
			continue
		}
		radicand := fdAdd(fdSquare(a), fdSquare(b))
		if radicand.Sign() == 0 {
			continue
		}
		rb := fdMul(c.Radius, b)
		z0, z1 := q.z[0], c.Tangency[1]
		if end == 1 {
			z0, z1 = c.Tangency[1], q.z[1]
		}
		// z'=Cz+r*b/sqrt(D); multiplying each strict height comparison
		// by positive sqrt(D) reduces it to sign(r*b-(height-Cz)*sqrt(D)).
		if fdRadicalSign(rb, fdSub(z0, c.Center[1]), radicand) <= 0 || fdRadicalSign(rb, fdSub(z1, c.Center[1]), radicand) >= 0 {
			continue
		}
		lo, hi := fdSqrt(radicand, bits)
		_, upper := fdSquareRange(fdSub(lo, c.Radius), fdSub(hi, c.Radius))
		if upper.Cmp(out.Upper) >= 0 {
			continue
		}
		nx, ny := new(big.Rat).Quo(dx, rhoLo), new(big.Rat).Quo(dy, rhoLo)
		ra := fdMul(c.Radius, a)
		witness := &FilletedFrustumToroidWitness{Center: [3]*big.Rat{fdAdd(q.center[0], fdMul(c.Center[0], nx)), fdAdd(q.center[1], fdMul(c.Center[0], ny)), new(big.Rat).Set(c.Center[1])}, Numerator: [3]*big.Rat{fdMul(ra, nx), fdMul(ra, ny), rb}, Radicand: radicand, Root: [2]*big.Rat{lo, hi}, Face: q.fillets.Blends[end]}
		out.Toroid = witness
		out.Upper = upper
		out.NearestProven = true
		for _, face := range base.Baseline.PerFace {
			if face.Face != witness.Face && face.SquaredLower.Cmp(upper) < 0 {
				out.NearestProven = false
				break
			}
		}
	}
	if out.Lower.Cmp(out.Upper) > 0 {
		return out, geom.ErrConvergence
	}
	tolerance := new(big.Rat)
	if o.Rims.Support.SquaredTolerance != nil {
		tolerance.Set(o.Rims.Support.SquaredTolerance)
	}
	out.Converged = fdSub(out.Upper, out.Lower).Cmp(tolerance) <= 0
	return out, nil
}
