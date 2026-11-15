package trimmed

import (
	"context"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// ConvexAffinePolygonBooleanOptions separates complete original crossing proof
// from material reconstruction limits. Neither stage has an approximate fallback.
type ConvexAffinePolygonBooleanOptions struct {
	Crossing ConvexAffinePolygonCrossingOptions
	Material ConvexAffinePolygonMaterialOptions
}

// CombineConvexAffinePolygonShells is an additive original-native entry point
// for strict transverse convex polygon shell Booleans. It returns one connected
// manifold result, and rejects unsupported coincidence, contact, empty or
// disconnected outcomes. Existing general and rectangular dispatch is unchanged.
func CombineConvexAffinePolygonShells(ctx context.Context, a, b Solid, op parametric.BooleanOp, o ConvexAffinePolygonBooleanOptions) (*ConvexAffinePolygonMaterial, error) {
	operation := intersect.AffineRingIntersection
	switch op {
	case parametric.Intersection:
	case parametric.Union:
		operation = intersect.AffineRingUnion
	case parametric.Difference:
		operation = intersect.AffineRingDifference
	default:
		return nil, geom.ErrInvalid
	}
	crossing, e := CertifyConvexAffinePolygonCrossing(ctx, a, b, o.Crossing)
	if e != nil {
		return nil, e
	}
	return crossing.Combine(ctx, operation, o.Material)
}
