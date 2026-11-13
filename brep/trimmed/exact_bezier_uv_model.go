package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ModelCurveCells composes the trim with its original support. It retains
// exact algebraic source-knot parameters and does not fit a new model curve.
func (q ExactBezierUV) ModelCurveCells(ctx context.Context, p *geom.SurfacePatch, o geom.SurfaceCurveAgreementOptions) (geom.SurfaceCurveCompositionCells, error) {
	if p == nil {
		return geom.SurfaceCurveCompositionCells{}, geom.ErrInvalid
	}
	c, e := q.descriptor(ctx, o.SurfacePatchOptions)
	if e != nil {
		return geom.SurfaceCurveCompositionCells{}, e
	}
	return p.BuildSurfaceCurveCompositionCells(ctx, c, o)
}

// ExactBezierModelPlaneSection has the same ownership and exact-parameter
// contract as the existing exact quadratic model-plane section.
type ExactBezierModelPlaneSection = ExactQuadraticAlgebraicPlaneSection

// IntersectModelRawPlane proves every contact with the represented raw model
// plane. It merges equal events on shared knot cells by algebraic identity.
func (q ExactBezierUV) IntersectModelRawPlane(ctx context.Context, p *geom.SurfacePatch, plane [4]*big.Rat,
	compose geom.SurfaceCurveAgreementOptions, o geom.ExactCurvePlaneEventOptions) (ExactBezierModelPlaneSection, error) {
	c, e := q.ModelCurveCells(ctx, p, compose)
	if e != nil {
		return ExactBezierModelPlaneSection{}, e
	}
	return intersectModelCurveCellsRawPlane(ctx, c, plane, o)
}

// ExactBezierModelLineSection carries isolated or coincident exact contacts
// in original trim-parameter order.
type ExactBezierModelLineSection = ExactQuadraticModelLineSection

// IntersectModelFiniteLine proves every contact with origin+t*direction in
// finite closed rational limits. The direction is used without normalization.
func (q ExactBezierUV) IntersectModelFiniteLine(ctx context.Context, p *geom.SurfacePatch, origin, direction geom.Vec3,
	limits [2]*big.Rat, compose geom.SurfaceCurveAgreementOptions, o geom.ComposedCurveLineOptions) (ExactBezierModelLineSection, error) {
	c, e := q.ModelCurveCells(ctx, p, compose)
	if e != nil {
		return ExactBezierModelLineSection{}, e
	}
	return intersectModelCurveCellsFiniteLine(ctx, c, origin, direction, limits, o)
}
