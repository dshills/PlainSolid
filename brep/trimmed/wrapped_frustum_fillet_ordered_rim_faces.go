package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// FilletedFrustumOrderedRimFaces classifies all original faces when opposing
// coaxial blind-floor rim minima can be ordered in one exact quadratic field.
// Rims contains the two independently attained candidates, ordered lower/upper
// floor; only Tied attains the global minimum. Comparison is the exact sign of
// lower-rim distance minus upper-rim distance. Complete requires strict exclusion
// of all unrelated faces. On unresolved cases no candidates are published.
type FilletedFrustumOrderedRimFaces struct {
	Baseline                   FilletedFrustumDistance
	Rims                       [2]*FilletedFrustumAlgebraicRimWitness
	Lower, Upper               *big.Rat
	Tied, Excluded, Unresolved []int
	Complete                   bool
	PairChecks                 int
	Comparison                 int
}

// ClassifyOrderedBlindRimsRational accepts a query strictly between two
// opposing coaxial blind floors and radially outside both disks. Each adjoining
// disk and finite wall independently minimizes at its own rim. The difference
// of the squared minima is A-B*sqrt(S), whose sign is exact even when distance
// enclosures overlap. Other supports still require strict interval separation.
// One bounded support query and at most MaxPairs pairs; errors are atomic.
func (q FilletedFrustumMaterial) ClassifyOrderedBlindRimsRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumOpposingRimOptions) (out FilletedFrustumOrderedRimFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOrderedRimFaces{}
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
			if !a.open[0] || a.open[1] || b.open[0] || !b.open[1] || a.z[1].Cmp(b.z[0]) >= 0 || a.center[0].Cmp(b.center[0]) != 0 || a.center[1].Cmp(b.center[1]) != 0 {
				continue
			}
			if p[2].Cmp(a.z[1]) <= 0 || p[2].Cmp(b.z[0]) >= 0 {
				continue
			}
			dx, dy := fdSub(p[0], a.center[0]), fdSub(p[1], a.center[1])
			S := fdAdd(fdSquare(dx), fdSquare(dy))
			if S.Cmp(fdSquare(a.radius)) <= 0 || S.Cmp(fdSquare(b.radius)) <= 0 {
				continue
			}
			lo, hi := fdSqrt(S, bits)
			hA, hB := fdSquare(fdSub(p[2], a.z[1])), fdSquare(fdSub(p[2], b.z[0]))
			constant := fdSub(fdAdd(fdSquare(a.radius), hA), fdAdd(fdSquare(b.radius), hB))
			coefficient := fdMul(big.NewRat(2, 1), fdSub(a.radius, b.radius))
			comparison := fdRadicalSign(constant, coefficient, S)
			radius, h2 := a.radius, hA
			if comparison > 0 {
				radius, h2 = b.radius, hB
			}
			lower, upper := fdSquareRange(fdSub(lo, radius), fdSub(hi, radius))
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
			out.Lower = lower
			out.Upper = upper
			out.Comparison = comparison
			switch {
			case comparison < 0:
				out.Tied = []int{a.wall, a.floor}
				excluded = append(excluded, b.wall, b.floor)
			case comparison > 0:
				out.Tied = []int{b.wall, b.floor}
				excluded = append(excluded, a.wall, a.floor)
			default:
				out.Tied = owners
			}
			sort.Ints(out.Tied)
			sort.Ints(excluded)
			out.Excluded = excluded
			out.Unresolved = nil
			out.Complete = true
			return out, nil
		}
	}
	return out, nil
}

// FilletedFrustumOccurrenceOrderedRimFaces maps both retained candidates.
// Original.Tied selects the nearest owners; a losing WorldRim is not a
// nearest witness. The proper rigid map preserves the exact ordering.
type FilletedFrustumOccurrenceOrderedRimFaces struct {
	Original  FilletedFrustumOrderedRimFaces
	WorldRims [2]*FilletedFrustumWorldRimWitness
}

// ClassifyOrderedBlindRimsRational preserves exact ordered distance and both
// source owner pairs through proper rational rigid placement; one source query.
func (q FilletedFrustumOccurrence) ClassifyOrderedBlindRimsRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumOpposingRimOptions) (out FilletedFrustumOccurrenceOrderedRimFaces, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceOrderedRimFaces{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	source, e := q.source.ClassifyOrderedBlindRimsRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = source
	if !source.Complete {
		return out, nil
	}
	for i, w := range source.Rims {
		placed, e := q.transformFilletedRimDistance(ctx, FilletedFrustumRimDistance{Lower: source.Lower, Upper: source.Upper, Rim: w})
		if e != nil {
			return out, e
		}
		out.WorldRims[i] = placed.WorldRim
	}
	return out, nil
}
