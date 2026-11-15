package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// WrappedAnisotropicStationOccurrence and WrappedAnisotropicBoreOccurrence
// retain an authenticated immutable original solid under an exact proper
// rational rigid pose. They do not construct rounded world-space B-reps.
// All reported face/edge/rim/vertex IDs continue to index Source(). Zero values
// are invalid. Pose arithmetic uses MaxTransformBits (default16384,max65536).
// Source queries retain their own budgets and run exactly once per call.
type WrappedAnisotropicStationOccurrence struct{ source wrappedAnisotropicOccurrence }
type WrappedAnisotropicBoreOccurrence struct{ source wrappedAnisotropicOccurrence }
type wrappedAnisotropicOccurrence struct {
	solid Solid
	pose  placement.RationalRigid
	bits  int
}

func (s Solid) prepareAnisotropicOccurrence(ctx context.Context, pose placement.RationalRigid, o WrappedRationalStationOccurrenceOptions, bore bool) (wrappedAnisotropicOccurrence, error) {
	var empty wrappedAnisotropicOccurrence
	if o.MaxTransformBits == 0 {
		o.MaxTransformBits = 16384
	}
	if ctx == nil || o.MaxTransformBits < 64 || o.MaxTransformBits > 65536 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	m, e := pose.Matrix()
	if e != nil {
		return empty, e
	}
	checked, e := placement.NewRationalRigid(m, o.MaxTransformBits)
	if e != nil {
		return empty, e
	}
	if bore {
		_, _, e = s.validateAnisotropicStationBore(ctx, o.Source)
	} else {
		_, _, e = s.validateWrappedAnisotropicStationLoft(ctx, o.Source)
	}
	if e != nil {
		return empty, e
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return wrappedAnisotropicOccurrence{s, checked, o.MaxTransformBits}, nil
}

func (s Solid) PlaceWrappedAnisotropicStation(ctx context.Context, pose placement.RationalRigid, o WrappedRationalStationOccurrenceOptions) (WrappedAnisotropicStationOccurrence, error) {
	q, e := s.prepareAnisotropicOccurrence(ctx, pose, o, false)
	if e != nil {
		return WrappedAnisotropicStationOccurrence{}, e
	}
	return WrappedAnisotropicStationOccurrence{q}, nil
}
func (s Solid) PlaceWrappedAnisotropicStationBore(ctx context.Context, pose placement.RationalRigid, o WrappedRationalStationOccurrenceOptions) (WrappedAnisotropicBoreOccurrence, error) {
	q, e := s.prepareAnisotropicOccurrence(ctx, pose, o, true)
	if e != nil {
		return WrappedAnisotropicBoreOccurrence{}, e
	}
	return WrappedAnisotropicBoreOccurrence{q}, nil
}
func (q WrappedAnisotropicStationOccurrence) Source() Solid { return q.source.solid }
func (q WrappedAnisotropicBoreOccurrence) Source() Solid    { return q.source.solid }
func (q WrappedAnisotropicStationOccurrence) Pose() ([3][4]*big.Rat, error) {
	return q.source.pose.Matrix()
}
func (q WrappedAnisotropicBoreOccurrence) Pose() ([3][4]*big.Rat, error) {
	return q.source.pose.Matrix()
}

func (q wrappedAnisotropicOccurrence) point(ctx context.Context, p [3]*big.Rat) ([3]*big.Rat, error) {
	if q.bits == 0 {
		return [3]*big.Rat{}, geom.ErrInvalid
	}
	return q.pose.TransformPoint(ctx, p, true, q.bits)
}
func (q wrappedAnisotropicOccurrence) line(ctx context.Context, p, d [3]*big.Rat) ([3]*big.Rat, [3]*big.Rat, error) {
	empty := [3]*big.Rat{}
	local, e := q.point(ctx, p)
	if e != nil {
		return empty, empty, e
	}
	direction, e := q.pose.TransformVector(ctx, d, true, q.bits)
	if e != nil {
		return empty, empty, e
	}
	return local, direction, nil
}

// LocateRational preserves the filled source's complete point owner ledger.
func (q WrappedAnisotropicStationOccurrence) LocateRational(ctx context.Context, p [3]*big.Rat, o WrappedRationalStationPointOptions) (WrappedAnisotropicStationPoint, error) {
	local, e := q.source.point(ctx, p)
	if e != nil {
		return WrappedAnisotropicStationPoint{}, e
	}
	return q.source.solid.LocateWrappedAnisotropicStationRational(ctx, local, o)
}

// LocateRational preserves the bore source's exact material classification.
func (q WrappedAnisotropicBoreOccurrence) LocateRational(ctx context.Context, p [3]*big.Rat, o WrappedRationalStationPointOptions) (parametric.Location, error) {
	local, e := q.source.point(ctx, p)
	if e != nil {
		return parametric.Outside, e
	}
	return q.source.solid.LocateWrappedAnisotropicStationBoreRational(ctx, local, o)
}

// ClipLineRational uses the filled source's nonhorizontal contract: horizontal
// means in original source coordinates. Use ClipHorizontalLineRational or
// ClipCapLineRational for their separate original contracts. No normalization
// changes t, isolating brackets, multiplicity or original ownership.
func (q WrappedAnisotropicStationOccurrence) ClipLineRational(ctx context.Context, p, d [3]*big.Rat, minT, maxT *big.Rat, o WrappedAnisotropicMultiBandLineOptions) (WrappedAnisotropicMultiBandLine, error) {
	local, direction, e := q.source.line(ctx, p, d)
	if e != nil {
		return WrappedAnisotropicMultiBandLine{}, e
	}
	return q.source.solid.ClipWrappedAnisotropicStationLineRational(ctx, local, direction, minT, maxT, o)
}
func (q WrappedAnisotropicStationOccurrence) ClipHorizontalLineRational(ctx context.Context, p, d [3]*big.Rat, minT, maxT *big.Rat, o WrappedRationalStationLineOptions) (WrappedRationalStationLine, error) {
	local, direction, e := q.source.line(ctx, p, d)
	if e != nil {
		return WrappedRationalStationLine{}, e
	}
	return q.source.solid.ClipWrappedAnisotropicStationHorizontalLineRational(ctx, local, direction, minT, maxT, o)
}
func (q WrappedAnisotropicStationOccurrence) ClipCapLineRational(ctx context.Context, p, d [3]*big.Rat, minT, maxT *big.Rat, o WrappedAnisotropicMultiBandLineOptions) (WrappedAnisotropicCapLine, error) {
	local, direction, e := q.source.line(ctx, p, d)
	if e != nil {
		return WrappedAnisotropicCapLine{}, e
	}
	return q.source.solid.ClipWrappedAnisotropicStationCapLineRational(ctx, local, direction, minT, maxT, o)
}
func (q WrappedAnisotropicStationOccurrence) ClipGeneratorRational(ctx context.Context, face int, p, d [3]*big.Rat, minT, maxT *big.Rat, o WrappedAnisotropicGeneratorOptions) (WrappedAnisotropicGeneratorLine, error) {
	local, direction, e := q.source.line(ctx, p, d)
	if e != nil {
		return WrappedAnisotropicGeneratorLine{}, e
	}
	return q.source.solid.ClipWrappedAnisotropicStationGeneratorRational(ctx, face, local, direction, minT, maxT, o)
}

// ClipLineRational preserves the bore's complete material/boundary-span
// contract, including world-oblique original caps and wall generators. Parts
// labels original lower/upper caps, not directions relative to world Z.
func (q WrappedAnisotropicBoreOccurrence) ClipLineRational(ctx context.Context, p, d [3]*big.Rat, minT, maxT *big.Rat, o WrappedAnisotropicMultiBandLineOptions) (WrappedAnisotropicBoreLine, error) {
	local, direction, e := q.source.line(ctx, p, d)
	if e != nil {
		return WrappedAnisotropicBoreLine{}, e
	}
	return q.source.solid.ClipWrappedAnisotropicStationBoreLineRational(ctx, local, direction, minT, maxT, o)
}

// WrappedAnisotropicBoreOccurrenceDistance owns independent local/world
// reports. Bounds, owner IDs and search counts are invariant; only witnesses
// change frame. WitnessFace remains one attained owner, not all nearest ties.
type WrappedAnisotropicBoreOccurrenceDistance struct {
	Original, World WrappedAnisotropicBoreDistance
}

// BoundBoundaryDistanceRational performs one original source query. Exact
// orthogonality proves squared-distance invariance; at most36 fixed-size3x4
// transforms (inverse query, global witness,32 walls/two caps) have the separate
// pose bit limit. Failure during any forward transform discards both reports.
func (q WrappedAnisotropicBoreOccurrence) BoundBoundaryDistanceRational(ctx context.Context, p [3]*big.Rat, o WrappedAnisotropicBoreDistanceOptions) (WrappedAnisotropicBoreOccurrenceDistance, error) {
	var empty WrappedAnisotropicBoreOccurrenceDistance
	local, e := q.source.point(ctx, p)
	if e != nil {
		return empty, e
	}
	original, e := q.source.solid.BoundWrappedAnisotropicStationBoreDistanceRational(ctx, local, o)
	if e != nil {
		return empty, e
	}
	witness, e := q.source.pose.TransformPoint(ctx, original.Witness, false, q.source.bits)
	if e != nil {
		return empty, e
	}
	world := WrappedAnisotropicBoreDistance{Lower: new(big.Rat).Set(original.Lower), Upper: new(big.Rat).Set(original.Upper), Witness: witness, WitnessFace: original.WitnessFace, VisitedCells: original.VisitedCells, ExactOperations: original.ExactOperations, PointOperations: original.PointOperations}
	for _, face := range original.PerFace {
		point, e := q.source.pose.TransformPoint(ctx, face.Witness, false, q.source.bits)
		if e != nil {
			return empty, e
		}
		world.PerFace = append(world.PerFace, WrappedRationalStationFaceDistance{Face: face.Face, Lower: new(big.Rat).Set(face.Lower), Upper: new(big.Rat).Set(face.Upper), Witness: point})
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return WrappedAnisotropicBoreOccurrenceDistance{original, world}, nil
}
