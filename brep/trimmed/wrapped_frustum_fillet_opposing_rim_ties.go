package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

type FilletedFrustumOpposingRimOptions struct {
	Support FilletedFrustumDistanceOptions
	// Zero selects 120, the maximum unordered pair count for 16 native cuts.
	MaxPairs int
}

// FilletedFrustumOpposingRimTies proves two distinct nearest rim points with
// four original adjoining owners. Complete requires same exact center/radius,
// opposite blind ends, exact axial midpoint, outside-disk query, and strict
// exclusion of EVERY other face. Otherwise Tied/Rims remain empty and all
// original faces Unresolved. Baseline remains independently conservative.
type FilletedFrustumOpposingRimTies struct {
	Baseline                   FilletedFrustumDistance
	Rims                       [2]*FilletedFrustumAlgebraicRimWitness
	Lower, Upper               *big.Rat
	Tied, Excluded, Unresolved []int
	Complete                   bool
	PairChecks                 int
}

// ClassifyOpposingBlindRimsRational authenticates the shared symbolic squared
// distance S+r²+h²-2r*sqrt(S) from original immutable cut inputs, not overlapping
// numerical intervals. Each disk and finite wall independently attains its rim.
// Uses one bounded support query; pair exhaustion returns no partial result.
func (q FilletedFrustumMaterial) ClassifyOpposingBlindRimsRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumOpposingRimOptions) (out FilletedFrustumOpposingRimTies, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOpposingRimTies{}
		}
	}()
	if o.MaxPairs == 0 {
		o.MaxPairs = 120
	}
	if o.MaxPairs < 1 || o.MaxPairs > 120 {
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
	for i := 0; i < len(q.cuts); i++ {
		for j := i + 1; j < len(q.cuts); j++ {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if out.PairChecks >= o.MaxPairs {
				return out, geom.ErrConvergence
			}
			out.PairChecks++
			a, b := q.cuts[i], q.cuts[j]
			if a.floor < 0 || b.floor < 0 {
				continue
			}
			if !a.open[0] {
				a, b = b, a
			}
			if !a.open[0] || a.open[1] || b.open[0] || !b.open[1] || a.z[1].Cmp(b.z[0]) >= 0 || a.radius.Cmp(b.radius) != 0 || a.center[0].Cmp(b.center[0]) != 0 || a.center[1].Cmp(b.center[1]) != 0 {
				continue
			}
			if fdMul(p[2], big.NewRat(2, 1)).Cmp(fdAdd(a.z[1], b.z[0])) != 0 {
				continue
			}
			dx, dy := fdSub(p[0], a.center[0]), fdSub(p[1], a.center[1])
			S := fdAdd(fdSquare(dx), fdSquare(dy))
			if S.Cmp(fdSquare(a.radius)) <= 0 {
				continue
			}
			lo, hi := fdSqrt(S, bits)
			lower, upper := fdSquareRange(fdSub(lo, a.radius), fdSub(hi, a.radius))
			h2 := fdSquare(fdSub(p[2], a.z[1]))
			lower.Add(lower, h2)
			upper.Add(upper, h2)
			owners := []int{a.wall, a.floor, b.wall, b.floor}
			sort.Ints(owners)
			duplicate := false
			for k := 1; k < 4; k++ {
				duplicate = duplicate || owners[k] == owners[k-1]
			}
			if duplicate {
				return out, geom.ErrConvergence
			}
			excluded := []int{}
			separated := true
			for _, f := range base.PerFace {
				own := false
				for _, id := range owners {
					own = own || id == f.Face
				}
				if !own {
					if f.SquaredLower.Cmp(upper) <= 0 {
						separated = false
						break
					}
					excluded = append(excluded, f.Face)
				}
			}
			if !separated {
				continue
			}
			if base.Upper.Cmp(lower) < 0 {
				return out, geom.ErrConvergence
			}
			for k, c := range []filletedFrustumMaterialCut{a, b} {
				z := a.z[1]
				if k == 1 {
					z = b.z[0]
				}
				faces := []int{c.wall, c.floor}
				sort.Ints(faces)
				edges := q.solid.filletedSharedRimEdges(c.wall, c.floor)
				if len(edges) != 4 {
					return out, geom.ErrConvergence
				}
				out.Rims[k] = &FilletedFrustumAlgebraicRimWitness{Center: [2]*big.Rat{new(big.Rat).Set(c.center[0]), new(big.Rat).Set(c.center[1])}, Radius: new(big.Rat).Set(c.radius), Z: new(big.Rat).Set(z), Azimuth: [2]*big.Rat{new(big.Rat).Set(dx), new(big.Rat).Set(dy)}, AzimuthSquared: new(big.Rat).Set(S), AzimuthRoot: [2]*big.Rat{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}, Faces: faces, RimEdges: edges}
			}
			sort.Ints(excluded)
			out.Lower = lower
			out.Upper = upper
			out.Tied = owners
			out.Excluded = excluded
			out.Unresolved = nil
			out.Complete = true
			return out, nil
		}
	}
	return out, nil
}

type FilletedFrustumOccurrenceOpposingRimTies struct {
	Original  FilletedFrustumOpposingRimTies
	WorldRims [2]*FilletedFrustumWorldRimWitness
}

// ClassifyOpposingBlindRimsRational preserves exact common distance and both
// source owner pairs through proper rational rigid placement; one source query.
func (q FilletedFrustumOccurrence) ClassifyOpposingBlindRimsRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumOpposingRimOptions) (out FilletedFrustumOccurrenceOpposingRimTies, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceOpposingRimTies{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	source, e := q.source.ClassifyOpposingBlindRimsRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = source
	if !source.Complete {
		return out, nil
	}
	for i, w := range source.Rims {
		placed, e := q.transformFilletedRimDistance(ctx, FilletedFrustumRimDistance{Lower: source.Lower, Upper: source.Upper, Rim: w, NearestProven: true})
		if e != nil {
			return out, e
		}
		out.WorldRims[i] = placed.WorldRim
	}
	return out, nil
}
