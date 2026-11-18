//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumDistanceOptions bounds this noniterative all-support query.
// SqrtBits defaults to64 (8..256 allowed). Each square-root enclosure uses one
// bounded integer square root; at most1+2*2+len(cuts) are needed. The checked
// input and immutable source have4096-bit rational coefficients. This is not
// a subdivision solver: unresolved trim exclusions can leave a wide interval.
// The16-cut preparation limit also bounds exact retained-witness checks to75.
type FilletedFrustumDistanceOptions struct {
	SqrtBits         int
	MaxSupportChecks int      // zero:37, range1..37; one per original native face
	SquaredTolerance *big.Rat // zero/nil means exact convergence only
}

// FilletedFrustumFaceDistanceLower bounds distance to one retained native face
// by distance to a larger analytic support, with axial slab and original
// circular cap/floor domain restrictions. Cap exclusions apply only to holes
// actually entering that cap; independent constraint distances use a maximum.
type FilletedFrustumFaceDistanceLower struct {
	Face         int
	SquaredLower *big.Rat
}

// FilletedFrustumDistance bounds minimum squared distance to the entire native
// retained boundary. Witness is an exact rational retained boundary point;
// WitnessFaces are its native owners, not a claim of globally unique nearest
// ownership. Only Exact proves that witness nearest. PerFace lower bounds may
// refer to closest points on trimmed-away supports. Converged compares the
// interval width with the requested tolerance; it does not imply uniqueness.
type FilletedFrustumDistance struct {
	Lower, Upper     *big.Rat
	Witness          [3]*big.Rat
	WitnessFaces     []int
	PerFace          []FilletedFrustumFaceDistanceLower
	Exact, Converged bool
	SupportChecks    int
}

func fdSquare(a *big.Rat) *big.Rat { return new(big.Rat).Mul(a, a) }
func fdSub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func fdAdd(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
func fdMul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
func fdSquareRange(lo, hi *big.Rat) (*big.Rat, *big.Rat) {
	a, b := fdSquare(lo), fdSquare(hi)
	if a.Cmp(b) > 0 {
		a, b = b, a
	}
	if lo.Sign() <= 0 && hi.Sign() >= 0 {
		a = new(big.Rat)
	}
	return a, b
}
func fdSqrt(x *big.Rat, bits int) (*big.Rat, *big.Rat) {
	n, d := new(big.Int).Sqrt(x.Num()), new(big.Int).Sqrt(x.Denom())
	if new(big.Int).Mul(n, n).Cmp(x.Num()) == 0 && new(big.Int).Mul(d, d).Cmp(x.Denom()) == 0 {
		v := new(big.Rat).SetFrac(n, d)
		return v, new(big.Rat).Set(v)
	}
	// floor(sqrt(num*den)*2^bits)/(den*2^bits) encloses sqrt(x).
	a := new(big.Int).Mul(x.Num(), x.Denom())
	a.Lsh(a, uint(2*bits))
	a.Sqrt(a)
	den := new(big.Int).Lsh(new(big.Int).Set(x.Denom()), uint(bits))
	return new(big.Rat).SetFrac(new(big.Int).Set(a), new(big.Int).Set(den)), new(big.Rat).SetFrac(a.Add(a, big.NewInt(1)), den)
}

// BoundBoundaryDistanceRational considers every original cap, cone, torus,
// bore wall and blind floor. Upper candidates are accepted only through the
// exact retained-material classifier, never from an untrimmed projection.
// Rational cap/bore and cone/fillet rim projections retain all exact native
// owners. An exact cap/blend joint provides a fallback witness. Errors are atomic.
func (q FilletedFrustumMaterial) BoundBoundaryDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (FilletedFrustumDistance, error) {
	return q.boundBoundaryDistanceRational(ctx, p, o, nil)
}

func (q FilletedFrustumMaterial) boundBoundaryDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions, observe func([3]*big.Rat, []int, *big.Rat) error) (out FilletedFrustumDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumDistance{}
		}
	}()
	if ctx == nil || q.recipe == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.SqrtBits == 0 {
		o.SqrtBits = 64
	}
	if o.MaxSupportChecks == 0 {
		o.MaxSupportChecks = 37
	}
	if o.SqrtBits < 8 || o.SqrtBits > 256 || o.MaxSupportChecks < 1 || o.MaxSupportChecks > 37 {
		return out, geom.ErrInvalid
	}
	for _, v := range p {
		if v == nil {
			return out, geom.ErrInvalid
		}
		if v.Num().BitLen() > 4096 || v.Denom().BitLen() > 4096 {
			return out, geom.ErrConvergence
		}
	}
	tolerance := new(big.Rat)
	if o.SquaredTolerance != nil {
		if o.SquaredTolerance.Sign() < 0 {
			return out, geom.ErrInvalid
		}
		if o.SquaredTolerance.Num().BitLen() > 4096 || o.SquaredTolerance.Denom().BitLen() > 4096 {
			return out, geom.ErrConvergence
		}
		tolerance.Set(o.SquaredTolerance)
	}
	add := func(face int, lower, z0, z1 *big.Rat) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		if out.SupportChecks >= o.MaxSupportChecks {
			return geom.ErrConvergence
		}
		out.SupportChecks++
		slab := new(big.Rat)
		if p[2].Cmp(z0) < 0 {
			slab = fdSquare(fdSub(p[2], z0))
		} else if p[2].Cmp(z1) > 0 {
			slab = fdSquare(fdSub(p[2], z1))
		}
		if slab.Cmp(lower) > 0 {
			lower = slab
		}
		out.PerFace = append(out.PerFace, FilletedFrustumFaceDistanceLower{face, new(big.Rat).Set(lower)})
		if out.Lower == nil || lower.Cmp(out.Lower) < 0 {
			out.Lower = new(big.Rat).Set(lower)
		}
		return nil
	}
	witness := func(v [3]*big.Rat) error {
		check, e := q.ClassifyRational(ctx, v)
		if e != nil {
			return e
		}
		if check.Location != geom.BoreBoundary {
			return nil
		}
		dist := new(big.Rat)
		for i := range p {
			dist.Add(dist, fdSquare(fdSub(p[i], v[i])))
		}
		if observe != nil {
			if e := observe(v, check.Faces, dist); e != nil {
				return e
			}
		}
		if out.Upper == nil || dist.Cmp(out.Upper) < 0 {
			out.Upper = dist
			out.WitnessFaces = append([]int(nil), check.Faces...)
			for i := range v {
				out.Witness[i] = new(big.Rat).Set(v[i])
			}
		}
		return nil
	}
	radialPoint := func(cx, cy, rho, radius, z *big.Rat) [3]*big.Rat {
		if rho.Sign() == 0 {
			return [3]*big.Rat{fdAdd(cx, radius), new(big.Rat).Set(cy), z}
		}
		scale := new(big.Rat).Quo(radius, rho)
		return [3]*big.Rat{fdAdd(cx, fdMul(fdSub(p[0], cx), scale)), fdAdd(cy, fdMul(fdSub(p[1], cy), scale)), z}
	}
	rhoLo, rhoHi := fdSqrt(fdAdd(fdSquare(fdSub(p[0], q.center[0])), fdSquare(fdSub(p[1], q.center[1]))), o.SqrtBits)
	type radialBracket struct{ lo, hi *big.Rat }
	cutRadial := make([]radialBracket, len(q.cuts))
	for i, c := range q.cuts {
		lo, hi := fdSqrt(fdAdd(fdSquare(fdSub(p[0], c.center[0])), fdSquare(fdSub(p[1], c.center[1]))), o.SqrtBits)
		cutRadial[i] = radialBracket{lo, hi}
	}
	positiveSquare := func(v *big.Rat) *big.Rat {
		if v.Sign() <= 0 {
			return new(big.Rat)
		}
		return fdSquare(v)
	}
	// A retained cap is inside its original flat disk and outside precisely
	// the cutter disks entering that end. Each exclusion independently bounds
	// planar travel; take their maximum, then add orthogonal plane separation.
	plane := func(face int, z, radialLower *big.Rat) error {
		if e := add(face, fdAdd(fdSquare(fdSub(p[2], z)), radialLower), z, z); e != nil {
			return e
		}
		return witness([3]*big.Rat{p[0], p[1], z})
	}
	for end := 0; end < 2; end++ {
		lower := positiveSquare(fdSub(rhoLo, q.fillets.Circles[end].Center[0]))
		for i, c := range q.cuts {
			if c.open[end] {
				v := positiveSquare(fdSub(c.radius, cutRadial[i].hi))
				if v.Cmp(lower) > 0 {
					lower = v
				}
			}
		}
		if e := plane(q.fillets.Caps[end], q.z[end], lower); e != nil {
			return out, e
		}
	}
	radius := fdAdd(q.r[0], fdMul(q.slope, fdSub(p[2], q.z[0])))
	resLo, resHi := fdSub(rhoLo, radius), fdSub(rhoHi, radius)
	denom := fdAdd(big.NewRat(1, 1), fdSquare(q.slope))
	lower, _ := fdSquareRange(resLo, resHi)
	lower.Quo(lower, denom)
	if e := add(q.fillets.Lateral, lower, q.fillets.Circles[0].Tangency[1], q.fillets.Circles[1].Tangency[1]); e != nil {
		return out, e
	}
	if rhoLo.Cmp(rhoHi) == 0 {
		delta := new(big.Rat).Quo(resLo, denom)
		rp := fdSub(rhoLo, delta)
		if rp.Sign() >= 0 {
			if e := witness(radialPoint(q.center[0], q.center[1], rhoLo, rp, fdAdd(p[2], fdMul(q.slope, delta)))); e != nil {
				return out, e
			}
		}
	}
	for end, c := range q.fillets.Circles {
		a, b := fdSub(rhoLo, c.Center[0]), fdSub(rhoHi, c.Center[0])
		lo, hi := fdSquareRange(a, b)
		dz := fdSub(p[2], c.Center[1])
		lo.Add(lo, fdSquare(dz))
		hi.Add(hi, fdSquare(dz))
		dl, _ := fdSqrt(lo, o.SqrtBits)
		_, dh := fdSqrt(hi, o.SqrtBits)
		lower, _ = fdSquareRange(fdSub(dl, c.Radius), fdSub(dh, c.Radius))
		z0, z1 := q.z[0], c.Tangency[1]
		if end == 1 {
			z0, z1 = c.Tangency[1], q.z[1]
		}
		if e := add(q.fillets.Blends[end], lower, z0, z1); e != nil {
			return out, e
		}
		if rhoLo.Cmp(rhoHi) == 0 && dl.Cmp(dh) == 0 && dl.Sign() != 0 {
			scale := new(big.Rat).Quo(c.Radius, dl)
			rp := fdAdd(c.Center[0], fdMul(scale, a))
			zp := fdAdd(c.Center[1], fdMul(scale, dz))
			if rp.Sign() >= 0 {
				if e := witness(radialPoint(q.center[0], q.center[1], rhoLo, rp, zp)); e != nil {
					return out, e
				}
			}
		}
		// Original full-circle cap/blend and cone/blend junctions. A rational
		// azimuth produces an exact candidate and classification proves all owners.
		if rhoLo.Cmp(rhoHi) == 0 {
			for _, rz := range [][2]*big.Rat{{c.Center[0], q.z[end]}, c.Tangency} {
				if e := witness(radialPoint(q.center[0], q.center[1], rhoLo, rz[0], rz[1])); e != nil {
					return out, e
				}
			}
		}
		// This exact cap/blend joint is clear of every cutter by preparation proof.
		if e := witness([3]*big.Rat{fdAdd(q.center[0], c.Center[0]), q.center[1], q.z[end]}); e != nil {
			return out, e
		}
	}
	for i, c := range q.cuts {
		lo, hi := cutRadial[i].lo, cutRadial[i].hi
		lower, _ := fdSquareRange(fdSub(lo, c.radius), fdSub(hi, c.radius))
		// Product cylinder support permits summing radial and axial distances.
		if p[2].Cmp(c.z[0]) < 0 {
			lower.Add(lower, fdSquare(fdSub(p[2], c.z[0])))
		} else if p[2].Cmp(c.z[1]) > 0 {
			lower.Add(lower, fdSquare(fdSub(p[2], c.z[1])))
		}
		if e := add(c.wall, lower, c.z[0], c.z[1]); e != nil {
			return out, e
		}
		if lo.Cmp(hi) == 0 {
			if e := witness(radialPoint(c.center[0], c.center[1], lo, c.radius, p[2])); e != nil {
				return out, e
			}
		}
		// Both exact end circles are retained cap or blind-floor rim contacts.
		if lo.Cmp(hi) == 0 {
			for _, z := range c.z {
				if e := witness(radialPoint(c.center[0], c.center[1], lo, c.radius, z)); e != nil {
					return out, e
				}
			}
		}
		if c.floor >= 0 {
			z := c.z[1]
			if c.open[1] {
				z = c.z[0]
			}
			if e := plane(c.floor, z, positiveSquare(fdSub(lo, c.radius))); e != nil {
				return out, e
			}
		}
	}
	if out.Upper == nil || out.Lower == nil || out.Lower.Cmp(out.Upper) > 0 {
		return out, geom.ErrConvergence
	}
	out.Exact = out.Lower.Cmp(out.Upper) == 0
	out.Converged = fdSub(out.Upper, out.Lower).Cmp(tolerance) <= 0
	return out, nil
}
