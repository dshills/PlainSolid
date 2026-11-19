//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// PhysicalRadialTubeRegionMassTensor returns exact full physical second moments
// and world-origin inertia for the immutable Boolean's complete radial regions.
// It selects bands using the same original wire, support and signed material
// checks as PhysicalRadialTubeRegionMassMoments; no caller band list is accepted.
// Original degrees are at most8 per axis, height products at most16, and radial
// integrands at most20. Recovery and integration share the cumulative quota.
// Errors publish neither a partial tensor nor a partial owner list.
func (s AlgebraicShellBoolean) PhysicalRadialTubeRegionMassTensor(ctx context.Context, center [2]*big.Rat, radii []*big.Rat, o geom.RadialGraphMomentOptions) (RadialTubeRegionMassTensor, error) {
	zero := RadialTubeRegionMassTensor{}
	bound, e := s.nativeRadialRegionsWithCharts(ctx, center, radii, geom.RadialGraphVolumeOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}, true)
	if e != nil {
		return zero, e
	}
	o.MaxOperations = bound.options.MaxOperations - bound.work
	o.MaxCoefficientBits = bound.options.MaxCoefficientBits
	tensor, e := bound.affineProof.MassTensorBands(ctx, bound.bands, o)
	if e != nil {
		return zero, e
	}
	tensor.ExactOperations += bound.work
	return RadialTubeRegionMassTensor{Tensor: tensor, Components: bound.components}, nil
}
