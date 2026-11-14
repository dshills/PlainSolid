package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// FilletedFrustumOccurrenceDistance retains the original proof and expresses
// its exact rational boundary witness in world coordinates. Proper rigid
// placement leaves squared bounds, owners and nearest/nonunique semantics
// unchanged. Original coordinates are explicitly still in the source frame.
type FilletedFrustumOccurrenceDistance struct {
	Original         FilletedFrustumDistance
	Lower, Upper     *big.Rat
	WorldWitness     [3]*big.Rat
	WitnessFaces     []int
	Exact, Converged bool
}

// BoundBoundaryDistanceRational uses exact inverse placement, not rounded
// support reconstruction or a heuristic scale factor. Transform intermediates
// have the occurrence's4096-bit limit, independently of source work limits.
func (q FilletedFrustumOccurrence) BoundBoundaryDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumDistanceOptions) (out FilletedFrustumOccurrenceDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundBoundaryDistanceRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	world, e := q.pose.TransformPoint(ctx, original.Witness, false, 4096)
	if e != nil {
		return out, e
	}
	return FilletedFrustumOccurrenceDistance{Original: original, Lower: new(big.Rat).Set(original.Lower), Upper: new(big.Rat).Set(original.Upper), WorldWitness: world, WitnessFaces: append([]int(nil), original.WitnessFaces...), Exact: original.Exact, Converged: original.Converged}, nil
}

// FilletedFrustumWorldRimWitness denotes the exact world point
// Center + RadialNumerator/sqrt(AzimuthSquared), with the positive square root
// enclosed in AzimuthRoot. It is on the circle with center Center, radius
// Radius, and unit ring Normal (not an outward material normal). The
// numerator includes radius; no irrational
// coordinate is converted to a represented point. Faces/RimEdges retain the
// original native topology, not a newly sewn world-space B-rep.
type FilletedFrustumWorldRimWitness struct {
	Center, RadialNumerator, Normal [3]*big.Rat
	Radius, AzimuthSquared          *big.Rat
	AzimuthRoot                     [2]*big.Rat
	Faces, RimEdges                 []int
}

// FilletedFrustumOccurrenceRimDistance selects exactly one world witness.
// WorldRim is nonnil for the selected algebraic circle point; otherwise
// WorldRationalWitness holds the selected rational baseline point. The source
// nearest proof is preserved without promoting interval-only cases.
type FilletedFrustumOccurrenceRimDistance struct {
	Original                 FilletedFrustumRimDistance
	Lower, Upper             *big.Rat
	WorldRationalWitness     [3]*big.Rat
	WorldRim                 *FilletedFrustumWorldRimWitness
	WitnessFaces             []int
	NearestProven, Converged bool
}

// BoundBoundaryRimDistanceRational transforms the source algebraic witness
// symbolically. Exact proper orthonormality proves both world squared-distance
// invariance and circle/trim ownership; all work and failure remains bounded.
func (q FilletedFrustumOccurrence) BoundBoundaryRimDistanceRational(ctx context.Context, p [3]*big.Rat, o FilletedFrustumRimDistanceOptions) (out FilletedFrustumOccurrenceRimDistance, err error) {
	defer func() {
		if err != nil {
			out = FilletedFrustumOccurrenceRimDistance{}
		}
	}()
	if q.source.recipe == nil {
		return out, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return out, e
	}
	original, e := q.source.BoundBoundaryRimDistanceRational(ctx, local, o)
	if e != nil {
		return out, e
	}
	out.Original = original
	out.Lower = new(big.Rat).Set(original.Lower)
	out.Upper = new(big.Rat).Set(original.Upper)
	out.NearestProven = original.NearestProven
	out.Converged = original.Converged
	if original.Rim == nil {
		out.WorldRationalWitness, e = q.pose.TransformPoint(ctx, original.Baseline.Witness, false, 4096)
		if e != nil {
			return out, e
		}
		out.WitnessFaces = append([]int(nil), original.Baseline.WitnessFaces...)
		return out, nil
	}
	rim := original.Rim
	center, e := q.pose.TransformPoint(ctx, [3]*big.Rat{rim.Center[0], rim.Center[1], rim.Z}, false, 4096)
	if e != nil {
		return out, e
	}
	numerator, e := q.pose.TransformVector(ctx, [3]*big.Rat{fdMul(rim.Radius, rim.Azimuth[0]), fdMul(rim.Radius, rim.Azimuth[1]), new(big.Rat)}, false, 4096)
	if e != nil {
		return out, e
	}
	normal, e := q.pose.TransformVector(ctx, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, false, 4096)
	if e != nil {
		return out, e
	}
	out.WorldRim = &FilletedFrustumWorldRimWitness{Center: center, RadialNumerator: numerator, Normal: normal, Radius: new(big.Rat).Set(rim.Radius), AzimuthSquared: new(big.Rat).Set(rim.AzimuthSquared), AzimuthRoot: [2]*big.Rat{new(big.Rat).Set(rim.AzimuthRoot[0]), new(big.Rat).Set(rim.AzimuthRoot[1])}, Faces: append([]int(nil), rim.Faces...), RimEdges: append([]int(nil), rim.RimEdges...)}
	out.WitnessFaces = append([]int(nil), rim.Faces...)
	return out, nil
}
