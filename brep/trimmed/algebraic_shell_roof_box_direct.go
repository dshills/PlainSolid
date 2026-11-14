package trimmed

import (
	"context"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// BoundRoofBoxVolume independently validates the original roof boxes and
// encloses their regularized Boolean volume. It also accepts coincident
// boundary areas. The operation does not construct a result topology.
func BoundRoofBoxVolume(ctx context.Context, a, b Solid, op parametric.BooleanOp, proof MixedRoofQueryOptions, options MixedRoofVolumeOptions) (MixedRoofVolume, error) {
	q, err := prepareDirectRoofBoxMeasure(ctx, a, b, op, proof)
	if err != nil {
		return MixedRoofVolume{}, err
	}
	return q.BoundVolume(ctx, options)
}

// BoundRoofBoxMass independently validates the original roof boxes and
// encloses all ten raw material moments through degree two. Equal-height
// roofs or shared vertical walls need no choice of boundary ownership because
// the integrations use the original positive-volume column material.
func BoundRoofBoxMass(ctx context.Context, a, b Solid, op parametric.BooleanOp, proof MixedRoofQueryOptions, options MixedRoofMassOptions) (MixedRoofMass, error) {
	q, err := prepareDirectRoofBoxMeasure(ctx, a, b, op, proof)
	if err != nil {
		return MixedRoofMass{}, err
	}
	return q.BoundMass(ctx, options)
}

func prepareDirectRoofBoxMeasure(ctx context.Context, a, b Solid, op parametric.BooleanOp, proof MixedRoofQueryOptions) (*MixedRoofMaterialQuery, error) {
	switch op {
	case parametric.Intersection, parametric.Union, parametric.Difference:
	default:
		return nil, geom.ErrInvalid
	}
	return prepareRoofMaterialQuery(ctx, [2]Solid{a, b}, op, proof)
}
