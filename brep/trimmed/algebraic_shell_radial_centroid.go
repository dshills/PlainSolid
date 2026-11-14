package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// RadialTubeRegionMassMoments contains original-material planar moments and
// first Z moments, hence exact XYZ centroids. It is not a full inertia tensor.
type RadialTubeRegionMassMoments struct {
	Moments    geom.RadialGraphMassMoments
	Components []RadialTubeRegionVolumeComponent
}

// RadialTubeRegionMassMoments selects complete original-owned radial bands
// exactly as BoundRadialTubeRegionVolume. It additionally integrates the actual
// polynomial height sum; it never replaces either original roof with z=0.
// Both original denominators must be positive constants. All recovery and
// integration arithmetic shares the requested quota; errors publish nothing.
func (s AlgebraicShellBoolean) RadialTubeRegionMassMoments(ctx context.Context, center [2]*big.Rat, radii []*big.Rat, o geom.RadialGraphMomentOptions) (RadialTubeRegionMassMoments, error) {
	zero := RadialTubeRegionMassMoments{}
	bound, e := s.nativeRadialRegions(ctx, center, radii, geom.RadialGraphVolumeOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return zero, e
	}
	o.MaxOperations = bound.options.MaxOperations - bound.work
	o.MaxCoefficientBits = bound.options.MaxCoefficientBits
	m, e := bound.proof.MassMomentsBands(ctx, bound.bands, o)
	if e != nil {
		return zero, e
	}
	m.ExactOperations += bound.work
	return RadialTubeRegionMassMoments{Moments: m, Components: bound.components}, nil
}
