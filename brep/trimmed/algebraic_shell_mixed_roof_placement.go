package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// PlacedRoofMaterialQuery evaluates a proved roof Boolean under one exact
// proper rigid placement of the entire result. Its source charts and face
// IDs remain those of the original operands. Moving an operand independently
// requires a new Boolean proof.
type PlacedRoofMaterialQuery struct {
	source *MixedRoofMaterialQuery
	pose   placement.RationalRigid
}

// Place binds a proved material query to an immutable proper rigid motion.
func (q *MixedRoofMaterialQuery) Place(pose placement.RationalRigid) (PlacedRoofMaterialQuery, error) {
	if q == nil || q.box[0] == nil || q.box[1] == nil {
		return PlacedRoofMaterialQuery{}, geom.ErrInvalid
	}
	if _, err := pose.Matrix(); err != nil {
		return PlacedRoofMaterialQuery{}, err
	}
	return PlacedRoofMaterialQuery{source: q, pose: pose}, nil
}

const roofPosePointOperations = 21 // three translations, nine products, nine sums.
const roofPoseVectorOperations = 18

// BoundBoundaryDistanceRational encloses the original material's world
// distance under a proper rigid placement. A returned rational witness is
// mapped to world coordinates; source owner IDs stay in the original shells.
func (q PlacedRoofMaterialQuery) BoundBoundaryDistanceRational(ctx context.Context, world [3]*big.Rat, o MixedRoofDistanceOptions) (MixedRoofDistance, error) {
	var zero MixedRoofDistance
	var err error
	o.SurfacePatchEvaluationOptions, err = q.source.evaluationOptions(ctx, o.SurfacePatchEvaluationOptions, world[:]...)
	if err != nil {
		return zero, err
	}
	const cost = 2 * roofPosePointOperations
	o.MaxOperations -= cost
	if o.MaxOperations < 1 {
		return zero, geom.ErrConvergence
	}
	p, err := q.pose.TransformPoint(ctx, world, true, o.MaxCoefficientBits)
	if err != nil {
		return zero, err
	}
	out, queryErr := q.source.BoundBoundaryDistanceRational(ctx, p, o)
	if queryErr != nil && !errors.Is(queryErr, geom.ErrConvergence) {
		return zero, queryErr
	}
	if out.Witness[0] != nil {
		out.Witness, err = q.pose.TransformPoint(ctx, out.Witness, false, o.MaxCoefficientBits)
		if err != nil {
			return zero, err
		}
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations += cost
	return out, queryErr
}

// LocateRational classifies an exact world-space point. Work includes a fixed
// charge for inverse placement plus the original graph evaluation.
func (q PlacedRoofMaterialQuery) LocateRational(ctx context.Context, world [3]*big.Rat, o geom.SurfacePatchEvaluationOptions) (MixedRoofMaterialPoint, error) {
	o, err := q.source.evaluationOptions(ctx, o, world[:]...)
	if err != nil {
		return MixedRoofMaterialPoint{}, err
	}
	o.MaxOperations -= roofPosePointOperations
	if o.MaxOperations < 1 {
		return MixedRoofMaterialPoint{}, geom.ErrConvergence
	}
	p, err := q.pose.TransformPoint(ctx, world, true, o.MaxCoefficientBits)
	if err != nil {
		return MixedRoofMaterialPoint{}, err
	}
	out, err := q.source.LocateRational(ctx, p, o)
	if err != nil {
		return MixedRoofMaterialPoint{}, err
	}
	out.ExactOperations += roofPosePointOperations
	return out, nil
}

// IntersectLineRational takes an exact world-space line. Pulling back the
// origin and direction separately preserves its original parameter units,
// including clipping, repeated roots, overlaps and original face ownership.
func (q PlacedRoofMaterialQuery) IntersectLineRational(ctx context.Context, origin, direction [3]*big.Rat, minT, maxT *big.Rat, o MixedRoofLineOptions) (MixedRoofLineSection, error) {
	var err error
	o.SurfacePatchEvaluationOptions, err = q.source.evaluationOptions(ctx, o.SurfacePatchEvaluationOptions, origin[0], origin[1], origin[2], direction[0], direction[1], direction[2], minT, maxT)
	if err != nil {
		return MixedRoofLineSection{}, err
	}
	const cost = roofPosePointOperations + roofPoseVectorOperations
	o.MaxOperations -= cost
	if o.MaxOperations < 1 {
		return MixedRoofLineSection{}, geom.ErrConvergence
	}
	p, err := q.pose.TransformPoint(ctx, origin, true, o.MaxCoefficientBits)
	if err != nil {
		return MixedRoofLineSection{}, err
	}
	d, err := q.pose.TransformVector(ctx, direction, true, o.MaxCoefficientBits)
	if err != nil {
		return MixedRoofLineSection{}, err
	}
	out, err := q.source.IntersectLineRational(ctx, p, d, minT, maxT, o)
	if err != nil {
		return MixedRoofLineSection{}, err
	}
	out.ExactOperations += cost
	return out, nil
}
