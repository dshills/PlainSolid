//go:build research

package trimmed

import (
	"context"
	"fmt"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// SealOverlappingAffineGraphLens discovers a closed graph contact on the exact
// projected overlap, then independently re-certifies all contacts on the full
// original patch/trim domains before sealing material. Original face supports
// and trims are retained. This creates only the bounded two-face graph lens;
// it is not a Boolean of arbitrary surrounding solids or their outside regions.
// Multiple/open contacts, incomplete full-domain coverage, or unproved material
// ordering return no lens. Clipped-domain coverage alone is never accepted.
func SealOverlappingAffineGraphLens(ctx context.Context, patches [2]intersect.Patch, faces [2]Face,
	discovery intersect.SplinePlanePairDiscoveryOptions, section AlgebraicTrimmedPairSectionOptions,
	material AlgebraicTubeLensOptions) (AlgebraicTubeLens, error) {
	empty := AlgebraicTubeLens{}
	if ctx == nil {
		return empty, geom.ErrInvalid
	}
	proposal, e := intersect.DiscoverOverlappingAffineXYGraphPairTubes(ctx, patches[0], patches[1], section.Trace.Seeds.Patches, discovery)
	if e != nil {
		return empty, fmt.Errorf("trimmed: overlap proposal: %w", e)
	}
	if proposal.Disjoint {
		return empty, geom.ErrConvergence
	}
	section.Trace.Coverage.OverlappingAffineXYFallback = true
	verified, e := SectionTrimmedSplinePair(ctx, patches, faces, &proposal.Seeds, section)
	if e != nil {
		return empty, fmt.Errorf("trimmed: original-domain section: %w", e)
	}
	if !verified.Complete {
		return empty, geom.ErrConvergence
	}
	return verified.SealAlgebraicTubeLens(ctx, material)
}
