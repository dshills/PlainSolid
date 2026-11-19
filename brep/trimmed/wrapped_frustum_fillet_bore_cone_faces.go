//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// FilletedFrustumBoreConeOptions bounds a single support query and the scan of
// original bore walls. Zero MaxBores selects16; valid range1..16.
type FilletedFrustumBoreConeOptions struct {
	Support  FilletedFrustumDistanceOptions
	MaxBores int
}

// FilletedFrustumBoreConeFaces retains independently attained interior cone
// and finite bore-wall candidates. Comparison is sign(D_cone-D_bore), proved
// symbolically, not by interval disjointness; it is meaningful only when
// Complete. Only Tied attains the global
// minimum. Complete requires all other original faces strictly separated.
// Unresolved output contains no candidates. This is an owner classification,
// not a claim that both candidate points are nearest or an exhaustive locus.
type FilletedFrustumBoreConeFaces struct {
	Baseline                   FilletedFrustumDistance
	Cone                       *FilletedFrustumConeWitness
	Bore                       *FilletedFrustumBoreWitness
	Lower, Upper               *big.Rat
	Tied, Excluded, Unresolved []int
	Comparison, BoreChecks     int
	Complete                   bool
}

// ClassifyBoreConeFacesRational compares original interior cone and bore
// minima in at most two positive radicals. The exact cone foot must have
// positive radius and strict retained cone height; the query must be strictly
// inside the original bore axial interval and off both symmetry axes.
// One bounded baseline query is reused. Failure is atomic; unsupported or
// insufficiently separated cases preserve its conservative bounds and owners.
func (q FilletedFrustumMaterial) ClassifyBoreConeFacesRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumBoreConeOptions) (out FilletedFrustumBoreConeFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumBoreConeFaces{}
		}
	}()
	if o.MaxBores == 0 {
		o.MaxBores = 16
	}
	if o.MaxBores < 1 || o.MaxBores > 16 {
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
	S := fdAdd(fdSquare(dx), fdSquare(dy))
	R := fdAdd(q.r[0], fdMul(q.slope, fdSub(p[2], q.z[0])))
	if S.Sign() <= 0 || R.Sign() <= 0 {
		return out, nil
	}
	den := fdAdd(big.NewRat(1, 1), fdSquare(q.slope))
	rc := new(big.Rat).Quo(R, den)
	rk := new(big.Rat).Quo(fdSquare(q.slope), den)
	hc := fdSub(p[2], fdMul(q.slope, rc))
	hk := new(big.Rat).Quo(q.slope, den)
	sign := func(c, k *big.Rat) int { return fdRadicalSign(c, new(big.Rat).Neg(k), S) }
	if sign(rc, rk) <= 0 || sign(fdSub(hc, q.fillets.Circles[0].Tangency[1]), hk) <= 0 || sign(fdSub(hc, q.fillets.Circles[1].Tangency[1]), hk) >= 0 {
		return out, nil
	}
	sl, sh := fdSqrt(S, bits)
	coneLo, coneHi := fdSquareRange(fdSub(sl, R), fdSub(sh, R))
	coneLo.Quo(coneLo, den)
	coneHi.Quo(coneHi, den)
	for _, c := range q.cuts {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if out.BoreChecks >= o.MaxBores {
			return out, geom.ErrConvergence
		}
		out.BoreChecks++
		if p[2].Cmp(c.z[0]) <= 0 || p[2].Cmp(c.z[1]) >= 0 {
			continue
		}
		bx, by := fdSub(p[0], c.center[0]), fdSub(p[1], c.center[1])
		T := fdAdd(fdSquare(bx), fdSquare(by))
		if T.Sign() <= 0 {
			continue
		}
		tl, th := fdSqrt(T, bits)
		boreLo, boreHi := fdSquareRange(fdSub(tl, c.radius), fdSub(th, c.radius))
		// (S+R²)/den - T-r² - (2R/den)*sqrt(S) + 2r*sqrt(T).
		A := fdSub(new(big.Rat).Quo(fdAdd(S, fdSquare(R)), den), fdAdd(T, fdSquare(c.radius)))
		B := new(big.Rat).Quo(fdMul(big.NewRat(2, 1), R), den)
		C := fdMul(big.NewRat(2, 1), c.radius)
		comparison := fdTwoPositiveRadicalsSign(A, B, S, C, T)
		lo, hi := coneLo, coneHi
		if comparison > 0 {
			lo, hi = boreLo, boreHi
		}
		excluded := []int{}
		separated := true
		for _, f := range base.PerFace {
			if f.Face == q.fillets.Lateral || f.Face == c.wall {
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
		if base.Upper.Cmp(lo) < 0 || q.fillets.Lateral == c.wall {
			return out, geom.ErrConvergence
		}
		vectorScale := new(big.Rat).Quo(rc, S)
		out.Cone = &FilletedFrustumConeWitness{Base: [3]*big.Rat{fdAdd(q.center[0], fdMul(dx, rk)), fdAdd(q.center[1], fdMul(dy, rk)), new(big.Rat).Set(hc)}, RadicalVector: [3]*big.Rat{fdMul(dx, vectorScale), fdMul(dy, vectorScale), new(big.Rat).Set(hk)}, Radicand: new(big.Rat).Set(S), Root: [2]*big.Rat{new(big.Rat).Set(sl), new(big.Rat).Set(sh)}, Face: q.fillets.Lateral}
		out.Bore = &FilletedFrustumBoreWitness{Center: [3]*big.Rat{new(big.Rat).Set(c.center[0]), new(big.Rat).Set(c.center[1]), new(big.Rat).Set(p[2])}, Numerator: [3]*big.Rat{fdMul(c.radius, bx), fdMul(c.radius, by), new(big.Rat)}, Radicand: new(big.Rat).Set(T), Root: [2]*big.Rat{new(big.Rat).Set(tl), new(big.Rat).Set(th)}, Face: c.wall}
		switch {
		case comparison < 0:
			out.Tied = []int{q.fillets.Lateral}
			excluded = append(excluded, c.wall)
		case comparison > 0:
			out.Tied = []int{c.wall}
			excluded = append(excluded, q.fillets.Lateral)
		default:
			out.Tied = []int{q.fillets.Lateral, c.wall}
		}
		sort.Ints(out.Tied)
		sort.Ints(excluded)
		out.Excluded = excluded
		out.Unresolved = nil
		out.Comparison = comparison
		out.Lower = new(big.Rat).Set(lo)
		out.Upper = new(big.Rat).Set(hi)
		out.Complete = true
		return out, nil
	}
	return out, nil
}
