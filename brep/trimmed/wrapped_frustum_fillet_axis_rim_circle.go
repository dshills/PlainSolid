//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// FilletedFrustumNearestRimCircle is a complete circular subset of nearest
// contacts. Circle.Face is the original bore wall; Faces also includes its
// original cap. RimEdges names the full native partition of this circle,
// not an arbitrary chosen seam parameter. The nearest locus may be larger.
type FilletedFrustumNearestRimCircle struct {
	Circle          FilletedFrustumNearestCircle
	Faces, RimEdges []int
}

type FilletedFrustumAxisRimCircleDistance struct {
	Baseline FilletedFrustumDistance
	Rim      *FilletedFrustumNearestRimCircle
}

// BoundEntryRimAxisNearestCircleRational proves the axis query is inside the
// missing entry disk and on its external axial side. The retained holed cap
// and finite cylindrical wall both minimize at every point of the shared rim.
// Other faces must independently separate. One support query and at most32
// immutable endpoint checks are used, with no new root search or sampling.
// Nil Rim retains conservative Baseline and does not mean a unique nearest.
func (q FilletedFrustumMaterial) BoundEntryRimAxisNearestCircleRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (out FilletedFrustumAxisRimCircleDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumAxisRimCircleDistance{}
		}
	}()
	base, e := q.BoundBoundaryDistanceRational(ctx, p, o)
	if e != nil {
		return out, e
	}
	out.Baseline = base
	for _, c := range q.cuts {
		for end, z := range c.z {
			if e := ctx.Err(); e != nil {
				return out, e
			}
			if !c.open[end] || p[0].Cmp(c.center[0]) != 0 || p[1].Cmp(c.center[1]) != 0 || (end == 0 && p[2].Cmp(z) > 0) || (end == 1 && p[2].Cmp(z) < 0) {
				continue
			}
			d := fdAdd(fdSquare(c.radius), fdSquare(fdSub(p[2], z)))
			if !base.Exact || base.Lower.Cmp(d) != 0 {
				continue
			}
			capFace := q.fillets.Caps[end]
			separated := true
			for _, f := range base.PerFace {
				if f.Face != c.wall && f.Face != capFace && f.SquaredLower.Cmp(d) < 0 {
					separated = false
					break
				}
			}
			if !separated {
				continue
			}
			edges := q.solid.filletedSharedRimEdges(c.wall, capFace)
			if len(edges) == 0 {
				return out, geom.ErrConvergence
			}
			faces := []int{c.wall, capFace}
			sort.Ints(faces)
			circle := FilletedFrustumNearestCircle{Center: [3]*big.Rat{new(big.Rat).Set(c.center[0]), new(big.Rat).Set(c.center[1]), new(big.Rat).Set(z)}, U: [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}, V: [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), new(big.Rat)}, Radius: new(big.Rat).Set(c.radius), SquaredDistance: d, Face: c.wall}
			out.Rim = &FilletedFrustumNearestRimCircle{Circle: circle, Faces: faces, RimEdges: edges}
			return out, nil
		}
	}
	return out, nil
}

// FilletedFrustumOccurrenceAxisRimCircleDistance keeps its baseline in the
// original frame. WorldRim has exact placed basis and unchanged native owners.
type FilletedFrustumOccurrenceAxisRimCircleDistance struct {
	Original FilletedFrustumAxisRimCircleDistance
	WorldRim *FilletedFrustumNearestRimCircle
}

// BoundEntryRimAxisNearestCircleRational preserves the circular minimizer
// family by exact proper rigid placement, with atomic 4096-bit pose limits.
func (q FilletedFrustumOccurrence) BoundEntryRimAxisNearestCircleRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (out FilletedFrustumOccurrenceAxisRimCircleDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceAxisRimCircleDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundEntryRimAxisNearestCircleRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = original
	if original.Rim == nil {
		return out, nil
	}
	c := original.Rim.Circle
	center, e := q.pose.TransformPoint(ctx, c.Center, false, 4096)
	if e != nil {
		return out, e
	}
	u, e := q.pose.TransformVector(ctx, c.U, false, 4096)
	if e != nil {
		return out, e
	}
	v, e := q.pose.TransformVector(ctx, c.V, false, 4096)
	if e != nil {
		return out, e
	}
	out.WorldRim = &FilletedFrustumNearestRimCircle{Circle: FilletedFrustumNearestCircle{Center: center, U: u, V: v, Radius: new(big.Rat).Set(c.Radius), SquaredDistance: new(big.Rat).Set(c.SquaredDistance), Face: c.Face}, Faces: append([]int(nil), original.Rim.Faces...), RimEdges: append([]int(nil), original.Rim.RimEdges...)}
	return out, nil
}
