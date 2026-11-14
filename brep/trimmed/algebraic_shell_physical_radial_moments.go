package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// PhysicalRadialTubeRegionMassMoments binds physical-coordinate exact moments
// to the complete immutable Boolean's retained original closed tube regions.
// It accepts signed-axis affine XY source charts with a positive rectangular
// common footprint. Center/radii are proposals re-proved on actual original
// supports, never a caller-selected band list or an asserted certificate.
// Original complete outer/inner wires select adjacent bands; actual face
// orientation and original chart orientation must agree with the height signs.
// Source-edge clipping, missing wires, ambiguous radial ownership and unsupported
// denominators reject atomically. Recovery and integration share one quota.
func (s AlgebraicShellBoolean) PhysicalRadialTubeRegionMassMoments(ctx context.Context, center [2]*big.Rat, radii []*big.Rat, o geom.RadialGraphMomentOptions) (RadialTubeRegionMassMoments, error) {
	zero := RadialTubeRegionMassMoments{}
	bound, e := s.nativeRadialRegionsWithCharts(ctx, center, radii, geom.RadialGraphVolumeOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits}, true)
	if e != nil {
		return zero, e
	}
	o.MaxOperations = bound.options.MaxOperations - bound.work
	o.MaxCoefficientBits = bound.options.MaxCoefficientBits
	m, e := bound.affineProof.MassMomentsBands(ctx, bound.bands, o)
	if e != nil {
		return zero, e
	}
	m.ExactOperations += bound.work
	return RadialTubeRegionMassMoments{Moments: m, Components: bound.components}, nil
}
