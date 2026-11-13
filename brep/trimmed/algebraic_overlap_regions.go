package trimmed

import (
	"context"
	"fmt"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// SealOverlappingAffineGraphRegions constructs all bounded graph regions after
// exact overlap-guided discovery and independent original-domain coverage.
// Original support/trim identities and each region's source branch are retained.
// The returned collection is not a regularized union: nested regions may share
// a zero-thickness rim, as documented by AlgebraicTubeRegions. Its proven loop
// hierarchy distinguishes those from independent disjoint root components.
func SealOverlappingAffineGraphRegions(ctx context.Context, patches [2]intersect.Patch, faces [2]Face,
	discovery intersect.SplinePlanePairDiscoveryOptions, section AlgebraicTrimmedPairSectionOptions,
	material AlgebraicTubeRegionOptions) (AlgebraicTubeRegions, error) {
	empty := AlgebraicTubeRegions{}
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	proposal, e := intersect.DiscoverOverlappingAffineXYGraphPairTubes(ctx, patches[0], patches[1], section.Trace.Seeds.Patches, discovery)
	if e != nil {
		return empty, fmt.Errorf("trimmed: region overlap proposal: %w", e)
	}
	if proposal.Disjoint {
		return empty, geom.ErrConvergence
	}
	section.Trace.Coverage.OverlappingAffineXYFallback = true
	verified, e := SectionTrimmedSplinePair(ctx, patches, faces, &proposal.Seeds, section)
	if e != nil {
		return empty, fmt.Errorf("trimmed: region original-domain section: %w", e)
	}
	if !verified.Complete {
		return empty, geom.ErrConvergence
	}
	return verified.SealAlgebraicTubeRegions(ctx, material)
}
