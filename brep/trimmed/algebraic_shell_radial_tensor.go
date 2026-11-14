package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// RadialTubeRegionMassTensor binds full raw second moments and WORLD-ORIGIN
// inertia to the complete original material. Coefficients multiply pi; the
// nested Mass result retains exact centroid and first-moment information.
type RadialTubeRegionMassTensor struct {
	Tensor     geom.RadialGraphMassTensor
	Components []RadialTubeRegionVolumeComponent
}

// RadialTubeRegionMassTensor uses the same complete original-band ownership
// proof as BoundRadialTubeRegionVolume. Original graph degrees are at most8 per
// axis, with positive constant denominators; polynomial height products are
// bounded at degree16 per axis and radial integrands at degree20. The default
// 8M operation quota covers support recovery, ownership and all integration.
// Errors publish no partial tensor or component list.
func (s AlgebraicShellBoolean) RadialTubeRegionMassTensor(ctx context.Context, center [2]*big.Rat, radii []*big.Rat, o geom.RadialGraphMomentOptions) (RadialTubeRegionMassTensor, error) {
	zero := RadialTubeRegionMassTensor{}
	bound, e := s.nativeRadialRegions(ctx, center, radii, geom.RadialGraphVolumeOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	o.MaxOperations = bound.options.MaxOperations - bound.work
	o.MaxCoefficientBits = bound.options.MaxCoefficientBits
	tensor, e := bound.proof.MassTensorBands(ctx, bound.bands, o)
	if e != nil {
		return zero, e
	}
	tensor.ExactOperations += bound.work
	return RadialTubeRegionMassTensor{Tensor: tensor, Components: bound.components}, nil
}
