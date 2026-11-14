package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// RadialTubeRegionPlanarMoments binds projected-area and volume-weighted XY
// moments to every retained original material component. Components follow
// native outer-branch order, while Moments.Bands follow increasing radius.
// All measures except CentroidXY are exact rational coefficients of pi.
type RadialTubeRegionPlanarMoments struct {
	Moments    geom.RadialGraphPlanarMoments
	Components []RadialTubeRegionVolumeComponent
}

// RadialTubeRegionPlanarMoments selects bands exclusively from the complete
// original material, using the same ownership, orientation and constant-height-
// denominator proof as BoundRadialTubeRegionVolume. Center/radii are proposals
// rechecked against original coefficients; no caller-selected bands or public
// diagnostic tables control the result. Arithmetic limits are cumulative across
// recovery and integration (default8M operations). Results are atomic.
// Projected area is not curved boundary area; no Z centroid is asserted.
func (s AlgebraicShellBoolean) RadialTubeRegionPlanarMoments(ctx context.Context, center [2]*big.Rat, radii []*big.Rat, o geom.RadialGraphMomentOptions) (RadialTubeRegionPlanarMoments, error) {
	zero := RadialTubeRegionPlanarMoments{}
	bound, e := s.nativeRadialRegions(ctx, center, radii, geom.RadialGraphVolumeOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	o.MaxOperations = bound.options.MaxOperations - bound.work
	o.MaxCoefficientBits = bound.options.MaxCoefficientBits
	m, e := bound.proof.PlanarMomentsBands(ctx, bound.bands, o)
	if e != nil {
		return zero, e
	}
	m.ExactOperations += bound.work
	return RadialTubeRegionPlanarMoments{Moments: m, Components: bound.components}, nil
}
