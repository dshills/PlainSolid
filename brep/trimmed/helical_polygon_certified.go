package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// CertifiedHelicalPolygonMesh has the same per-face image and display
// guarantees as CertifiedRevolvedPolygonMesh. The exact ideal source uses
// mathematical pi, positive radius and axial profile width strictly below one
// lead. Stored numerical cap discrepancies are included, not treated as exact
// sewing. SignedDisplayVolume is diagnostic rather than an error certificate.
type CertifiedHelicalPolygonMesh = CertifiedRevolvedPolygonMesh

// SweepHelicalPolygonCertified certifies a closed display for up to four turns
// of the constant-lead positive-radius meridian sweep. Exact turn reduction
// and an independently bounded Machin pi interval enclose angular samples.
// The axial lead is affine in turns and adds no Hessian interpolation error.
// Both handednesses and translated turn ranges are supported. All source
// embedding conditions are rechecked by SweepHelicalPolygon; every display
// still requires closed incidence, manifold links and a zero-contact proof.
func SweepHelicalPolygonCertified(ctx context.Context, frame geom.Frame, profile []geom.Vec2, lead, start, end float64, hand geom.HelixHandedness, o CertifiedRevolvePolygonOptions) (CertifiedHelicalPolygonMesh, error) {
	var empty CertifiedHelicalPolygonMesh
	if ctx == nil || !finite(start) || !finite(end) || end <= start || end-start > 4 {
		return empty, geom.ErrInvalid
	}
	p, _, e := parametric.PrepareRevolutionMeridian(frame, profile)
	if e != nil {
		return empty, e
	}
	source, e := SweepHelicalPolygon(ctx, frame, p, lead, start, end, hand)
	if e != nil {
		return empty, e
	}
	var piMid, piError *big.Rat
	phase := func(ctx context.Context, u *big.Rat, options CertifiedRevolvePolygonOptions, work *int) (*big.Rat, *big.Rat, *big.Rat, error) {
		if piMid == nil {
			var e error
			piMid, piError, e = certifiedSweepPi(ctx, options, work)
			if e != nil {
				return nil, nil, nil, e
			}
		}
		// Exact nearest-integer reduction bounds the argument by pi, without a
		// rounded remainder or a false represented-2*pi seam identity.
		integer := new(big.Int).Quo(u.Num(), u.Denom())
		fraction := new(big.Rat).Sub(u, new(big.Rat).SetInt(integer))
		if fraction.Cmp(big.NewRat(1, 2)) > 0 {
			fraction.Sub(fraction, big.NewRat(1, 1))
		}
		if fraction.Cmp(big.NewRat(-1, 2)) < 0 {
			fraction.Add(fraction, big.NewRat(1, 1))
		}
		factor := new(big.Rat).Mul(big.NewRat(2*int64(hand), 1), fraction)
		x := new(big.Rat).Mul(piMid, factor)
		sn, cs, remainder, e := revolutionTrigTaylor(ctx, x, options, work)
		if e != nil {
			return nil, nil, nil, e
		}
		remainder.Add(remainder, new(big.Rat).Mul(new(big.Rat).Abs(factor), piError)) // sine/cosine are 1-Lipschitz.
		return sn, cs, remainder, nil
	}
	recipe := certifiedAngularSweep{lo: ratInset(start), hi: ratInset(end), rate: big.NewRat(7, 1), lead: ratInset(lead), reversed: hand == geom.LeftHanded, phase: phase}
	// 2*pi<7 follows from the independently checked enclosure below. The loose
	// rational derivative constant avoids introducing square-root/pi arithmetic
	// into each cell bound. The first accepted grid always evaluates that proof.
	return certifiedPolygonAngularMesh(ctx, frame, p, source, recipe, o)
}

// Machin's identity pi=16*atan(1/5)-4*atan(1/239), with alternating-series
// remainder bounds, yields an exact rational interval. Outward dyadic rounding
// keeps subsequent Taylor coefficient denominators bounded. No libm result is
// used in the proof. The fixed enclosure has width at most a few 2^-128 units;
// tighter requested display errors conservatively remain unresolved.
func certifiedSweepPi(ctx context.Context, o CertifiedRevolvePolygonOptions, work *int) (*big.Rat, *big.Rat, error) {
	atan := func(q int64) (*big.Rat, *big.Rat, error) {
		sum := new(big.Rat)
		power := big.NewInt(q)
		square := big.NewInt(q * q)
		for k := 0; k < 64; k++ {
			if e := ctx.Err(); e != nil {
				return nil, nil, e
			}
			if *work >= o.MaxTrigTerms {
				return nil, nil, geom.ErrConvergence
			}
			*work++
			denom := new(big.Int).Mul(new(big.Int).Set(power), big.NewInt(int64(2*k+1)))
			term := new(big.Rat).SetFrac(big.NewInt(1), denom)
			if k%2 == 0 {
				sum.Add(sum, term)
			} else {
				sum.Sub(sum, term)
			}
			if sum.Num().BitLen() > o.MaxBits || sum.Denom().BitLen() > o.MaxBits {
				return nil, nil, geom.ErrConvergence
			}
			power.Mul(power, square)
		}
		remainder := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Mul(power, big.NewInt(129)))
		return sum, remainder, nil
	}
	a, ae, e := atan(5)
	if e != nil {
		return nil, nil, e
	}
	b, be, e := atan(239)
	if e != nil {
		return nil, nil, e
	}
	center := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(16, 1), a), new(big.Rat).Mul(big.NewRat(4, 1), b))
	radius := new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(16, 1), ae), new(big.Rat).Mul(big.NewRat(4, 1), be))
	scale := new(big.Int).Lsh(big.NewInt(1), 128)
	low := new(big.Rat).Mul(new(big.Rat).Sub(center, radius), new(big.Rat).SetInt(scale))
	high := new(big.Rat).Mul(new(big.Rat).Add(center, radius), new(big.Rat).SetInt(scale))
	lo := new(big.Int).Quo(low.Num(), low.Denom())
	hi, rem := new(big.Int), new(big.Int)
	hi.QuoRem(high.Num(), high.Denom(), rem)
	if rem.Sign() != 0 {
		hi.Add(hi, big.NewInt(1))
	}
	lower, upper := new(big.Rat).SetFrac(lo, new(big.Int).Set(scale)), new(big.Rat).SetFrac(hi, new(big.Int).Set(scale))
	if lower.Cmp(big.NewRat(3, 1)) <= 0 || upper.Cmp(big.NewRat(7, 2)) >= 0 {
		return nil, nil, geom.ErrConvergence
	}
	return new(big.Rat).Quo(new(big.Rat).Add(lower, upper), big.NewRat(2, 1)), new(big.Rat).Quo(new(big.Rat).Sub(upper, lower), big.NewRat(2, 1)), nil
}
