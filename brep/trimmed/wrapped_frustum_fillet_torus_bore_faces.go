//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// FilletedFrustumTorusBoreOptions bounds one baseline query and at most two
// original torus/bore scans. Zero MaxPairs selects32; range1..32.
type FilletedFrustumTorusBoreOptions struct {
	Support  FilletedFrustumDistanceOptions
	MaxPairs int
}

// FilletedFrustumTorusBoreFaces contains two attained retained candidates.
// Comparison is sign(D_torus-D_bore), meaningful only when Complete. Tied
// identifies the globally nearest original owners, not both candidates.
// All unrelated faces must be strictly excluded; unresolved returns no feet.
type FilletedFrustumTorusBoreFaces struct {
	Baseline                   FilletedFrustumDistance
	Toroid                     *FilletedFrustumToroidWitness
	Bore                       *FilletedFrustumBoreWitness
	Lower, Upper               *big.Rat
	Tied, Excluded, Unresolved []int
	Comparison, PairChecks     int
	Complete                   bool
}

// ClassifyTorusBoreFacesRational accepts exact rational stock azimuth radius,
// strict retained outer torus-circle foot and strict finite bore axial interior.
// The two squared minima are compared by guarded exact two-radical signs.
// Generic irrational stock azimuth, endpoints and insufficient separation stay
// unresolved. One baseline query; aggregate pair limit; every error is atomic.
func (q FilletedFrustumMaterial) ClassifyTorusBoreFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumTorusBoreOptions) (out FilletedFrustumTorusBoreFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumTorusBoreFaces{}
		}
	}()
	if o.MaxPairs == 0 {
		o.MaxPairs = 32
	}
	if o.MaxPairs < 1 || o.MaxPairs > 32 {
		return out, geom.ErrInvalid
	}
	base, e := q.BoundBoundaryDistanceRational(ctx, p, o.Support)
	if e != nil {
		return out, e
	}
	out.Baseline = base
	out.Lower = new(big.Rat).Set(base.Lower)
	out.Upper = new(big.Rat).Set(base.Upper)
	for _, f := range base.PerFace {
		out.Unresolved = append(out.Unresolved, f.Face)
	}
	sort.Ints(out.Unresolved)
	bits := o.Support.SqrtBits
	if bits == 0 {
		bits = 64
	}
	dx, dy := fdSub(p[0], q.center[0]), fdSub(p[1], q.center[1])
	rho, other := fdSqrt(fdAdd(fdSquare(dx), fdSquare(dy)), bits)
	if rho.Sign() <= 0 || rho.Cmp(other) != 0 {
		return out, nil
	}
	ux, uy := new(big.Rat).Quo(dx, rho), new(big.Rat).Quo(dy, rho)
	for end, circle := range q.fillets.Circles {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		a, b := fdSub(rho, circle.Center[0]), fdSub(p[2], circle.Center[1])
		if a.Sign() <= 0 {
			continue
		}
		D := fdAdd(fdSquare(a), fdSquare(b))
		if D.Sign() <= 0 {
			continue
		}
		z0, z1 := q.z[0], circle.Tangency[1]
		if end == 1 {
			z0, z1 = circle.Tangency[1], q.z[1]
		}
		rb := fdMul(circle.Radius, b)
		sign := func(z *big.Rat) int { return fdRadicalSign(rb, new(big.Rat).Neg(fdSub(circle.Center[1], z)), D) }
		if sign(z0) <= 0 || sign(z1) >= 0 {
			continue
		}
		dl, dh := fdSqrt(D, bits)
		torusLo, torusHi := fdSquareRange(fdSub(dl, circle.Radius), fdSub(dh, circle.Radius))
		for _, cut := range q.cuts {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if out.PairChecks >= o.MaxPairs {
				return out, geom.ErrConvergence
			}
			out.PairChecks++
			if p[2].Cmp(cut.z[0]) <= 0 || p[2].Cmp(cut.z[1]) >= 0 {
				continue
			}
			bx, by := fdSub(p[0], cut.center[0]), fdSub(p[1], cut.center[1])
			T := fdAdd(fdSquare(bx), fdSquare(by))
			if T.Sign() <= 0 {
				continue
			}
			tl, th := fdSqrt(T, bits)
			boreLo, boreHi := fdSquareRange(fdSub(tl, cut.radius), fdSub(th, cut.radius))
			A := fdSub(fdAdd(D, fdSquare(circle.Radius)), fdAdd(T, fdSquare(cut.radius)))
			comparison := fdTwoPositiveRadicalsSign(A, fdMul(big.NewRat(2, 1), circle.Radius), D, fdMul(big.NewRat(2, 1), cut.radius), T)
			lo, hi := torusLo, torusHi
			if comparison > 0 {
				lo, hi = boreLo, boreHi
			}
			face := q.fillets.Blends[end]
			excluded := []int{}
			separated := true
			for _, f := range base.PerFace {
				if f.Face == face || f.Face == cut.wall {
					continue
				}
				if f.SquaredLower.Cmp(hi) <= 0 {
					separated = false
					break
				}
				excluded = append(excluded, f.Face)
			}
			if !separated {
				continue
			}
			if base.Upper.Cmp(lo) < 0 || face == cut.wall {
				return out, geom.ErrConvergence
			}
			ra := fdMul(circle.Radius, a)
			out.Toroid = &FilletedFrustumToroidWitness{Center: [3]*big.Rat{fdAdd(q.center[0], fdMul(circle.Center[0], ux)), fdAdd(q.center[1], fdMul(circle.Center[0], uy)), new(big.Rat).Set(circle.Center[1])}, Numerator: [3]*big.Rat{fdMul(ra, ux), fdMul(ra, uy), new(big.Rat).Set(rb)}, Radicand: new(big.Rat).Set(D), Root: [2]*big.Rat{new(big.Rat).Set(dl), new(big.Rat).Set(dh)}, Face: face}
			out.Bore = &FilletedFrustumBoreWitness{Center: [3]*big.Rat{new(big.Rat).Set(cut.center[0]), new(big.Rat).Set(cut.center[1]), new(big.Rat).Set(p[2])}, Numerator: [3]*big.Rat{fdMul(cut.radius, bx), fdMul(cut.radius, by), new(big.Rat)}, Radicand: new(big.Rat).Set(T), Root: [2]*big.Rat{new(big.Rat).Set(tl), new(big.Rat).Set(th)}, Face: cut.wall}
			switch {
			case comparison < 0:
				out.Tied = []int{face}
				excluded = append(excluded, cut.wall)
			case comparison > 0:
				out.Tied = []int{cut.wall}
				excluded = append(excluded, face)
			default:
				out.Tied = []int{face, cut.wall}
			}
			sort.Ints(out.Tied)
			sort.Ints(excluded)
			out.Excluded = excluded
			out.Unresolved = nil
			out.Lower = new(big.Rat).Set(lo)
			out.Upper = new(big.Rat).Set(hi)
			out.Comparison = comparison
			out.Complete = true
			return out, nil
		}
	}
	return out, nil
}
