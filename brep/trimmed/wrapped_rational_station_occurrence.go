package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// WrappedRationalStationOccurrenceOptions keeps source authentication and
// exact pose arithmetic separate. MaxTransformBits defaults to16384 and is
// bounded by65536. Each query uses at most four fixed-size3x4 transformations;
// original point/normal queries retain their own explicit budgets.
type WrappedRationalStationOccurrenceOptions struct {
	Source           geom.SurfacePatchOptions
	MaxTransformBits int
}

// WrappedRationalStationOccurrence is a proper rational rigid occurrence of
// an authenticated original weighted-profile loft. Source supports, trims and
// IDs remain in their original frame. This is not a rounded world-space B-rep.
// The zero value is invalid. Source and pose are immutable.
type WrappedRationalStationOccurrence struct {
	source Solid
	pose   placement.RationalRigid
	bits   int
}

// PlaceWrappedRationalStation authenticates the native source and retains its
// exact proper rigid pose without normalizing directions through binary64.
func (s Solid) PlaceWrappedRationalStation(ctx context.Context, pose placement.RationalRigid, o WrappedRationalStationOccurrenceOptions) (WrappedRationalStationOccurrence, error) {
	empty := WrappedRationalStationOccurrence{}
	if o.MaxTransformBits == 0 {
		o.MaxTransformBits = 16384
	}
	if ctx == nil || o.MaxTransformBits < 64 || o.MaxTransformBits > 65536 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	matrix, e := pose.Matrix()
	if e != nil {
		return empty, e
	}
	checked, e := placement.NewRationalRigid(matrix, o.MaxTransformBits)
	if e != nil {
		return empty, e
	}
	if _, _, e = s.validateWrappedProfileStationLoft(ctx, o.Source, false); e != nil {
		return empty, e
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return WrappedRationalStationOccurrence{source: s, pose: checked, bits: o.MaxTransformBits}, nil
}

// Source returns the immutable unplaced native source with original IDs.
func (q WrappedRationalStationOccurrence) Source() Solid { return q.source }

// Pose returns an independently owned original-to-world exact matrix.
func (q WrappedRationalStationOccurrence) Pose() ([3][4]*big.Rat, error) { return q.pose.Matrix() }

// LocateRational pulls the world point back exactly. Material classification
// and every original cap/rim/crease owner are invariant under the proved pose.
func (q WrappedRationalStationOccurrence) LocateRational(ctx context.Context, p [3]*big.Rat, o WrappedRationalStationPointOptions) (WrappedRationalStationPoint, error) {
	empty := WrappedRationalStationPoint{}
	if q.bits == 0 {
		return empty, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, q.bits)
	if e != nil {
		return empty, e
	}
	return q.source.LocateWrappedRationalStationLoftRational(ctx, local, o)
}

// WrappedRationalStationOccurrenceNormals retains independent original and
// world evidence. Normals have original face IDs and exact world directions;
// squared lengths are invariant. No unit-vector rounding or crease averaging
// occurs. The original and world rational values do not alias one another.
type WrappedRationalStationOccurrenceNormals struct {
	Original WrappedRationalStationNormals
	Point    [3]*big.Rat
	Normals  []WrappedRationalStationFaceNormal
}

// BoundaryNormalsRational rotates every original outward normal exactly.
// Proper orthogonality proves tangent orthogonality, outwardness and squared
// length preservation. Translation affects only Point. Transformation failure
// after an original query still returns an entirely empty result.
func (q WrappedRationalStationOccurrence) BoundaryNormalsRational(ctx context.Context, p [3]*big.Rat, o WrappedRationalStationNormalOptions) (WrappedRationalStationOccurrenceNormals, error) {
	empty := WrappedRationalStationOccurrenceNormals{}
	if q.bits == 0 {
		return empty, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, q.bits)
	if e != nil {
		return empty, e
	}
	original, e := q.source.WrappedRationalStationBoundaryNormalsRational(ctx, local, o)
	if e != nil {
		return empty, e
	}
	world, e := q.pose.TransformPoint(ctx, original.Point, false, q.bits)
	if e != nil {
		return empty, e
	}
	result := WrappedRationalStationOccurrenceNormals{Original: original, Point: world}
	for _, n := range original.Normals {
		direction, e := q.pose.TransformVector(ctx, n.Direction, false, q.bits)
		if e != nil {
			return empty, e
		}
		result.Normals = append(result.Normals, WrappedRationalStationFaceNormal{Face: n.Face, Direction: direction, SquaredLength: new(big.Rat).Set(n.SquaredLength)})
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
