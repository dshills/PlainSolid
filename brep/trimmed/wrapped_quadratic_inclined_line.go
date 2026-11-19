//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// IntersectInclinedLine solves the exact XY shadow of a finite represented
// model line, then recovers original V independently at every rational t.
// Direction may rise, fall or be horizontal, but must have nonzero XY part;
// vertical support-coincident/grazing domains remain explicitly unsupported.
// Unlike IntersectHorizontalLine's legacy extraction contract, rational hit V
// need not be representable: the reference iso is at original represented VMin
// and each actual point is corroborated by exact rational surface evaluation.
// All shadow roots must be rational/simple even if clipped out later. Exact
// finite limits, trim holes, periodic occurrences and oriented normals retain
// the common result contract. ExcludedByHeight counts finite shadow contacts
// beyond the original support; these are not actual face contacts.
func (r *WrappedQuadraticFace) IntersectInclinedLine(ctx context.Context, origin, direction geom.Vec3, limits [2]*big.Rat, o geom.SurfacePatchOptions) (WrappedCylinderFaceLineIntersection, error) {
	return r.intersectCylinderLine(ctx, origin, direction, limits, o, true)
}

// IntersectInclinedLineRational applies the same complete source and trim
// tests to an exact rational line. Original line and UV parameters are never
// converted to binary64. Vertical and non-simple shadow contacts remain
// outside this query's supported scope.
func (r *WrappedQuadraticFace) IntersectInclinedLineRational(ctx context.Context, origin, direction [3]*big.Rat,
	limits [2]*big.Rat, o geom.SurfacePatchOptions) (WrappedCylinderFaceLineIntersection, error) {
	return r.intersectCylinderLineRational(ctx, origin, direction, limits, o, true)
}
