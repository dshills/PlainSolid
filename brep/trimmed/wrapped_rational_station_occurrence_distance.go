package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedRationalStationOccurrenceDistance keeps independent original and
// world boundary evidence. Every squared bound and original face ID is
// identical; all World witnesses are exact transformed original support
// points. Faces describes ownership of the attained upper witness, not an
// assertion that all nearest faces have been isolated.
type WrappedRationalStationOccurrenceDistance struct {
	Original, World WrappedRationalStationDistance
}

// BoundBoundaryDistanceRational invokes the authenticated original boundary
// query exactly once, then transports its global and per-face witnesses through
// the private proved proper pose. Orthogonality establishes exact squared
// distance invariance, including conservative lower bounds; no scaled tolerance
// or rounded surrogate is used. The original search/point/source budgets remain
// unchanged. At most20 fixed-size3x4 transformations (16 walls, two caps, global
// witness and inverse query) use the occurrence's separate bit limit. Counts in
// Original and World describe that single source search. Failure at any stage
// returns neither a partial original report nor a partial world report.
func (q WrappedRationalStationOccurrence) BoundBoundaryDistanceRational(ctx context.Context, p [3]*big.Rat, o WrappedRationalStationDistanceOptions) (WrappedRationalStationOccurrenceDistance, error) {
	empty := WrappedRationalStationOccurrenceDistance{}
	if q.bits == 0 {
		return empty, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, q.bits)
	if e != nil {
		return empty, e
	}
	original, e := q.source.BoundWrappedRationalStationBoundaryDistanceRational(ctx, local, o)
	if e != nil {
		return empty, e
	}
	witness, e := q.pose.TransformPoint(ctx, original.Witness, false, q.bits)
	if e != nil {
		return empty, e
	}
	world := WrappedRationalStationDistance{Lower: new(big.Rat).Set(original.Lower), Upper: new(big.Rat).Set(original.Upper), Witness: witness, Faces: append([]int(nil), original.Faces...), VisitedCells: original.VisitedCells, ExactOperations: original.ExactOperations}
	for _, face := range original.PerFace {
		witness, e := q.pose.TransformPoint(ctx, face.Witness, false, q.bits)
		if e != nil {
			return empty, e
		}
		world.PerFace = append(world.PerFace, WrappedRationalStationFaceDistance{Face: face.Face, Lower: new(big.Rat).Set(face.Lower), Upper: new(big.Rat).Set(face.Upper), Witness: witness})
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return WrappedRationalStationOccurrenceDistance{Original: original, World: world}, nil
}
